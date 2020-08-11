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
	FeedURL     string
	WebsiteURL  string
	Title       string
	Description string
	UpdatedAt   time.Time
	Items       []FeedItem
}

type FeedItem struct {
	Key        string
	PermaLink  string
	PubDate    time.Time
	Title      string
	Link       string
	Body       string
	ID         string
	Comments   string
	Enclosures []FeedItemEnclosure
	Thumbnails []FeedItemThumbnail
}

type FeedItemEnclosure struct {
	URL    string
	Type   string
	Length int
}

type FeedItemThumbnail struct {
	URL    string
	Height int
	Width  int
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
    CREATE TABLE IF NOT EXISTS keys (
      Bucket TEXT,
      Key TEXT,
      PRIMARY KEY (Bucket, Key)
    );

    CREATE TABLE IF NOT EXISTS subscriptions (
      FeedURL    TEXT,
      Name       TEXT,
      PRIMARY KEY (FeedURL, Name)
    );

    CREATE TABLE IF NOT EXISTS feeds (
      FeedURL     TEXT PRIMARY KEY,
      WebsiteURL  TEXT,
      Title       TEXT,
      Description TEXT,
      UpdatedAt   DATETIME
    );

    CREATE TABLE IF NOT EXISTS feedItems (
      Key       TEXT,
      FeedURL   TEXT,
      PermaLink TEXT,
      PubDate   DATETIME,
      Title     TEXT,
      Link      TEXT,
      Body      TEXT,
      ID        TEXT,
      Comments  TEXT,
      PRIMARY KEY (Key, FeedURL)
    );

    CREATE TABLE IF NOT EXISTS enclosures (
      ID      INTEGER PRIMARY KEY AUTOINCREMENT,
      ItemKey TEXT,
      URL     TEXT,
      Type    TEXT,
      Length  INTEGER
    );

    CREATE TABLE IF NOT EXISTS thumbnails (
      ID      INTEGER PRIMARY KEY AUTOINCREMENT,
      ItemKey TEXT,
      URL     TEXT,
      Height  INTEGER,
      Width   INTEGER
    );

    CREATE TABLE IF NOT EXISTS feedFetches (
      FeedURL   TEXT NOT NULL,
      FetchedAt DATETIME NOT NULL,
      Value     TEXT,
      PRIMARY KEY (FeedURL, FetchedAt)
    );
`)

	log.Println("migrated")
	return err
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) Read(uri string) (feed Feed, err error) {
	row := d.db.QueryRow("SELECT WebsiteURL, Title, Description, UpdatedAt FROM feeds WHERE FeedURL = ?",
		uri)

	feed.FeedURL = uri
	if err = row.Scan(&feed.WebsiteURL, &feed.Title, &feed.Description, &feed.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return feed, nil
		}
		return feed, fmt.Errorf("scanning feed row: %w", err)
	}

	rows, err := d.db.Query(`SELECT Key, PermaLink, PubDate, Title, Link, Body, ID, Comments
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
		if err = rows.Scan(&item.Key, &item.PermaLink, &item.PubDate, &item.Title, &item.Link, &item.Body, &item.ID, &item.Comments); err != nil {
			return feed, fmt.Errorf("scanning feedItems row: %w", err)
		}

		// TODO: enclosures and thumbnails
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

	_, err = tx.Exec(`REPLACE INTO feeds (WebsiteURL, Title, Description, UpdatedAt, FeedURL)
                                VALUES (?,          ?,     ?,           ?,         ?)`,
		feed.WebsiteURL,
		feed.Title,
		feed.Description,
		feed.UpdatedAt,
		feed.FeedURL)
	if err != nil {
		return err
	}

	for _, item := range feed.Items {
		_, err = tx.Exec(`INSERT INTO feedItems (Key, FeedURL, PermaLink, PubDate, Title, Link, Body, ID, Comments)
                                     VALUES (?,   ?,       ?,         ?,       ?,     ?,    ?,    ?,  ?)`,
			item.Key,
			feed.FeedURL,
			item.PermaLink,
			item.PubDate,
			item.Title,
			item.Link,
			item.Body,
			item.ID,
			item.Comments)
		if err != nil {
			return err
		}

		for _, enclosure := range item.Enclosures {
			_, err = tx.Exec(`INSERT INTO enclosures (ItemKey, URL, Type, Length)
                                        VALUES (?,       ?,   ?,    ?)`,
				item.Key,
				enclosure.URL,
				enclosure.Type,
				enclosure.Length)
			if err != nil {
				return err
			}
		}

		for _, thumbnail := range item.Thumbnails {
			_, err = tx.Exec(`INSERT INTO thumbnails (ItemKey, URL, Height, Width)
                                        VALUES (?,       ?,   ?,      ?)`,
				item.Key,
				thumbnail.URL,
				thumbnail.Height,
				thumbnail.Width)
			if err != nil {
				return err
			}
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
