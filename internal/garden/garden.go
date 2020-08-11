// Package garden aggregates feeds into a gardenjs file.
//
// That format doesn't exist, yet, the purpose of this is to define it. The idea
// is to have something similiar/exactly the same as fraidycat does: a list of
// feeds ordered by most recently updated, with a compact list of recent items
// for each feed.
//
// This can and will co-exist nicely with rivers because they each solve
// different problems. A garden is for friends, a river is for things you don't
// mind missing. Neither are the dreaded inbox with all of the management of
// read status, etc.
//
// Hopefully this works out as nicely as I think it will.
package garden

import (
	"encoding/json"
	"errors"
	"io"
	"sort"
	"sync"
	"time"

	"hawx.me/code/arboretum/internal/data"
	"hawx.me/code/arboretum/internal/gardenjs"
)

type Options struct {
	Refresh time.Duration
}

type Garden struct {
	store        Database
	cacheTimeout time.Duration

	mu      sync.RWMutex
	flowers map[string]*Feed
}

type Database interface {
	Contains(uri, key string) bool
	Read(uri string) (data.Feed, error)
	UpdateFeed(data.Feed) error
}

func New(store Database, options Options) *Garden {
	if options.Refresh <= 0 {
		options.Refresh = time.Hour
	}

	g := &Garden{
		store:        store,
		cacheTimeout: options.Refresh,
		flowers:      map[string]*Feed{},
	}

	return g
}

func (g *Garden) Latest() (gardenjs.Garden, error) {
	garden := gardenjs.Garden{
		Metadata: gardenjs.Metadata{
			BuiltAt: time.Now(),
		},
	}

	for uri, _ := range g.flowers {
		feed, err := g.store.Read(uri)
		if err != nil {
			return gardenjs.Garden{}, err
		}

		mapped := gardenjs.Feed{
			URL:        uri,
			WebsiteURL: feed.WebsiteURL,
			Title:      feed.Title,
		}

		for _, item := range feed.Items {
			mapped.Items = append(mapped.Items, gardenjs.Item{
				PermaLink: item.PermaLink,
				PubDate:   item.PubDate,
				Title:     item.Title,
				Link:      item.Link,
			})
			if item.PubDate.After(mapped.UpdatedAt) {
				mapped.UpdatedAt = item.PubDate
			}
		}

		garden.Feeds = append(garden.Feeds, mapped)
	}

	sort.Slice(garden.Feeds, func(i, j int) bool {
		return garden.Feeds[i].UpdatedAt.Before(garden.Feeds[j].UpdatedAt)
	})

	return garden, nil
}

func (g *Garden) Encode(w io.Writer) error {
	latest, err := g.Latest()
	if err != nil {
		return err
	}

	return json.NewEncoder(w).Encode(latest)
}

func (g *Garden) Add(uri string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.flowers[uri]; exists {
		return errors.New("already added uri")
	}

	flower, err := NewFeed(g.store, g.cacheTimeout, uri)
	if err != nil {
		return err
	}
	flower.Start()

	g.flowers[uri] = flower
	return nil
}

func (g *Garden) Remove(uri string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if flower, exists := g.flowers[uri]; exists {
		flower.Stop()
		delete(g.flowers, uri)
	}

	return nil
}

func (g *Garden) Close() error {
	for _, flower := range g.flowers {
		flower.Stop()
	}
	return nil
}
