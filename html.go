// Package greener provides more efficient ways of building web applications
package greener

import (
	"fmt"
	"html/template"
)

// Text escapes some specical characters and returns a template.HTML which the html/template package will treat as HTML without further escaping.
func Text(t string) template.HTML {
	return template.HTML(template.HTMLEscapeString(t))
}

// HTMLPrintf takes a string containing %s characters and a set of template.HTML strings and returns an template.HTML with the placeholders substituted. This is faster than using template/html Template objects by about 8x but less safe in that no context specific checks about where you are substituing things are made.
func HTMLPrintf(h string, hws ...template.HTML) template.HTML {
	hs := []interface{}{}
	for _, hw := range hws {
		hs = append(hs, hw)
	}
	return template.HTML(fmt.Sprintf(h, hs...))
}
