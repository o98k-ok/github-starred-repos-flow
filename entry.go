package main

import (
	"context"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/o98k-ok/lazy/v2/alfred"
	"github.com/o98k-ok/lazy/v2/cache/file"
	"golang.org/x/oauth2"
	"os"
	"strings"
	"time"
)

const (
	CacheJsonFile   = "./cache.json"
	DefaultTTL      = "8h"
	DefaultPageSize = 20
	TokenKey        = "GithubToken"
	TTLKey          = "TTL"
)

func ListRepos(token string) (*alfred.Items, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	client := github.NewClient(oauth2.NewClient(ctx, ts))

	option := github.ListOptions{
		Page:    1,
		PerPage: DefaultPageSize,
	}

	res := alfred.NewItems()
	for {
		repos, _, err := client.Activity.ListStarred(ctx, "", &github.ActivityListStarredOptions{ListOptions: option})
		if err != nil {
			return nil, err
		}

		for _, repo := range repos {
			desc := ""
			if repo.Repository.Description != nil {
				desc = *repo.Repository.Description
			}
			res.Append(alfred.NewItem(*repo.Repository.FullName, desc, *repo.Repository.CloneURL))
		}

		if len(repos) < option.PerPage {
			break
		}

		option.Page += 1
	}
	return res, nil
}

func main() {
	variables, err := alfred.FlowVariables()
	if err != nil {
		alfred.ErrItems("Read variables error", err)
		return
	}

	var duration time.Duration
	duration, err = time.ParseDuration(variables[TTLKey])
	if err != nil {
		duration, _ = time.ParseDuration(DefaultTTL)
	}
	token := variables[TokenKey]

	cli := alfred.NewApp("list github star repos")
	cli.Bind("list", func(keys []string) {
		var keyword string
		if len(keys) > 0 {
			keyword = keys[0]
		}
		f, err := os.OpenFile(CacheJsonFile, os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			alfred.ErrItems("open cache file error", err)
			return
		}
		defer f.Close()

		gitStars := file.NewFileCache[*alfred.Items](f, duration)
		old, err := gitStars.Load(func() (*alfred.Items, error) { return ListRepos(token) })
		if err != nil {
			alfred.ErrItems("load github star list error", err)
			return
		}

		var match alfred.Items
		for _, item := range old.Items {
			if !strings.Contains(strings.ToLower(item.Title), strings.ToLower(keyword)) &&
				!strings.Contains(item.SubTitle, keyword) {
				continue
			}

			match.Append(item)
		}
		fmt.Println(match.Encode())
	})

	err = cli.Run(os.Args)
	if err != nil {
		alfred.ErrItems("run failed", err)
		return
	}
}
