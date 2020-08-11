package subscriptions

import (
	"log"
	"net/http"
)

type Add interface {
	Add(string) error
}

func AddHandler(subs ...Add) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uri := r.FormValue("url")

		for _, sub := range subs {
			if err := sub.Add(uri); err != nil {
				log.Println(err)
			}
		}
		log.Println("subscribed to", uri)

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

type Remove interface {
	Remove(string) error
}

func RemoveHandler(subs ...Remove) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uri := r.FormValue("url")

		for _, sub := range subs {
			if err := sub.Remove(uri); err != nil {
				log.Println(err)
			}
		}
		log.Println("unsubscribed from", uri)

		http.Redirect(w, r, "/", http.StatusFound)
	}
}
