// Package river generates river.js files. See riverjs.org for more information
// on the format.
package river

import (
	"github.com/hawx/riviera/data"
	"github.com/hawx/riviera/river/models"
	"github.com/hawx/riviera/river/persistence"
	"github.com/hawx/riviera/subscriptions"

	"encoding/json"
	"io"
	"time"
)

const DOCS = "http://scripting.com/stories/2010/12/06/innovationRiverOfNewsInJso.html"

type River interface {
	WriteTo(io.Writer) error
	SubscribeTo(subscriptions.List)
}

type river struct {
	confluence   *confluence
	store        data.Database
	cacheTimeout time.Duration
	subs         subscriptions.List
	mapping      Mapping
}

func New(store data.Database, mapping Mapping, cutOff, cacheTimeout time.Duration) River {
	r, _ := persistence.NewRiver(store)
	confluence := newConfluence(r, cutOff)

	return &river{confluence, store, cacheTimeout, nil, mapping}
}

func (r *river) SubscribeTo(subs subscriptions.List) {
	r.subs = subs

	for _, sub := range subs.List() {
		r.Add(sub)
	}

	subs.OnAdd(func(sub subscriptions.Subscription) {
		r.Add(sub)
	})

	subs.OnRemove(func(uri string) {
		r.Remove(uri)
	})
}

func (r *river) WriteTo(w io.Writer) error {
	updatedFeeds := models.Feeds{r.confluence.Latest()}
	now := time.Now()

	metadata := models.Metadata{
		Docs:      DOCS,
		WhenGMT:   models.RssTime{now.UTC()},
		WhenLocal: models.RssTime{now},
		Version:   "3",
		Secs:      0,
	}

	return json.NewEncoder(w).Encode(models.River{
		Metadata:     metadata,
		UpdatedFeeds: updatedFeeds,
	})
}

func (r *river) Add(sub subscriptions.Subscription) {
	b, _ := persistence.NewBucket(r.store, sub.Uri)

	tributary := newTributary(b, sub.Uri, r.cacheTimeout, r.mapping)

	tributary.OnUpdate(func(feed models.Feed) {
		sub.FeedUrl = feed.FeedUrl
		sub.WebsiteUrl = feed.WebsiteUrl
		sub.FeedTitle = feed.FeedTitle
		sub.FeedDescription = feed.FeedDescription

		r.subs.Refresh(sub)
	})

	tributary.OnStatus(func(code Status) {
		switch code {
		case Good:
			sub.Status = subscriptions.Good
		case Bad:
			sub.Status = subscriptions.Bad
		case Gone:
			sub.Status = subscriptions.Gone
			defer tributary.Kill()
		}

		r.subs.Refresh(sub)
	})

	r.confluence.Add(tributary)
}

func (r *river) Remove(uri string) bool {
	return r.confluence.Remove(uri)
}
