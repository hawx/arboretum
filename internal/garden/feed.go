package garden

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/html/charset"
	"hawx.me/code/arboretum/internal/data"
	"hawx.me/code/riviera/feed"
	"hawx.me/code/riviera/feed/common"
	"hawx.me/code/riviera/river/mapping"
)

type Feed struct {
	uri    *url.URL
	feed   *feed.Feed
	client *http.Client
	db     Database
}

type dbWrapper struct {
	db  Database
	uri string
}

func (d *dbWrapper) Contains(key string) bool {
	return d.db.Contains(d.uri, key)
}

func NewFeed(db Database, cacheTimeout time.Duration, uri string) (*Feed, error) {
	parsedURI, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	f := &Feed{
		uri:    parsedURI,
		client: http.DefaultClient,
		db:     db,
	}

	f.feed = feed.New(cacheTimeout, f.itemHandler, &dbWrapper{db: db, uri: uri})

	return f, nil
}

func (f *Feed) Run(ctx context.Context) {
	f.fetch()

	for {
		select {
		case <-time.After(f.feed.DurationTillUpdate()):
			f.fetch()

		case <-ctx.Done():
			return
		}
	}
}

func (f *Feed) fetch() {
	code, err := f.feed.Fetch(f.uri.String(), f.client, charset.NewReaderLabel)
	if err != nil {
		log.Printf("error fetching %s: %d %s\n", f.uri, code, err)
	}
}

func (f *Feed) itemHandler(feed *feed.Feed, ch *common.Channel, newitems []*common.Item) {
	if len(newitems) == 0 {
		return
	}

	items := make([]data.FeedItem, len(newitems))

	for i, item := range newitems {
		converted := mapping.DefaultMapping(item)

		if converted != nil {
			converted.Link = maybeResolvedLink(f.uri, converted.Link)
			converted.PermaLink = maybeResolvedLink(f.uri, converted.PermaLink)

			items[i] = data.FeedItem{
				Key:       converted.PermaLink,
				PermaLink: converted.PermaLink,
				PubDate:   converted.PubDate.Add(0),
				Title:     converted.Title,
				Link:      converted.Link,
			}
		}
	}

	feedURL := f.uri.String()
	websiteURL := ""
	for _, link := range ch.Links {
		if link.Rel != "self" {
			websiteURL = maybeResolvedLink(f.uri, link.Href)
			break
		}
	}

	log.Println("updating feed", feedURL)
	if err := f.db.UpdateFeed(data.Feed{
		FeedURL:     feedURL,
		WebsiteURL:  websiteURL,
		Title:       ch.Title,
		Description: ch.Description,
		UpdatedAt:   time.Now(),
		Items:       items,
	}); err != nil {
		log.Println(err)
	}
}

func maybeResolvedLink(root *url.URL, other string) string {
	parsed, err := root.Parse(other)
	if err == nil {
		return parsed.String()
	}

	return other
}
