//go:build !noexample
// +build !noexample

package greener_test

import (
	"fmt"
	"github.com/thejimmyg/greener"
	"html/template"
)

func Page(title string, body BodyBlock) template.HTML {
	return greener.HTMLPrintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
</head>
<body>%s</body>
</html>
`, greener.Text(title), body.Body())
}

type BodyBlock interface {
	Body() template.HTML
}
type HeaderMainFooter struct {
	Header, Main, Footer template.HTML
}

func (h *HeaderMainFooter) Body() template.HTML {
	return greener.HTMLPrintf(`
  <header>%s</header>
  <main>%s</main>
  <footer>%s</footer>
`, h.Header, h.Main, h.Footer)
}

func Webpage(title string, header, main, footer template.HTML) template.HTML {
	return Page(title, &HeaderMainFooter{header, main, footer})
}

func Example_templates() {
	fmt.Printf("%v\n", Webpage(
		"My Dynamic Page",
		template.HTML("<h1>Custom Header Content</h1>"),
		template.HTML("<p>This is the main section with <strong>custom content</strong>.</p>"),
		template.HTML("<p>© 2024 My Website</p>"),
	))
	// Output: <!DOCTYPE html>
	// <html lang="en">
	// <head>
	//   <meta charset="UTF-8">
	//   <meta name="viewport" content="width=device-width, initial-scale=1.0">
	//   <title>My Dynamic Page</title>
	// </head>
	// <body>
	//   <header><h1>Custom Header Content</h1></header>
	//   <main><p>This is the main section with <strong>custom content</strong>.</p></main>
	//   <footer><p>© 2024 My Website</p></footer>
	// </body>
	// </html>
}
