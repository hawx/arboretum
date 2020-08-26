package garden

import "hawx.me/code/arboretum/internal/data"

type DB interface {
	Read(uri string) (data.Feed, error)
	ReadAll() ([]data.Feed, error)
	UpdateFeed(data.Feed) error
}

type dbWrapper struct {
	db  DB
	uri string
}

// Contains will always report false, because we will decide whether to update
// when trying to insert the new items.
func (d *dbWrapper) Contains(key string) bool {
	return false
}
