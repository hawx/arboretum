package river

import (
	"log"
	"net/http"
	"time"

	"code.google.com/p/go-charset/charset"
	_ "code.google.com/p/go-charset/data"
	"hawx.me/code/riviera/feed"
	"hawx.me/code/riviera/river/internal/persistence"
	"hawx.me/code/riviera/river/models"
)

type tributary struct {
	OnUpdate func(models.Feed)
	OnStatus func(int)

	uri     string
	feed    *feed.Feed
	client  *http.Client
	mapping Mapping
	quit    chan struct{}
}

func newTributary(store persistence.Bucket, uri string, cacheTimeout time.Duration, mapping Mapping) *tributary {
	p := &tributary{
		OnUpdate: func(models.Feed) {},
		OnStatus: func(int) {},
		uri:      uri,
		mapping:  mapping,
		quit:     make(chan struct{}),
	}

	p.feed = feed.New(cacheTimeout, p.itemHandler, store)
	p.client = &http.Client{Timeout: time.Minute, Transport: &statusTransport{http.DefaultTransport.(*http.Transport), p}}

	return p
}

func (t *tributary) Uri() string {
	return t.uri
}

func (t *tributary) Poll() {
	log.Println("started fetching", t.uri)
	t.fetch()

loop:
	for {
		select {
		case <-t.quit:
			break loop
		case <-time.After(t.feed.DurationTillUpdate()):
			log.Println("fetching", t.uri)
			t.fetch()
		}
	}

	log.Println("stopped fetching", t.uri)
}

type statusTransport struct {
	*http.Transport
	trib *tributary
}

// RoundTrip performs a RoundTrip using the underlying Transport, but then
// checks if the status returned was a 301 MovedPermanently. If so it modifies
// the underlying uri which will then be saved to the subscriptions next time it
// is fetched.
func (t *statusTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	resp, err = t.Transport.RoundTrip(req)
	if err != nil {
		return
	}

	if resp.StatusCode == http.StatusMovedPermanently {
		newLoc := resp.Header.Get("Location")
		log.Println(t.trib.uri, "moved to", newLoc)
		t.trib.uri = newLoc
	}

	return
}

// fetch retrieves the feed for the tributary.
func (t *tributary) fetch() {
	code, err := t.feed.Fetch(t.uri, t.client, charset.NewReader)
	t.OnStatus(code)

	if err != nil {
		log.Println("error fetching", t.uri+":", code, err)
		return
	}
}

func (t *tributary) itemHandler(feed *feed.Feed, ch *feed.Channel, newitems []*feed.Item) {
	items := []models.Item{}
	for _, item := range newitems {
		converted := t.mapping(item)

		if converted != nil {
			items = append(items, *converted)
		}
	}

	log.Println(len(items), "new item(s) in", t.uri)
	if len(items) == 0 {
		return
	}

	feedUrl := t.uri
	websiteUrl := ""
	for _, link := range ch.Links {
		if feedUrl != "" && websiteUrl != "" {
			break
		}

		if link.Rel == "self" {
			feedUrl = link.Href
		} else {
			websiteUrl = link.Href
		}
	}

	t.OnUpdate(models.Feed{
		FeedUrl:         feedUrl,
		WebsiteUrl:      websiteUrl,
		FeedTitle:       ch.Title,
		FeedDescription: ch.Description,
		WhenLastUpdate:  models.RssTime{time.Now()},
		Items:           items,
	})
}

func (t *tributary) Kill() {
	close(t.quit)
}
