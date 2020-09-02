// Arboretum is a feed aggregator.
package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"time"

	"hawx.me/code/arboretum/internal/data"
	"hawx.me/code/arboretum/internal/garden"
	"hawx.me/code/arboretum/internal/signin"
	"hawx.me/code/arboretum/internal/subscriptions"
	"hawx.me/code/indieauth"
	"hawx.me/code/indieauth/sessions"
	"hawx.me/code/riviera/subscriptions/opml"
	"hawx.me/code/serve"
)

func printHelp() {
	fmt.Println(`Usage: arboretum [options]

  Arboretum is a feed aggregator.

   --refresh DUR='2h'
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
			log.Println(err)
		}
	}
	return nil
}

func parseTemplates(path string) (*template.Template, error) {
	return template.New("").Funcs(map[string]interface{}{
		"ago": func(t time.Time) string {
			dur := time.Now().Sub(t)
			if dur < time.Minute {
				return fmt.Sprintf("%vs", math.Ceil(dur.Seconds()))
			}
			if dur < time.Hour {
				return fmt.Sprintf("%vm", math.Ceil(dur.Minutes()))
			}
			if dur < 24*time.Hour {
				return fmt.Sprintf("%vh", math.Ceil(dur.Hours()))
			}
			if dur < 31*24*time.Hour {
				return fmt.Sprintf("%vd", math.Ceil(dur.Hours()/24))
			}
			if dur < 365*24*time.Hour {
				return fmt.Sprintf("%vM", math.Ceil(dur.Hours()/24/31))
			}

			return fmt.Sprintf("%vY", math.Ceil(dur.Hours()/24/365))
		},
	}).ParseGlob(path + "/template/*.gotmpl")
}

func importOpml(ctx context.Context, path, dbPath string) (int, error) {
	doc, err := opml.Load(path)
	if err != nil {
		return 0, err
	}

	log.Println("opening db at", dbPath)
	db, err := data.Open(dbPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	oks := 0
	for _, item := range doc.Body.Outline {
		if err := db.Subscribe(ctx, item.XMLURL); err != nil {
			log.Printf("error adding %s: %v\n", item.XMLURL, err)
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
		refresh = flag.String("refresh", "15m", "")
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

	if flag.Arg(0) == "import" {
		file := flag.Arg(1)
		fmt.Println("importing ", file)

		n, err := importOpml(ctx, file, *dbPath)
		if err != nil {
			log.Println(err)
		} else {
			log.Println("added", n)
		}
		return
	}

	auth, err := indieauth.Authentication(*url, *url+"/callback")
	if err != nil {
		log.Println(err)
		return
	}

	session, err := sessions.New(*me, *secret, auth)
	if err != nil {
		log.Println(err)
		return
	}

	cacheTimeout, err := time.ParseDuration(*refresh)
	if err != nil {
		log.Println(err)
		return
	}

	templates, err := parseTemplates(*webPath)
	if err != nil {
		log.Println(err)
		return
	}

	db, err := data.Open(*dbPath)
	if err != nil {
		log.Println(err)
		return
	}
	defer db.Close()

	garden := garden.New(db, cacheTimeout)

	go func() {
		garden.Run(ctx)
	}()

	if err = addSubs(ctx, db, garden); err != nil {
		log.Println(err)
		return
	}

	if *private {
		http.HandleFunc("/", session.Choose(
			garden.Handler(templates, true),
			signin.Handler(templates)))
	} else {
		http.HandleFunc("/", session.Choose(
			garden.Handler(templates, true),
			garden.Handler(templates, false)))
	}

	http.Handle("/public/", http.StripPrefix("/public",
		http.FileServer(http.Dir(*webPath+"/static"))))

	http.HandleFunc("/subscriptions.opml", session.Shield(
		subscriptions.List(db)))

	http.HandleFunc("/remove", session.Shield(
		subscriptions.Remove(db, garden)))

	http.HandleFunc("/add", session.Shield(
		subscriptions.Add(db, garden)))

	http.HandleFunc("/sign-in", session.SignIn())
	http.HandleFunc("/callback", session.Callback())
	http.HandleFunc("/sign-out", session.SignOut())

	serve.Serve(*port, *socket, http.DefaultServeMux)
}
