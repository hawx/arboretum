package data

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"hawx.me/code/assert"
)

func TestReadAll(t *testing.T) {
	assert := assert.Wrap(t)

	db, err := Open("file:TestReadAll?cache=shared&mode=memory")
	assert(err).Must.Nil()
	defer db.Close()

	feed := Feed{
		URL:        "feed-url",
		WebsiteURL: "website-url",
		Title:      "feed-title",
		UpdatedAt:  time.Now().Add(-2 * time.Hour).UTC(),
		Items: []FeedItem{
			{
				Key:       "item-key",
				PermaLink: "item-permalink",
				PubDate:   time.Now().Add(-5 * time.Minute).UTC(),
				Title:     "item-title",
				Link:      "item-link",
			},
			{
				Key:       "item2-key",
				PermaLink: "item2-permalink",
				PubDate:   time.Now().Add(-10 * time.Minute).UTC(),
				Title:     "item2-title",
				Link:      "item2-link",
			},
		},
	}

	assert(db.UpdateFeed(context.Background(), feed)).Must.Nil()

	feed2 := Feed{
		URL:        "feed2-url",
		WebsiteURL: "website2-url",
		Title:      "feed2-title",
		UpdatedAt:  time.Now().Add(-time.Hour).UTC(),
		Items: []FeedItem{
			{
				Key:       "2item-key",
				PermaLink: "2item-permalink",
				PubDate:   time.Now().Add(-5*time.Minute - time.Hour).UTC(),
				Title:     "2item-title",
				Link:      "2item-link",
			},
			{
				Key:       "2item2-key",
				PermaLink: "2item2-permalink",
				PubDate:   time.Now().Add(-10*time.Minute - time.Hour).UTC(),
				Title:     "2item2-title",
				Link:      "2item2-link",
			},
		},
	}

	assert(db.UpdateFeed(context.Background(), feed2)).Must.Nil()

	result, err := db.ReadAll(context.Background())
	assert(err).Must.Nil()
	assert(result).Equal([]Feed{feed2, feed})
}

func TestUpdatedAt(t *testing.T) {
	assert := assert.Wrap(t)

	db, err := Open("file:TestUpdatedAt?cache=shared&mode=memory")
	assert(err).Must.Nil()
	defer db.Close()

	updatedAt := time.Now()
	url := "a url"

	db.db.Exec("INSERT INTO feeds (UpdatedAt, URL) VALUES (?, ?)",
		updatedAt, url)

	result, err := db.UpdatedAt(context.Background(), url)
	assert(err).Nil()
	assert(result.Unix()).Equal(updatedAt.Unix())
}

func TestUpdateFeed(t *testing.T) {
	assert := assert.Wrap(t)

	db, err := Open("file:TestUpdateFeed?cache=shared&mode=memory")
	assert(err).Must.Nil()
	defer db.Close()

	feed := Feed{
		URL:        "feed-url",
		WebsiteURL: "website-url",
		Title:      "feed-title",
		UpdatedAt:  time.Now(),
		Items: []FeedItem{
			{
				Key:       "item-key",
				PermaLink: "item-permalink",
				PubDate:   time.Now().Add(-5 * time.Minute),
				Title:     "item-title",
				Link:      "item-link",
			},
			{
				Key:       "item2-key",
				PermaLink: "item2-permalink",
				PubDate:   time.Now().Add(-10 * time.Minute),
				Title:     "item2-title",
				Link:      "item2-link",
			},
		},
	}

	err = db.UpdateFeed(context.Background(), feed)
	assert(err).Must.Nil()

	var feedsCount int
	assert(db.db.QueryRow("SELECT COUNT(1) FROM feeds").Scan(&feedsCount)).Must.Nil()
	assert(feedsCount).Equal(1)

	var itemsCount int
	assert(db.db.QueryRow("SELECT COUNT(1) FROM feedItems").Scan(&itemsCount)).Must.Nil()
	assert(itemsCount).Equal(2)
}

func TestSubscribe(t *testing.T) {
	assert := assert.Wrap(t)

	db, err := Open("file:TestSubscribe?cache=shared&mode=memory")
	assert(err).Must.Nil()
	defer db.Close()

	url := "a-uri"

	err = db.Subscribe(context.Background(), url)
	assert(err).Must.Nil()

	var feedsCount int
	assert(db.db.QueryRow("SELECT COUNT(1) FROM feeds").Scan(&feedsCount)).Must.Nil()
	assert(feedsCount).Equal(1)

	var result string
	assert(db.db.QueryRow("SELECT URL FROM feeds").Scan(&result)).Must.Nil()
	assert(result).Equal(url)
}

func TestUnsubscribe(t *testing.T) {
	assert := assert.Wrap(t)

	db, err := Open("file:TestUnsubscribe?cache=shared&mode=memory")
	assert(err).Must.Nil()
	defer db.Close()

	url := "a-uri"

	err = db.Subscribe(context.Background(), url)
	assert(err).Must.Nil()

	err = db.Unsubscribe(context.Background(), url)
	assert(err).Must.Nil()

	var feedsCount int
	assert(db.db.QueryRow("SELECT COUNT(1) FROM feeds").Scan(&feedsCount)).Must.Nil()
	assert(feedsCount).Equal(0)
}

func TestSubscriptions(t *testing.T) {
	assert := assert.Wrap(t)
	ctx := context.Background()

	db, err := Open("file:TestSubscriptions?cache=shared&mode=memory")
	assert(err).Must.Nil()
	defer db.Close()

	assert(db.Subscribe(ctx, "a")).Must.Nil()
	assert(db.Subscribe(ctx, "b")).Must.Nil()
	assert(db.Subscribe(ctx, "c")).Must.Nil()

	result, err := db.Subscriptions(ctx)
	assert(err).Nil()

	sort.Strings(result)
	assert(result).Equal([]string{"a", "b", "c"})
}

func TestFetched(t *testing.T) {
	assert := assert.Wrap(t)

	db, err := Open("file:TestFetched?cache=shared&mode=memory")
	assert(err).Must.Nil()
	defer db.Close()

	feedURL := "feed-url"
	fetchedAt := time.Now()
	status := 234
	errIn := errors.New("err-in")

	assert(db.Fetched(context.Background(), feedURL, fetchedAt, status, errIn)).Must.Nil()

	var feedFetchesCount int
	assert(db.db.QueryRow("SELECT COUNT(1) FROM feedFetches").Scan(&feedFetchesCount)).Must.Nil()
	assert(feedFetchesCount).Equal(1)

	var result struct {
		FeedURL   string
		FetchedAt time.Time
		Status    int
		Error     string
	}
	assert(db.db.QueryRow("SELECT FeedURL, FetchedAt, Status, Error FROM feedFetches").
		Scan(&result.FeedURL, &result.FetchedAt, &result.Status, &result.Error)).Must.Nil()
	assert(result.FeedURL).Equal(feedURL)
	assert(result.FetchedAt.Unix()).Equal(fetchedAt.Unix())
	assert(result.Status).Equal(status)
	assert(result.Error).Equal(errIn.Error())
}

func TestFetchedNoError(t *testing.T) {
	assert := assert.Wrap(t)

	db, err := Open("file:TestFetchedNoError?cache=shared&mode=memory")
	assert(err).Must.Nil()
	defer db.Close()

	feedURL := "feed-url"
	fetchedAt := time.Now()
	status := 234

	assert(db.Fetched(context.Background(), feedURL, fetchedAt, status, nil)).Must.Nil()

	var feedFetchesCount int
	assert(db.db.QueryRow("SELECT COUNT(1) FROM feedFetches").Scan(&feedFetchesCount)).Must.Nil()
	assert(feedFetchesCount).Equal(1)

	var result struct {
		FeedURL   string
		FetchedAt time.Time
		Status    int
		Error     string
	}
	assert(db.db.QueryRow("SELECT FeedURL, FetchedAt, Status, Error FROM feedFetches").
		Scan(&result.FeedURL, &result.FetchedAt, &result.Status, &result.Error)).Must.Nil()
	assert(result.FeedURL).Equal(feedURL)
	assert(result.FetchedAt.Unix()).Equal(fetchedAt.Unix())
	assert(result.Status).Equal(status)
	assert(result.Error).Equal("")
}
