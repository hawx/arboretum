// Arboretum is a feed aggregator.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"hawx.me/code/arboretum/internal/data"
	"hawx.me/code/arboretum/internal/garden"
	"hawx.me/code/arboretum/internal/signin"
	"hawx.me/code/arboretum/internal/subscriptions"
	"hawx.me/code/indieauth/v2"
	"hawx.me/code/riviera/subscriptions/opml"
	"hawx.me/code/serve"
)

func printHelp() {
	fmt.Println(`Usage: arboretum [options]

Arboretum is a feed aggregator.

	--refresh DUR='6h'
		Time to refresh feeds after. This is the default used, but if
		advice is given in the feed itself it may be ignored.

	--private
		Prevent showing any feeds when not signed in.

	--db PATH=':memory:'
		Use the sqlitedb file at the given path.

	--url URL='http://localhost:8080/'
		URL arboretum is hosted at.

	--secret BASE64
		Base64 string to use for the cookie secret.

	--me URL
		Your profile URL used for authenticating with IndieAuth.

	--web PATH='web'
		Path to the 'web' directory.

	--port PORT='8080'
		Serve on given port.

	--socket SOCK
		Serve at given socket, instead.`)
}

func addSubs(
	ctx context.Context,
	from interface {
		Subscriptions(context.Context) ([]string, error)
	},
	to interface {
		Subscribe(context.Context, string) error
	},
) error {
	subs, err := from.Subscriptions(ctx)
	if err != nil {
		return err
	}

	for _, sub := range subs {
		if err := to.Subscribe(ctx, sub); err != nil {
			slog.Error("add subscription", slog.String("sub", sub), slog.Any("err", err))
		}
	}

	return nil
}

func importOpml(ctx context.Context, path, dbPath string) (int, error) {
	doc, err := opml.Load(path)
	if err != nil {
		return 0, err
	}

	slog.Info("opening db", slog.String("path", dbPath))
	db, err := data.Open(dbPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	oks := 0
	for _, item := range doc.Body.Outline {
		if err := db.Subscribe(ctx, item.XMLURL); err != nil {
			slog.Error("add subscription", slog.String("sub", item.XMLURL), slog.Any("err", err))
		} else {
			oks++
		}
	}

	return oks, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		refresh = flag.String("refresh", "6h", "")
		private = flag.Bool("private", false, "")

		dbPath = flag.String("db", ":memory:", "")

		url    = flag.String("url", "http://localhost:8080", "")
		secret = flag.String("secret", "GpgGqpnfFkpjgXj7u3RCdKkoOf/tQqbHkOuuys90Ds4=", "")
		me     = flag.String("me", "", "")

		webPath = flag.String("web", "web", "")
		port    = flag.String("port", "8080", "")
		socket  = flag.String("socket", "", "")
	)

	flag.Usage = func() { printHelp() }
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	if flag.Arg(0) == "import" {
		file := flag.Arg(1)
		fmt.Println("importing ", file)

		n, err := importOpml(ctx, file, *dbPath)
		if err != nil {
			slog.Error("import opml", slog.Any("err", err))
		} else {
			slog.Info("import opml", slog.Int("added", n))
		}
		return
	}

	if *me == "" {
		slog.Error("--me must be specified")
		os.Exit(1)
	}

	session, err := indieauth.NewSessions(*secret, &indieauth.Config{
		ClientID:    *url,
		RedirectURL: *url + "/callback",
	})
	if err != nil {
		slog.Error("new indieauth session", slog.Any("err", err))
		return
	}

	cacheTimeout, err := time.ParseDuration(*refresh)
	if err != nil {
		slog.Error("parse --refresh", slog.Any("err", err))
		return
	}

	db, err := data.Open(*dbPath)
	if err != nil {
		slog.Error("open database", slog.Any("err", err))
		return
	}
	defer db.Close()

	garden := garden.New(db, cacheTimeout)

	go func() {
		garden.Run(ctx)
	}()

	if err := addSubs(ctx, db, garden); err != nil {
		slog.Error("add subscriptions", slog.Any("err", err))
		return
	}

	choose := func(a, b http.Handler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if response, ok := session.SignedIn(r); ok && response.Me == *me {
				a.ServeHTTP(w, r)
			} else {
				b.ServeHTTP(w, r)
			}
		}
	}

	signedIn := func(a http.Handler) http.HandlerFunc {
		return choose(a, http.NotFoundHandler())
	}

	if *private {
		http.HandleFunc("/", choose(
			garden.Handler(true),
			signin.Handler()))
	} else {
		http.HandleFunc("/", choose(
			garden.Handler(true),
			garden.Handler(false)))
	}

	http.Handle("/public/", http.StripPrefix("/public",
		http.FileServer(http.Dir(*webPath+"/static"))))

	http.HandleFunc("/subscriptions.opml", signedIn(
		subscriptions.List(db)))

	http.HandleFunc("/remove", signedIn(
		subscriptions.Remove(db, garden)))

	http.HandleFunc("/add", signedIn(
		subscriptions.Add(db, garden)))

	http.HandleFunc("/sign-in", func(w http.ResponseWriter, r *http.Request) {
		if err := session.RedirectToSignIn(w, r, *me); err != nil {
			slog.Error("sign-in", slog.Any("err", err))
		}
	})
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if err := session.Verify(w, r); err != nil {
			slog.Error("callback", slog.Any("err", err))
		}
		http.Redirect(w, r, "/", http.StatusFound)
	})
	http.HandleFunc("/sign-out", func(w http.ResponseWriter, r *http.Request) {
		if err := session.SignOut(w, r); err != nil {
			slog.Error("sign-out", slog.Any("err", err))
		}
		http.Redirect(w, r, "/", http.StatusFound)
	})

	serve.Serve(*port, *socket, http.DefaultServeMux)
}
