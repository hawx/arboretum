package subscriptions

import (
	"context"
	"log"
	"net/http"
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
