package data

import (
	"context"
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
	assert(result).Equal([]Feed{feed, feed2})
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

func TestSetUpdatedAt(t *testing.T) {
	assert := assert.Wrap(t)

	db, err := Open("file:TestUpdatedAt?cache=shared&mode=memory")
	assert(err).Must.Nil()
	defer db.Close()

	updatedAt := time.Now().Add(-5 * time.Minute)
	url := "a url"

	db.db.Exec("INSERT INTO feeds (UpdatedAt, URL) VALUES (?, ?)",
		updatedAt, url)

	newUpdatedAt := time.Now()
	err = db.SetUpdatedAt(context.Background(), url, newUpdatedAt)
	assert(err).Must.Nil()

	result, err := db.UpdatedAt(context.Background(), url)
	assert(err).Nil()
	assert(result.Unix()).Equal(newUpdatedAt.Unix())
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
