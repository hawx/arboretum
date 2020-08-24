// Package data implements the data access for arboretum.
package data

import (
	"database/sql"
	"fmt"
	"log"
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
      URL         TEXT PRIMARY KEY,
      WebsiteURL  TEXT,
      Title       TEXT,
      UpdatedAt   DATETIME
    );

    CREATE TABLE IF NOT EXISTS feedItems (
      Key       TEXT,
      FeedURL   TEXT,
      PermaLink TEXT,
      PubDate   DATETIME,
      Title     TEXT,
      Link      TEXT,
      PRIMARY KEY (Key, FeedURL)
    );

    CREATE TABLE IF NOT EXISTS feedFetches (
      FeedURL   TEXT NOT NULL,
      FetchedAt DATETIME NOT NULL,
      Value     TEXT,
      PRIMARY KEY (FeedURL, FetchedAt)
    );
`)

	return err
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) ReadAll() ([]Feed, error) {
	rows, err := d.db.Query(`SELECT
                             i.Key, i.PermaLink, i.PubDate, i.Title, i.Link,
                             f.WebsiteURL, f.Title, f.UpdatedAt, f.URL
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

	return feeds, nil
}

func (d *DB) Read(uri string) (feed Feed, err error) {
	row := d.db.QueryRow("SELECT WebsiteURL, Title, UpdatedAt FROM feeds WHERE URL = ? AND WebsiteURL IS NOT NULL",
		uri)

	feed.URL = uri
	if err = row.Scan(&feed.WebsiteURL, &feed.Title, &feed.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return feed, nil
		}
		return feed, fmt.Errorf("scanning feed row: %w", err)
	}

	rows, err := d.db.Query(`SELECT Key, PermaLink, PubDate, Title, Link
                           FROM feedItems
                           WHERE FeedURL = ?
                           ORDER BY PubDate DESC
                           LIMIT 7`,
		uri)
	if err != nil {
		return feed, fmt.Errorf("selecting feedItems: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item FeedItem
		if err = rows.Scan(&item.Key, &item.PermaLink, &item.PubDate, &item.Title, &item.Link); err != nil {
			return feed, fmt.Errorf("scanning feedItems row: %w", err)
		}

		feed.Items = append(feed.Items, item)
	}

	if err = rows.Err(); err != nil {
		return feed, fmt.Errorf("rows err: %w", err)
	}

	return
}

func (d *DB) UpdateFeed(feed Feed) (err error) {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	_, err = tx.Exec(`REPLACE INTO feeds (URL, WebsiteURL, Title, UpdatedAt)
                                VALUES (?,   ?,          ?,     ?)`,
		feed.URL,
		feed.WebsiteURL,
		feed.Title,
		feed.UpdatedAt)
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO feedItems (Key, FeedURL, PermaLink, PubDate, Title, Link)
                                          VALUES (?,   ?,       ?,         ?,       ?,     ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range feed.Items {
		_, err = stmt.Exec(item.Key, feed.URL, item.PermaLink, item.PubDate, item.Title, item.Link)
		if err != nil {
			log.Println(item.Key)
			return err
		}
	}

	return nil
}

func (d *DB) Contains(uri, key string) bool {
	var v int
	err := d.db.QueryRow("SELECT 1 FROM feedItems WHERE FeedURL = ? AND Key = ?", uri, key).Scan(&v)

	if err != nil {
		if err != sql.ErrNoRows {
			log.Println("sql contains:", err)
		}
		return false
	}

	return true
}

func (d *DB) Subscribe(uri string) error {
	_, err := d.db.Exec("INSERT OR IGNORE INTO feeds (URL) VALUES (?)",
		uri)

	return err
}

func (d *DB) Unsubscribe(uri string) error {
	_, err := d.db.Exec("DELETE FROM feedItems WHERE FeedURL = ?", uri)
	if err != nil {
		return err
	}

	_, err = d.db.Exec("DELETE FROM feeds WHERE URL = ?", uri)

	return err
}

func (d *DB) Subscriptions() (list []string, err error) {
	rows, err := d.db.Query("SELECT URL FROM feeds")
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
