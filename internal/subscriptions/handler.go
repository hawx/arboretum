package subscriptions

import (
	"log"
	"net/http"
)

func Add(subs ...interface{ Subscribe(string) error }) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uri := r.FormValue("url")

		for _, sub := range subs {
			if err := sub.Subscribe(uri); err != nil {
				log.Println(err)
			}
		}
		log.Println("subscribed to", uri)

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func Remove(subs ...interface{ Unsubscribe(string) error }) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uri := r.FormValue("url")

		for _, sub := range subs {
			if err := sub.Unsubscribe(uri); err != nil {
				log.Println(err)
			}
		}
		log.Println("unsubscribed from", uri)

		http.Redirect(w, r, "/", http.StatusFound)
	}
}
