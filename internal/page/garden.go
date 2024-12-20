package page

import (
	"fmt"
	"math"
	"time"

	"hawx.me/code/arboretum/internal/gardenjs"
	"hawx.me/code/lmth"
	. "hawx.me/code/lmth/elements"
	"hawx.me/code/lmth/escape"
)

func Garden(signedIn bool, where string, feeds []gardenjs.Feed) lmth.Node {
	return Html(lmth.Attr{"lang": "en"},
		pageHead,
		Body(lmth.Attr{},
			Header(lmth.Attr{},
				Div(lmth.Attr{"class": "h-app"},
					H1(lmth.Attr{"class": "p-name"},
						A(lmth.Attr{"class": "u-url", "href": "/"}, lmth.Text("Arboretum")),
					),
				),
				menu(signedIn),
			),

			Form(lmth.Attr{"action": "/add", "method": "post", "data-toggled": "add"},
				Input(lmth.Attr{"name": "where", "type": "hidden", "value": where}),
				Div(lmth.Attr{},
					Label(lmth.Attr{"for": "url"}, lmth.Text("URL")),
					Input(lmth.Attr{"name": "url", "id": "url", "type": "text"}),
				),
				Button(lmth.Attr{"type": "submit"}, lmth.Text("Add")),
			),

			Main(lmth.Attr{"class": "garden"},
				Ul(lmth.Attr{},
					lmth.Map(func(feed gardenjs.Feed) lmth.Node {
						return Li(lmth.Attr{"data-toggled": escape.Attr(feed.URL)},
							A(lmth.Attr{"data-toggled": "edit", "href": "/remove?where=garden&url=" + escape.Query(feed.URL), "class": "remove"},
								lmth.Text("x"),
							),
							H2(lmth.Attr{},
								A(lmth.Attr{"href": escape.URL(feed.WebsiteURL)}, lmth.Text(feed.Title)),
							),
							lmth.Text(" "),
							Time(lmth.Attr{"datetime": feed.UpdatedAt.Format(time.RFC3339)}, lmth.Text(ago(feed.UpdatedAt))),
							Code(lmth.Attr{"data-toggled": "edit"}, lmth.Text("<"+feed.URL+">")),
							Span(lmth.Attr{"class": "toggle", "data-toggle": escape.Attr(feed.URL)}, lmth.Text("∴")),
							Ol(lmth.Attr{},
								lmth.Map(func(item gardenjs.Item) lmth.Node {
									return Li(lmth.Attr{},
										H3(lmth.Attr{},
											A(lmth.Attr{"href": escape.URL(item.PermaLink)}, lmth.Text(item.Title)),
										),
										Time(lmth.Attr{"datetime": item.PubDate.Format(time.RFC3339)}, lmth.Text(ago(item.PubDate))),
									)
								}, feed.Items),
							),
						)
					}, feeds),
				),
			),
			Script(lmth.Attr{"src": "/public/toggle.js"}),
		),
	)
}

func menu(signedIn bool) lmth.Node {
	if signedIn {
		return Ul(lmth.Attr{"class": "actions"},
			Li(lmth.Attr{},
				A(lmth.Attr{"data-toggle": "add", "href": "#"}, lmth.Text("add")),
			),
			Li(lmth.Attr{},
				A(lmth.Attr{"data-toggle": "edit", "href": "#"}, lmth.Text("edit")),
			),
			Li(lmth.Attr{},
				A(lmth.Attr{"href": "/sign-out"}, lmth.Text("sign-out")),
			),
		)
	} else {
		return Ul(lmth.Attr{"class": "actions"},
			Li(lmth.Attr{},
				A(lmth.Attr{"href": "/sign-in"}, lmth.Text("sign-in")),
			),
		)
	}
}

func ago(t time.Time) string {
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
}
