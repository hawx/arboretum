// Package data implements the data access for arboretum.
package data

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sort"
	"time"

	// register sqlite3 for database/sql
	_ "github.com/mattn/go-sqlite3"
)

type Feed struct {
	URL        string
	WebsiteURL string
	Title      string
	UpdatedAt  time.Time
	Items      []FeedItem
}

type FeedItem struct {
	Key       string
	PermaLink string
	PubDate   time.Time
	Title     string
	Link      string
}

type DB struct {
	db *sql.DB
}

func Open(path string) (*DB, error) {
	sqlite, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	db := &DB{db: sqlite}

	return db, db.migrate()
}

func (d *DB) migrate() error {
	_, err := d.db.Exec(`
    CREATE TABLE IF NOT EXISTS feeds (
      URL         TEXT NOT NULL PRIMARY KEY,
      WebsiteURL  TEXT,
      Title       TEXT,
      UpdatedAt   DATETIME
    );

    CREATE TABLE IF NOT EXISTS feedItems (
      Key       TEXT NOT NULL,
      FeedURL   TEXT NOT NULL,
      PermaLink TEXT,
      PubDate   DATETIME,
      Title     TEXT,
      Link      TEXT,
      PRIMARY KEY (Key, FeedURL)
    );
`)

	return err
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) ReadAll(ctx context.Context) ([]Feed, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT i.Key, i.PermaLink, i.PubDate, i.Title, i.Link, f.WebsiteURL, f.Title, f.UpdatedAt, f.URL
     FROM feedItems i
     JOIN feeds f ON f.URL = i.FeedURL
     ORDER BY FeedURL, PubDate DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	feedsMap := map[string]*Feed{}

	for rows.Next() {
		var (
			websiteURL, title, feedURL string
			updatedAt                  time.Time
			item                       FeedItem
		)
		if err = rows.Scan(&item.Key, &item.PermaLink, &item.PubDate, &item.Title, &item.Link, &websiteURL, &title, &updatedAt, &feedURL); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		if feed, ok := feedsMap[feedURL]; ok {
			feed.Items = append(feed.Items, item)
		} else {
			feedsMap[feedURL] = &Feed{
				URL:        feedURL,
				WebsiteURL: websiteURL,
				Title:      title,
				UpdatedAt:  updatedAt,
				Items:      []FeedItem{item},
			}
		}
	}

	var feeds []Feed
	for _, feed := range feedsMap {
		feeds = append(feeds, *feed)
	}

	if err = rows.Err(); err != nil {
		return feeds, fmt.Errorf("rows err: %w", err)
	}

	sort.Slice(feeds, func(i, j int) bool {
		return feeds[i].UpdatedAt.After(feeds[j].UpdatedAt)
	})

	return feeds, nil
}

func (d *DB) UpdatedAt(ctx context.Context, uri string) (time.Time, error) {
	row := d.db.QueryRowContext(ctx,
		"SELECT UpdatedAt FROM feeds WHERE URL = ?",
		uri)

	var updatedAt *time.Time
	if err := row.Scan(&updatedAt); err != nil {
		return time.Time{}, fmt.Errorf("scanning feed row: %w", err)
	}

	if updatedAt == nil {
		return time.Time{}, nil
	}

	return *updatedAt, nil
}

func (d *DB) UpdateFeed(ctx context.Context, feed Feed) (err error) {
	if len(feed.Items) == 0 {
		return nil
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		action := tx.Rollback
		if err == nil {
			action = tx.Commit
		}

		if rerr := action(); rerr != nil {
			log.Println(rerr)
		}
	}()

	row := tx.QueryRowContext(ctx,
		`SELECT Key FROM feedItems WHERE FeedURL = ? ORDER BY PubDate DESC LIMIT 1`,
		feed.URL)

	var lastKey string
	if err := row.Scan(&lastKey); err != nil && err != sql.ErrNoRows {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`REPLACE INTO feeds (URL, WebsiteURL, Title, UpdatedAt)
    VALUES (?,   ?,          ?,     ?)`,
		feed.URL,
		feed.WebsiteURL,
		feed.Title,
		feed.UpdatedAt)
	if err != nil {
		return err
	}

	sort.Slice(feed.Items, func(i, j int) bool {
		return feed.Items[i].PubDate.After(feed.Items[j].PubDate)
	})
	if feed.Items[0].Key == lastKey {
		return nil
	}

	_, err = tx.ExecContext(ctx,
		`DELETE FROM feedItems WHERE FeedURL = ?`,
		feed.URL)
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO feedItems (Key, FeedURL, PermaLink, PubDate, Title, Link)
                                          VALUES (?,   ?,       ?,         ?,       ?,     ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	if len(feed.Items) > 7 {
		feed.Items = feed.Items[:7]
	}

	for _, item := range feed.Items {
		_, err = stmt.ExecContext(ctx, item.Key, feed.URL, item.PermaLink, item.PubDate, item.Title, item.Link)
		if err != nil {
			log.Println(item.Key)
			return err
		}
	}

	return nil
}

func (d *DB) Subscribe(ctx context.Context, uri string) error {
	_, err := d.db.ExecContext(ctx, "INSERT OR IGNORE INTO feeds (URL) VALUES (?)",
		uri)

	return err
}

func (d *DB) Unsubscribe(ctx context.Context, uri string) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM feedItems WHERE FeedURL = ?", uri)
	if err != nil {
		return err
	}

	_, err = d.db.ExecContext(ctx, "DELETE FROM feeds WHERE URL = ?", uri)

	return err
}

func (d *DB) Subscriptions(ctx context.Context) (list []string, err error) {
	rows, err := d.db.QueryContext(ctx, "SELECT URL FROM feeds")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var uri string
		if err = rows.Scan(&uri); err != nil {
			return
		}
		list = append(list, uri)
	}

	err = rows.Err()
	return
}
