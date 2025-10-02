package signin

import (
	"log/slog"
	"net/http"

	"hawx.me/code/arboretum/internal/page"
)

func Handler() http.HandlerFunc {
	signInPage := page.SignIn()

	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := signInPage.WriteTo(w); err != nil {
			slog.Error("sign-in", slog.Any("err", err))
		}
	}
}
