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
	CacheJsonFile = "./cache.json"
	DefaultTTL    = "8h"
	FullPageSize  = 20
	IncrPageSize  = 5
	TokenKey      = "GithubToken"
	TTLKey        = "TTL"
)

// ListRepos from GitHub apis sorted by starred time desc
//           cache history data, compare and update
func ListRepos(cached *alfred.Items, token string) (*alfred.Items, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	client := github.NewClient(oauth2.NewClient(ctx, ts))

	var pageSize = FullPageSize
	cachedItems := make(map[string]*alfred.Item)
	if cached != nil && cached.Len() != 0 {
		pageSize = IncrPageSize
		for _, item := range cached.Items {
			cachedItems[item.Title] = item
		}
	}

	option := github.ListOptions{
		Page:    1,
		PerPage: pageSize,
	}

	var finish bool
	for {
		optns := &github.ActivityListStarredOptions{
			Sort:        "created",
			Direction:   "desc",
			ListOptions: option,
		}
		repos, _, err := client.Activity.ListStarred(ctx, "", optns)
		if err != nil {
			return nil, err
		}

		for _, repo := range repos {
			desc := ""
			if repo.Repository.Description != nil {
				desc = *repo.Repository.Description
			}

			// last starred repo has been cache, so finish call GitHub api
			if _, finish = cachedItems[*repo.Repository.FullName]; finish {
				break
			}
			cachedItems[*repo.Repository.FullName] = alfred.NewItem(*repo.Repository.FullName, desc, *repo.Repository.CloneURL)
		}

		if len(repos) < option.PerPage || finish {
			break
		}

		option.Page += 1
	}

	res := alfred.NewItems()
	for _, item := range cachedItems {
		res.Append(item)
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
		old, err := gitStars.Load(func(cached *alfred.Items, _ time.Time) (*alfred.Items, error) { return ListRepos(cached, token) })
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
