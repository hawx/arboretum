package garden

import (
	"context"
	"fmt"
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

const userAgent = "arboretum golang"

type Feed struct {
	uri     *url.URL
	client  *http.Client
	db      DB
	refresh time.Duration

	ctx        context.Context
	lastUpdate time.Time
	lastETag   string
}

func NewFeed(ctx context.Context, db DB, refresh time.Duration, uri string) (*Feed, error) {
	parsedURI, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	lastUpdate, err := db.UpdatedAt(ctx, uri)
	if err != nil {
		return nil, err
	}

	return &Feed{
		uri:        parsedURI,
		client:     http.DefaultClient,
		db:         db,
		refresh:    refresh,
		ctx:        ctx,
		lastUpdate: lastUpdate,
	}, nil
}

func (f *Feed) Run() {
	for {
		dur := f.refresh - time.Now().Sub(f.lastUpdate)
		if dur < 0 {
			dur = 0
		}

		log.Printf("waiting uri=%s dur=%v\n", f.uri, dur)

		select {
		case <-time.After(dur):
			f.fetch()
			f.lastUpdate = time.Now()

		case <-f.ctx.Done():
			return
		}
	}
}

func (f *Feed) fetch() {
	status, err := f.doFetch()
	log.Printf("fetched uri=%s status=%d err=%v", f.uri, status, err)
}

func (f *Feed) doFetch() (int, error) {
	req, err := http.NewRequest("GET", f.uri.String(), nil)
	if err != nil {
		return -1, fmt.Errorf("creating request for %v: %w", f.uri, err)
	}

	req.Header.Set("User-Agent", userAgent)
	if !f.lastUpdate.IsZero() {
		req.Header.Set("If-Modified-Since", f.lastUpdate.Format(time.RFC1123))
	}
	if f.lastETag != "" {
		req.Header.Set("If-None-Match", f.lastETag)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return -1, fmt.Errorf("making request for %v: %w", f.uri, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, f.db.SetUpdatedAt(f.ctx, f.uri.String(), time.Now())
	}

	f.lastETag = resp.Header.Get("ETag")

	channels, err := feed.Parse(resp.Body, f.uri, charset.NewReaderLabel)

	if err == nil && len(channels) > 0 {
		for _, channel := range channels {
			f.handleItems(channel, channel.Items)
		}
	}

	return resp.StatusCode, err
}

func (f *Feed) handleItems(ch *common.Channel, newitems []*common.Item) {
	items := make([]data.FeedItem, len(newitems))

	for i, item := range newitems {
		converted := mapping.DefaultMapping(item)

		if converted != nil {
			converted.Link = maybeResolvedLink(f.uri, converted.Link)
			converted.PermaLink = maybeResolvedLink(f.uri, converted.PermaLink)

			items[i] = data.FeedItem{
				Key:       item.Key(),
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

	log.Printf("updating uri=%s items=%d\n", feedURL, len(items))
	if err := f.db.UpdateFeed(f.ctx, data.Feed{
		URL:        feedURL,
		WebsiteURL: websiteURL,
		Title:      ch.Title,
		UpdatedAt:  time.Now(),
		Items:      items,
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
