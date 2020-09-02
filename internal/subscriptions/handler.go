package subscriptions

import (
	"context"
	"encoding/xml"
	"log"
	"net/http"

	"hawx.me/code/riviera/subscriptions/opml"
)

func Add(subs ...interface {
	Subscribe(context.Context, string) error
}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uri := r.FormValue("url")

		for _, sub := range subs {
			if err := sub.Subscribe(r.Context(), uri); err != nil {
				log.Println(err)
			}
		}
		log.Println("subscribed to", uri)

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func Remove(subs ...interface {
	Unsubscribe(context.Context, string) error
}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uri := r.FormValue("url")

		for _, sub := range subs {
			if err := sub.Unsubscribe(r.Context(), uri); err != nil {
				log.Println(err)
			}
		}
		log.Println("unsubscribed from", uri)

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func List(subs interface {
	Subscriptions(context.Context) ([]string, error)
}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := subs.Subscriptions(r.Context())
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var outlines []opml.Outline
		for _, uri := range list {
			outlines = append(outlines, opml.Outline{
				XMLURL: uri,
			})
		}

		data := opml.Opml{
			Version: "1.0",
			Head: opml.Head{
				Title: "arboretum subscriptions",
			},
			Body: opml.Body{
				Outline: outlines,
			},
		}

		if err := xml.NewEncoder(w).Encode(data); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
