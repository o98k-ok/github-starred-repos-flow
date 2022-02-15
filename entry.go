package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/o98k-ok/lazy/app"
	"golang.org/x/oauth2"
	"io/ioutil"
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
	ErrPage         = "github.com"
)

func ResError(stage string, info string) string {
	return app.NewItems().Append(app.NewItem(stage, info, ErrPage, "")).Encode()
}

func dumpCache() error {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv(TokenKey)},
	)
	client := github.NewClient(oauth2.NewClient(ctx, ts))

	option := github.ListOptions{
		Page:    1,
		PerPage: DefaultPageSize,
	}

	res := app.NewItems()
	for {
		repos, _, err := client.Activity.ListStarred(ctx, "", &github.ActivityListStarredOptions{ListOptions: option})
		if err != nil {
			return err
		}

		for _, repo := range repos {
			desc := ""
			if repo.Repository.Description != nil {
				desc = *repo.Repository.Description
			}
			res.Append(app.NewItem(*repo.Repository.FullName, desc, *repo.Repository.CloneURL, ""))
		}

		if len(repos) < option.PerPage {
			break
		}

		option.Page += 1
	}

	ioutil.WriteFile(CacheJsonFile, []byte(res.Encode()), 0644)
	return nil
}

func loadCache() (*app.Items, error) {
	d, err := ioutil.ReadFile(CacheJsonFile)
	if err != nil {
		return nil, err
	}

	items := app.NewItems()
	if err = json.Unmarshal(d, items); err != nil {
		return nil, err
	}
	return items, nil
}

func main() {
	var match string
	if len(os.Args) > 1 {
		match = os.Args[1]
	}

	duration, err := time.ParseDuration(os.Getenv(TTLKey))
	if err != nil {
		duration, _ = time.ParseDuration(DefaultTTL)
	}

	// check expire time
	info, err := os.Stat(CacheJsonFile)
	if err != nil || info.ModTime().Add(duration).Before(time.Now()) {
		if err = dumpCache(); err != nil {
			fmt.Println(ResError("Caching", err.Error()))
			return
		}
	}

	js, err := loadCache()
	if err != nil {
		fmt.Println(ResError("Loading", err.Error()))
		return
	}

	res := app.NewItems()
	for _, item := range js.Items {
		if !strings.Contains(strings.ToLower(item.Title), strings.ToLower(match)) {
			continue
		}

		res.Append(item)
	}
	fmt.Println(res.Encode())
}
