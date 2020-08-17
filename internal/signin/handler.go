package signin

import (
	"io"
	"log"
	"net/http"
)

type ExecuteTemplate interface {
	ExecuteTemplate(io.Writer, string, interface{}) error
}

func Handler(templates ExecuteTemplate) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := templates.ExecuteTemplate(w, "sign-in.gotmpl", nil); err != nil {
			log.Println("/sign-in:", err)
		}
	}
}
