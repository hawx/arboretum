// Package garden aggregates feeds.
package garden

import (
	"context"
	"log"
	"sort"
	"time"

	"hawx.me/code/arboretum/internal/gardenjs"
)

type Garden struct {
	db      DB
	refresh time.Duration

	added   chan string
	removed chan string
	feeds   map[string]context.CancelFunc
}

func New(store DB, refresh time.Duration) *Garden {
	g := &Garden{
		db:      store,
		refresh: refresh,
		feeds:   map[string]context.CancelFunc{},
		added:   make(chan string),
		removed: make(chan string),
	}

	return g
}

func (g *Garden) Latest() (gardenjs.Garden, error) {
	garden := gardenjs.Garden{
		Metadata: gardenjs.Metadata{
			BuiltAt: time.Now(),
		},
	}

	for uri, _ := range g.feeds {
		feed, err := g.db.Read(uri)
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

func (g *Garden) Subscribe(uri string) error {
	g.added <- uri
	return nil
}

func (g *Garden) Unsubscribe(uri string) error {
	g.removed <- uri
	return nil
}

func (g *Garden) Run(ctx context.Context) {
	for {
		select {
		case uri := <-g.added:
			if _, ok := g.feeds[uri]; ok {
				log.Println("already added", uri)
				continue
			}

			feed, err := NewFeed(g.db, g.refresh, uri)
			if err != nil {
				log.Printf("error adding %s: %v\n", uri, err)
				continue
			}

			childCtx, cancel := context.WithCancel(ctx)
			g.feeds[uri] = cancel

			go func() {
				feed.Run(childCtx)
			}()

		case uri := <-g.removed:
			cancel, ok := g.feeds[uri]
			if !ok {
				log.Println("no such feed", uri)
				continue
			}

			cancel()
			delete(g.feeds, uri)

		case <-ctx.Done():
			return
		}
	}
}
