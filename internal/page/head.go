package page

import (
	"hawx.me/code/lmth"
	. "hawx.me/code/lmth/elements"
)

var pageHead = Head(lmth.Attr{},
	Meta(lmth.Attr{"charset": "utf-8"}),
	Meta(lmth.Attr{"viewport": "width=device-width, initial-scale=1.0"}),
	Title(lmth.Attr{}, lmth.Text("Arboretum")),
	Link(lmth.Attr{"rel": "stylesheet", "href": "/public/styles.css", "type": "text/css"}),
)
