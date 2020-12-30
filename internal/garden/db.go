package garden

import (
	"context"
	"time"

	"hawx.me/code/arboretum/internal/data"
)

type DB interface {
	ReadAll(context.Context) ([]data.Feed, error)
	UpdateFeed(context.Context, data.Feed) error
	UpdatedAt(context.Context, string) (time.Time, error)
	SetUpdatedAt(context.Context, string, time.Time) error
}
