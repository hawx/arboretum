// Package river aggregates feeds into a riverjs file.
//
// See http://riverjs.org for more information on the format.
package river

import (
	"encoding/json"
	"io"
	"time"

	"hawx.me/code/riviera/feed"
	"hawx.me/code/riviera/river/confluence"
	"hawx.me/code/riviera/river/events"
	"hawx.me/code/riviera/river/mapping"
	"hawx.me/code/riviera/river/riverjs"
	"hawx.me/code/riviera/river/tributary"
)

const docsPath = "http://scripting.com/stories/2010/12/06/innovationRiverOfNewsInJso.html"

// A River aggregates feeds that it is subscribed to, and writes them in riverjs format.
type River interface {
	// Encode writes the river to w in json format. It does not write the json in
	// a javascript callback function.
	Encode(w io.Writer) error

	Latest() riverjs.River

	// Log returns a list of fetch events.
	Log() []events.Event

	// Add subscribes the river to the feed at uri.
	Add(uri string)

	// Remove unsubscribes the river from the feed at url.
	Remove(uri string)

	// Close gracefully stops feeds from being checked.
	Close() error
}

type Subs interface {
	List() ([]string, error)
	OnAdd(func(string))
	OnRemove(func(string))
}

// Database is a key-value store with data arranged in buckets.
type Database interface {
	// Feed returns a database for storing known items from a named feed.
	Feed(name string) (feed.Database, error)

	// Confluence returns a database for storing past rivers.
	Confluence() confluence.Database

	// Close releases all database resources.
	Close() error
}

// river acts as the top-level factory. It manages the creation of the initial
// confluence and creating new tributaries to add to it.
type river struct {
	confluence   confluence.Confluence
	store        Database
	cacheTimeout time.Duration
	mapping      mapping.Mapping
}

// New creates an empty river.
func New(store Database, options Options, subs Subs) River {
	if options.Mapping == nil {
		options.Mapping = DefaultOptions.Mapping
	}
	if options.CutOff == 0 {
		options.CutOff = DefaultOptions.CutOff
	}
	if options.Refresh == 0 {
		options.Refresh = DefaultOptions.Refresh
	}

	confluenceStore := store.Confluence()

	g := &river{
		confluence:   confluence.New(confluenceStore, options.CutOff, options.LogLength),
		store:        store,
		cacheTimeout: options.Refresh,
		mapping:      options.Mapping,
	}

	list, _ := subs.List()
	for _, uri := range list {
		g.Add(uri)
	}

	subs.OnAdd(func(uri string) {
		g.Add(uri)
	})
	subs.OnRemove(func(uri string) {
		g.Remove(uri)
	})

	return g
}

func (r *river) Latest() riverjs.River {
	updatedFeeds := riverjs.Feeds{
		UpdatedFeeds: r.confluence.Latest(),
	}

	now := time.Now()
	metadata := riverjs.Metadata{
		Docs:      docsPath,
		WhenGMT:   riverjs.Time(now.UTC()),
		WhenLocal: riverjs.Time(now),
		Version:   "3",
		Secs:      0,
	}

	return riverjs.River{
		Metadata:     metadata,
		UpdatedFeeds: updatedFeeds,
	}
}

func (r *river) Encode(w io.Writer) error {
	updatedFeeds := riverjs.Feeds{
		UpdatedFeeds: r.confluence.Latest(),
	}

	now := time.Now()
	metadata := riverjs.Metadata{
		Docs:      docsPath,
		WhenGMT:   riverjs.Time(now.UTC()),
		WhenLocal: riverjs.Time(now),
		Version:   "3",
		Secs:      0,
	}

	return json.NewEncoder(w).Encode(riverjs.River{
		Metadata:     metadata,
		UpdatedFeeds: updatedFeeds,
	})
}

func (r *river) Add(uri string) {
	feedStore, _ := r.store.Feed(uri)
	tributary := tributary.New(feedStore, uri, r.cacheTimeout, r.mapping)
	r.confluence.Add(tributary)

	tributary.Start()
}

func (r *river) Remove(uri string) {
	r.confluence.Remove(uri)
}

func (r *river) Log() []events.Event {
	return r.confluence.Log()
}

func (r *river) Close() error {
	r.confluence.Close()
	return nil
}
