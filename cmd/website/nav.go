package main

import (
	"bytes"
	"embed"
	"fmt"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/thejimmyg/greener"
	"github.com/yuin/goldmark"
)

// Embed the markdown files located in the pages directory
//
//go:embed pages/*
var pageFiles embed.FS

type Page struct {
	Title    string
	URL      string
	Content  string // Path to markdown file in embedded FS
	HTML     template.HTML
	once     sync.Once
	Children []*Page
	section  *Section
}

type Section struct {
	Title    string
	Children []*Section
	Page     *Page
	parent   *Section
}

var rootSection *Section
var pageMap map[string]*Page

type PageHandler struct {
	greener.EmptyPageProvider
}

func (h *PageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if strings.HasSuffix(path, "/") {
		http.Redirect(w, r, path+"index.html", http.StatusTemporaryRedirect)
		return
	}
	if p, ok := pageMap[path]; ok {
		if err := p.ConvertMarkdownToHTML(path); err != nil {
			fmt.Printf("%v\n", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		var breadcrumbs template.HTML
		if p.section != rootSection {
			breadcrumbs = generateBreadcrumbs(path, p)
		}
		sectionNav := generateSectionNav(p, false, "section", path)
		renderTemplate(w, h.Page, p.Title, breadcrumbs, sectionNav, p.HTML, path)
	} else {
		http.Error(w, "404 Not Found", http.StatusNotFound)
	}
}

func buildPageInSectionMap(s *Section) {
	if s.Page != nil {
		pageMap[s.Page.URL] = s.Page
		buildPageMap(s.Page, s) // Build the page map for child pages
	}
	for _, child := range s.Children {
		child.parent = s
		buildPageInSectionMap(child)
	}
}

func buildPageMap(p *Page, s *Section) {
	p.section = s
	pageMap[p.URL] = p
	for _, child := range p.Children {
		buildPageMap(child, s) // Recursively add all child pages
	}
}

func generateBreadcrumbs(currentPath string, p *Page) template.HTML {
	dir := filepath.Dir(currentPath)
	crumbs := []template.HTML{}
	for s := p.section; s != nil; s = s.parent {
		if s.Page != nil { // Ensure there is a page to link to
			title := s.Title
			if title == "" {
				title = s.Page.Title
			}
			if s == p.section && s.Page == p {
				crumbs = append(crumbs, greener.HTMLPrintf("      <li>%s</li>\n", greener.Text(title)))
			} else {
				crumbs = append(crumbs, greener.HTMLPrintf("      <li><a href=\"%s\">%s</a></li>\n", greener.Text(relativeURL(dir, s.Page.URL)), greener.Text(s.Title)))
			}
		}
	}
	// Reverse the crumbs
	for i, j := 0, len(crumbs)-1; i < j; i, j = i+1, j-1 {
		crumbs[i], crumbs[j] = crumbs[j], crumbs[i]
	}
	return greener.HTMLPrintf(`    <ul class="breadcrumbs">
%s    </ul>
`, greener.ConcatenateHTML(crumbs, ""))
}

func generateChildSectionsNav(currentPath string, currentPage *Page) template.HTML {
	dir := filepath.Dir(currentPath)
	childSections := []template.HTML{}
	for _, section := range currentPage.section.Children {
		childSections = append(childSections, greener.HTMLPrintf(`      <li class="section"><a href="%s">%s</a>
`, greener.Text(relativeURL(dir, section.Page.URL)), greener.Text(section.Title)))
	}
	return greener.ConcatenateHTML(childSections, "")
}

func generateSectionNav(currentPage *Page, linkEverything bool, class string, currentPath string) template.HTML {
	dir := filepath.Dir(currentPath)
	s := currentPage.section
	p := s.Page

	navBuilder := &greener.HTMLBuilder{}
	navBuilder.Printf(`    <ul class="%s">
`, greener.Text(class))
	if (currentPage == p && !linkEverything) || p.URL == currentPath {
		navBuilder.Printf(`      <li class="sectionhome">%s</li>
`, greener.Text(p.Title))
	} else {
		navBuilder.Printf(`      <li class="sectionhome"><a href="%s">%s</a></li>
`, greener.Text(relativeURL(dir, p.URL)), greener.Text(p.Title))
	}
	appendChildPagesNav(currentPage, navBuilder, p.Children, linkEverything, currentPath)
	if class != "sitemap" {
		navBuilder.WriteHTML(generateChildSectionsNav(currentPath, currentPage))
	}
	navBuilder.WriteHTML(template.HTML("    </ul>\n"))
	return navBuilder.HTML()
}

func appendChildPagesNav(currentPage *Page, navBuilder *greener.HTMLBuilder, pages []*Page, linkEverything bool, currentPath string) {
	dir := filepath.Dir(currentPath)
	for _, page := range pages {
		if (currentPage == page && !linkEverything) || page.URL == currentPath {
			navBuilder.Printf("      <li>%s</li>\n", greener.Text(page.Title))
		} else {
			navBuilder.Printf(`      <li><a href="%s">%s</a></li>
`, greener.Text(relativeURL(dir, page.URL)), greener.Text(page.Title))
		}
		if len(page.Children) > 0 {
			navBuilder.WriteHTML(template.HTML("    <ul>\n"))
			appendChildPagesNav(currentPage, navBuilder, page.Children, linkEverything, currentPath)
			navBuilder.WriteHTML(template.HTML("    </ul>\n"))
		}
	}
}

func (p *Page) ConvertMarkdownToHTML(currentPath string) error {
	var err error
	p.once.Do(func() {
		var mdContent []byte
		mdContent, err = fs.ReadFile(pageFiles, p.Content)
		if err != nil {
			return
		}
		buffer := new(bytes.Buffer)

		// Create a new Markdown processor with the custom transformer
		md := goldmark.New(
			goldmark.WithRendererOptions(),
			goldmark.WithParserOptions(
				parser.WithASTTransformers(
					util.Prioritized(&mdLinkTransformer{currentPath: currentPath}, 100), // Use an instance of the struct
				),
			),
		)

		// Convert Markdown to HTML
		if err = md.Convert(mdContent, buffer); err != nil {
			return
		}
		b := buffer.Bytes()
		p.HTML = template.HTML(AddIndentation(b[:len(b)-1], "    "))
	})
	return err
}

func AddIndentation(input []byte, indent string) string {
	lines := strings.Split(string(input), "\n")
	for i, line := range lines {
		if line != "" { // Optionally skip indenting empty lines
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}

func renderTemplate(w http.ResponseWriter, page func(string, template.HTML, string) template.HTML, pageTitle string, breadcrumbs, sectionNav, content template.HTML, currentPath string) {
	html := page(pageTitle, greener.HTMLPrintf("%s%s%s", breadcrumbs, sectionNav, content), currentPath)
	w.Write([]byte(html))
}

func relativeURL(dir, dest string) string {
	// fmt.Printf("Dir: %s Dest: %s\n", dir, dest)
	if strings.HasPrefix(dest, "/") {
		relativePath, _ := filepath.Rel(dir, dest)
		return relativePath
	} else {
		return dest
	}
}
