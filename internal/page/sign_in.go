package page

import (
	"hawx.me/code/lmth"
	. "hawx.me/code/lmth/elements"
)

func SignIn() lmth.Node {
	return Html(lmth.Attr{"lang": "en"},
		pageHead,
		Body(lmth.Attr{},
			Div(lmth.Attr{"hidden": lmth.AttrSet, "class": "h-app"},
				H1(lmth.Attr{"class": "p-name"},
					A(lmth.Attr{"class": "u-url", "href": "/"}, lmth.Text("Arboretum")),
				),
			),
			Div(lmth.Attr{"id": "cover"},
				A(lmth.Attr{"href": "/sign-in"}, lmth.Text("Sign-in")),
			),
		),
	)
}
