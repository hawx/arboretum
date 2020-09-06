package garden

import (
	"context"
	"time"

	"hawx.me/code/arboretum/internal/data"
)

type DB interface {
	ReadAll(context.Context) ([]data.Feed, error)
	UpdateFeed(context.Context, data.Feed) error
	Fetched(context.Context, string, time.Time, int, error) error
	UpdatedAt(context.Context, string) (time.Time, error)
}
