package signin

import (
	"log"
	"net/http"

	"hawx.me/code/arboretum/internal/page"
)

func Handler() http.HandlerFunc {
	signInPage := page.SignIn()

	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := signInPage.WriteTo(w); err != nil {
			log.Println("/sign-in:", err)
		}
	}
}
