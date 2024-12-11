package garden

import (
	"log"
	"net/http"

	"hawx.me/code/arboretum/internal/page"
)

func (garden *Garden) Handler(signedIn bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		latest, err := garden.Latest(r.Context())
		if err != nil {
			log.Println("/garden:", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		if _, err := page.Garden(signedIn, "garden", latest.Feeds).WriteTo(w); err != nil {
			log.Println("/garden:", err)
		}
	}
}
