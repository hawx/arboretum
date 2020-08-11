package data

import (
	"testing"

	"hawx.me/code/assert"
)

func TestFeedDB(t *testing.T) {
	assert := assert.Wrap(t)

	db, err := Open("file:TestFeedDB?cache=shared&mode=memory")
	assert(err).Must.Nil()
	defer db.Close()

	feedDB, err := db.Feed("what")
	assert(err).Must.Nil()

	ok := feedDB.Contains("hey")
	if assert(ok).False() {
		ok = feedDB.Contains("hey")
		assert(ok).True()
	}
}

func TestBucket(t *testing.T) {
	assert := assert.Wrap(t)

	db, err := Open("file:TestBucket?cache=shared&mode=memory")
	assert(err).Must.Nil()
	defer db.Close()

	bucket, err := db.Feed("test")
	assert(err).Nil()

	key := "1"
	assert(bucket.Contains(key)).False()
	assert(bucket.Contains(key)).True()

	bucket2, err := db.Feed("test2")
	assert(err).Nil()

	assert(bucket2.Contains(key)).False()
	assert(bucket2.Contains(key)).True()
}
