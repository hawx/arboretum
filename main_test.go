package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hawx.me/code/arboretum/internal/data"
	"hawx.me/code/arboretum/internal/garden"
	"hawx.me/code/arboretum/internal/gardenjs"
	"hawx.me/code/assert"
)

const (
	atomOneItem = `<feed xmlns="http://www.w3.org/2005/Atom">
	<title type="text">Some title</title>
	<id>http://www.example.com/feed/atom/</id>
  <updated>2099-11-09T17:23:12Z</updated>
	<entry>
		<title>First title</title>
		<id>1</id>
		<updated>2003-11-09T17:23:02Z</updated>
	</entry>
</feed>`
	atomTwoItem = `<feed xmlns="http://www.w3.org/2005/Atom">
	<title type="text">Some title</title>
	<id>http://www.example.com/feed/atom/</id>
  <updated>2099-11-09T17:23:12Z</updated>
	<entry>
		<title>First title</title>
		<id>1</id>
		<updated>2003-11-09T17:23:02Z</updated>
	</entry>
	<entry>
		<title>Second title</title>
		<id>2</id>
		<updated>2003-11-10T17:23:02Z</updated>
	</entry>
</feed>`
)

type handlerQueue struct {
	handlers []http.HandlerFunc
	idx      int
}

func newHandlerQueue(handlers ...http.HandlerFunc) http.Handler {
	return &handlerQueue{handlers: handlers}
}

func (q *handlerQueue) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if q.idx > len(q.handlers) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	q.handlers[q.idx].ServeHTTP(w, r)
	q.idx++
}

func contextWithDelayedCancel() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	return ctx, func() {
		// call in go routine so http handler finishes
		go func() {
			// don't cancel immediately as everything still needs to process
			time.Sleep(time.Millisecond)
			cancel()
		}()
	}
}

func TestGardenLatestWithNoFeeds(t *testing.T) {
	db, err := data.Open(":memory:")
	if err != nil {
		t.Error(err)
		return
	}
	defer db.Close()

	garden := garden.New(db, time.Millisecond)

	result, err := garden.Latest(context.Background())
	if err != nil {
		t.Error(err)
		return
	}

	assert.Len(t, result.Feeds, 0)
}

func TestGardenLatestWithFeed(t *testing.T) {
	ctx, cancel := contextWithDelayedCancel()
	defer cancel()

	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		io.WriteString(w, atomOneItem)
		cancel()
	}))

	db, err := data.Open(":memory:")
	if err != nil {
		t.Error(err)
		return
	}
	defer db.Close()

	if err := db.Subscribe(ctx, feed.URL); err != nil {
		t.Error(err)
		return
	}

	garden := garden.New(db, time.Millisecond)
	go func() {
		garden.Run(ctx)
	}()
	garden.Subscribe(ctx, feed.URL)

	<-ctx.Done()

	result, err := garden.Latest(context.Background())
	if err != nil {
		t.Error(err)
		return
	}

	assert.Equal(t, []gardenjs.Feed{{
		URL:       feed.URL,
		Title:     "Some title",
		UpdatedAt: time.Date(2003, time.November, 9, 17, 23, 2, 0, time.UTC),
		Items: []gardenjs.Item{{
			PermaLink: feed.URL,
			Title:     "First title",
			PubDate:   time.Date(2003, time.November, 9, 17, 23, 2, 0, time.UTC),
			Link:      feed.URL,
		}},
	}}, result.Feeds)
}

func TestGardenLatestWithFeedAndUpdate(t *testing.T) {
	ctx, cancel := contextWithDelayedCancel()
	defer cancel()

	feed := httptest.NewServer(newHandlerQueue(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			io.WriteString(w, atomOneItem)
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			io.WriteString(w, atomTwoItem)
			cancel()
		},
	))

	db, err := data.Open(":memory:")
	if err != nil {
		t.Error(err)
		return
	}
	defer db.Close()

	if err := db.Subscribe(ctx, feed.URL); err != nil {
		t.Error(err)
		return
	}

	garden := garden.New(db, time.Millisecond)
	go func() {
		garden.Run(ctx)
	}()
	garden.Subscribe(ctx, feed.URL)

	<-ctx.Done()

	result, err := garden.Latest(context.Background())
	if err != nil {
		t.Error(err)
		return
	}

	assert.Equal(t, []gardenjs.Feed{{
		URL:       feed.URL,
		Title:     "Some title",
		UpdatedAt: time.Date(2003, time.November, 10, 17, 23, 2, 0, time.UTC),
		Items: []gardenjs.Item{{
			PermaLink: feed.URL,
			Title:     "Second title",
			PubDate:   time.Date(2003, time.November, 10, 17, 23, 2, 0, time.UTC),
			Link:      feed.URL,
		}, {
			PermaLink: feed.URL,
			Title:     "First title",
			PubDate:   time.Date(2003, time.November, 9, 17, 23, 2, 0, time.UTC),
			Link:      feed.URL,
		}},
	}}, result.Feeds)
}

func TestGardenLatestWithFeedAndETagPreventingUpdate(t *testing.T) {
	ctx, cancel := contextWithDelayedCancel()
	defer cancel()

	feed := httptest.NewServer(newHandlerQueue(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			w.Header().Set("ETag", "an-etag")
			io.WriteString(w, atomOneItem)
		},
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get("If-None-Match"), "an-etag")
			w.WriteHeader(http.StatusNotModified)
			cancel()
		},
	))

	db, err := data.Open(":memory:")
	if err != nil {
		t.Error(err)
		return
	}
	defer db.Close()

	if err := db.Subscribe(ctx, feed.URL); err != nil {
		t.Error(err)
		return
	}

	garden := garden.New(db, time.Millisecond)
	go func() {
		garden.Run(ctx)
	}()
	garden.Subscribe(ctx, feed.URL)

	<-ctx.Done()

	result, err := garden.Latest(context.Background())
	if err != nil {
		t.Error(err)
		return
	}

	assert.Equal(t, []gardenjs.Feed{{
		URL:       feed.URL,
		Title:     "Some title",
		UpdatedAt: time.Date(2003, time.November, 9, 17, 23, 2, 0, time.UTC),
		Items: []gardenjs.Item{{
			PermaLink: feed.URL,
			Title:     "First title",
			PubDate:   time.Date(2003, time.November, 9, 17, 23, 2, 0, time.UTC),
			Link:      feed.URL,
		}},
	}}, result.Feeds)
}

func TestGardenLatestWithFeedAndIfModifiedSincePreventingUpdate(t *testing.T) {
	ctx, cancel := contextWithDelayedCancel()
	defer cancel()

	feed := httptest.NewServer(newHandlerQueue(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			io.WriteString(w, atomOneItem)
		},
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, r.Header.Get("If-Modified-Since"), time.Now().Format(time.RFC1123))
			w.WriteHeader(http.StatusNotModified)
			cancel()
		},
	))

	db, err := data.Open(":memory:")
	if err != nil {
		t.Error(err)
		return
	}
	defer db.Close()

	if err := db.Subscribe(ctx, feed.URL); err != nil {
		t.Error(err)
		return
	}

	garden := garden.New(db, time.Millisecond)
	go func() {
		garden.Run(ctx)
	}()
	garden.Subscribe(ctx, feed.URL)

	<-ctx.Done()

	result, err := garden.Latest(context.Background())
	if err != nil {
		t.Error(err)
		return
	}

	assert.Equal(t, []gardenjs.Feed{{
		URL:       feed.URL,
		Title:     "Some title",
		UpdatedAt: time.Date(2003, time.November, 9, 17, 23, 2, 0, time.UTC),
		Items: []gardenjs.Item{{
			PermaLink: feed.URL,
			Title:     "First title",
			PubDate:   time.Date(2003, time.November, 9, 17, 23, 2, 0, time.UTC),
			Link:      feed.URL,
		}},
	}}, result.Feeds)
}
