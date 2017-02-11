/*
 Package feed provides an RSS and Atom feed fetcher.

 They are parsed into an object tree which is a hybrid of both the RSS and Atom
 standards.

 Supported feeds are:
 	- RSS v0.91, 0.91 and 2.0
 	- Atom 1.0

 The package allows us to maintain cache timeout management. This prevents
 querying the servers for feed updates too often. Apart from setting a cache
 timeout manually, the package also optionally adheres to the TTL, SkipDays and
 SkipHours values specified in RSS feeds.

 Because the object structure is a hybrid between both RSS and Atom specs, not
 all fields will be filled when requesting either an RSS or Atom feed. As many
 shared fields as possible are used but some of them simply do not occur in
 either the RSS or Atom spec.
*/
package feed

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	xmlx "github.com/jteeuwen/go-pkg-xmlx"
)

const userAgent = "riviera golang"

type ItemHandler func(f *Feed, ch *Channel, newitems []*Item)

type Feed struct {
	// Custom cache timeout.
	cacheTimeout time.Duration

	// Type of feed. Rss, Atom, etc
	format string

	// Channels with content.
	channels []*Channel

	// Url from which this feed was created.
	url string

	// Known containing a list of known Items and Channels for this instance
	known Database

	// A notification function, used to notify the host when a new item
	// has been found for a given channel.
	itemhandler ItemHandler

	// Last time content was fetched. Used in conjunction with CacheTimeout
	// to ensure we don't get content too often.
	lastupdate time.Time

	// The latest value of the ETag header returned from the last fetch.
	eTag string
}

func New(cachetimeout time.Duration, ih ItemHandler, database Database) *Feed {
	v := new(Feed)
	v.cacheTimeout = cachetimeout
	v.format = "none"
	v.known = database
	v.itemhandler = ih
	return v
}

// Fetch retrieves the feed's latest content if necessary.
//
// The charset parameter overrides the xml decoder's CharsetReader.
// This allows us to specify a custom character encoding conversion
// routine when dealing with non-utf8 input. Supply 'nil' to use the
// default from Go's xml package.
//
// The client parameter allows the use of arbitrary network connections, for
// example the Google App Engine "URL Fetch" service.
func (f *Feed) Fetch(uri string, client *http.Client, charset xmlx.CharsetFunc) (int, error) {
	if !f.CanUpdate() {
		return -1, nil
	}

	f.url = uri

	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return -1, err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("If-Modified-Since", f.lastupdate.Format(time.RFC1123))
	if f.eTag != "" {
		req.Header.Set("If-None-Match", f.eTag)
	}

	resp, err := client.Do(req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}

	f.eTag = resp.Header.Get("ETag")

	return resp.StatusCode, f.load(resp.Body, charset)
}

func Parse(r io.Reader, charset xmlx.CharsetFunc) (chs []*Channel, err error) {
	doc := xmlx.New()

	if err = doc.LoadStream(r, charset); err != nil {
		return
	}

	format, version := GetVersionInfo(doc)
	if ok := testVersions(format, version); !ok {
		err = errors.New(fmt.Sprintf("Unsupported feed: %s, version: %+v", format, version))
		return
	}

	return buildFeed(format, doc)
}

func (f *Feed) load(r io.Reader, charset xmlx.CharsetFunc) (err error) {
	f.channels, err = Parse(r, charset)
	if err != nil || len(f.channels) == 0 {
		return
	}

	// reset cache timeout values according to feed specified values (TTL)
	if f.cacheTimeout < time.Minute*time.Duration(f.channels[0].TTL) {
		f.cacheTimeout = time.Minute * time.Duration(f.channels[0].TTL)
	}

	f.notifyListeners()
	return
}

func (f *Feed) notifyListeners() {
	for _, channel := range f.channels {
		var newitems []*Item

		for _, item := range channel.Items {
			if !f.known.Contains(item.Key()) {
				newitems = append(newitems, item)
			}
		}

		if len(newitems) > 0 && f.itemhandler != nil {
			f.itemhandler(f, channel, newitems)
		}
	}
}

// This function returns true or false, depending on whether the CacheTimeout
// value has expired or not. Additionally, it will ensure that we adhere to the
// RSS spec's SkipDays and SkipHours values. If this function returns true, you
// can be sure that a fresh feed update will be performed.
func (f *Feed) CanUpdate() bool {
	// Make sure we are not within the specified cache-limit.
	// This ensures we don't request data too often.
	utc := time.Now().UTC()
	if utc.Sub(f.lastupdate) < f.cacheTimeout {
		return false
	}

	// If skipDays or skipHours are set in the RSS feed, use these to see if
	// we can update.
	if len(f.channels) == 1 && f.format == "rss" {
		if len(f.channels[0].SkipDays) > 0 {
			for _, v := range f.channels[0].SkipDays {
				if time.Weekday(v) == utc.Weekday() {
					return false
				}
			}
		}

		if len(f.channels[0].SkipHours) > 0 {
			for _, v := range f.channels[0].SkipHours {
				if v == utc.Hour() {
					return false
				}
			}
		}
	}

	f.lastupdate = utc
	return true
}

// Returns the number of seconds needed to elapse
// before the feed should update.
func (f *Feed) DurationTillUpdate() time.Duration {
	return f.cacheTimeout - time.Now().UTC().Sub(f.lastupdate)
}

func buildFeed(format string, doc *xmlx.Document) ([]*Channel, error) {
	switch format {
	case "atom":
		return readAtom(doc)
	default:
		return readRss2(doc)
	}
}

func testVersions(format string, version [2]int) bool {
	switch format {
	case "rss":
		if version[0] > 2 || (version[0] == 2 && version[1] > 0) {
			return false
		}

	case "atom":
		if version[0] > 1 || (version[0] == 1 && version[1] > 0) {
			return false
		}

	default:
		return false
	}

	return true
}

func GetVersionInfo(doc *xmlx.Document) (string, [2]int) {
	if node := doc.SelectNode("http://www.w3.org/2005/Atom", "feed"); node != nil {
		return "atom", [2]int{1, 0}
	}

	if node := doc.SelectNode("", "rss"); node != nil {
		version := node.As("", "version")
		p := strings.Index(version, ".")
		major, _ := strconv.Atoi(version[0:p])
		minor, _ := strconv.Atoi(version[p+1 : len(version)])

		return "rss", [2]int{major, minor}
	}

	// issue#5: Some documents have an RDF root node instead of rss.
	if node := doc.SelectNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#", "RDF"); node != nil {
		return "rss", [2]int{1, 1}
	}

	return "unknown", [2]int{0, 0}
}
