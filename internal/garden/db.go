package garden

import (
	"context"

	"hawx.me/code/arboretum/internal/data"
)

type DB interface {
	Read(ctx context.Context, uri string) (data.Feed, error)
	ReadAll(context.Context) ([]data.Feed, error)
	UpdateFeed(context.Context, data.Feed) error
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
