package templates

import "html/template"

type CenterForm struct {
	Title  string
	Header template.HTML
	Form   template.HTML
	CSS    template.HTML
	JS     template.HTML
}
