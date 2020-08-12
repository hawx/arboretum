package garden

import "hawx.me/code/arboretum/internal/data"

type DB interface {
	Contains(uri, key string) bool
	Read(uri string) (data.Feed, error)
	UpdateFeed(data.Feed) error
}

type dbWrapper struct {
	db  DB
	uri string
}

func (d *dbWrapper) Contains(key string) bool {
	return d.db.Contains(d.uri, key)
}
