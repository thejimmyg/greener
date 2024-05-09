package greener_test

import (
	"fmt"
	"bytes"
	"html/template"
)

// Define your HTML template as a string
var htmlTemplateStr = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Title}}</title>
</head>
<body>
  <header>{{template "header" .}}</header>
  <main>{{template "main" .}}</main>
  <footer>{{template "footer" .}}</footer>
</body>
</html>
{{define "header"}}Default Header{{end}}
{{define "main"}}Default Main Content{{end}}
{{define "footer"}}Default Footer{{end}}
`

func Example_template_html() {
	// Parse the template from the string
	tmpl, err := template.New("webpage").Parse(htmlTemplateStr)
	if err != nil {
		panic(err) // Handle error appropriately in real applications
	}

	tmpl, err = tmpl.Parse(`{{define "header"}}{{.Header}}{{end}}
                        {{define "main"}}{{.Main}}{{end}}
                        {{define "footer"}}{{.Footer}}{{end}}`)

	pageData := struct {
		Title     string
		Header    template.HTML
		Main      template.HTML
		Footer    template.HTML
	}{
		Title:     "My Dynamic Page",
		Header:    template.HTML("<h1>Custom Header Content</h1>"),
		Main:      template.HTML("<p>This is the main section with <strong>custom content</strong>.</p>"),
		Footer:    template.HTML("<p>© 2024 My Website</p>"),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, pageData); err != nil {
		fmt.Printf("error rendering page: %v", err)
	}
	fmt.Printf(buf.String())
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
