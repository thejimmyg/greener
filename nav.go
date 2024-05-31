package greener

import (
	"bytes"
	"fmt"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/util"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yuin/goldmark"
)

type Page struct {
	Title    string
	URL      string
	Content  string // Path to markdown file in embedded FS
	HTML     template.HTML
	once     sync.Once
	Children []*Page
	Section  *Section
}

type Section struct {
	Title    string
	Children []*Section
	Page     *Page
	parent   *Section
}

// var rootSection *Section
// var pageMap map[string]*Page

type PageHandler struct {
	PagesFS fs.FS
	EmptyPageProvider
	PageMap     map[string]*Page
	RootSection *Section
}

func (h *PageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if strings.HasSuffix(path, "/") {
		http.Redirect(w, r, path+"index.html", http.StatusTemporaryRedirect)
		return
	}
	if p, ok := h.PageMap[path]; ok {
		if err := p.ConvertMarkdownToHTML(h.PagesFS, path); err != nil {
			fmt.Printf("%v\n", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		var breadcrumbs template.HTML
		if p.Section != h.RootSection {
			breadcrumbs = GenerateBreadcrumbs(path, p)
		}
		sectionNav := GenerateSectionNav(p, false, "section", path)
		renderTemplate(w, h.Page, p.Title, breadcrumbs, sectionNav, p.HTML, path)
	} else {
		http.Error(w, "404 Not Found", http.StatusNotFound)
	}
}

func BuildPageInSectionMap(s *Section, pageMap map[string]*Page) {
	if s.Page != nil {
		pageMap[s.Page.URL] = s.Page
		buildPageMap(s.Page, s, pageMap) // Build the page map for child pages
	}
	for _, child := range s.Children {
		child.parent = s
		BuildPageInSectionMap(child, pageMap)
	}
}

func buildPageMap(p *Page, s *Section, pageMap map[string]*Page) {
	p.Section = s
	pageMap[p.URL] = p
	for _, child := range p.Children {
		buildPageMap(child, s, pageMap) // Recursively add all child pages
	}
}

func GenerateBreadcrumbs(currentPath string, p *Page) template.HTML {
	dir := filepath.Dir(currentPath)
	crumbs := []template.HTML{}
	for s := p.Section; s != nil; s = s.parent {
		if s.Page != nil { // Ensure there is a page to link to
			title := s.Title
			if title == "" {
				title = s.Page.Title
			}
			if s == p.Section && s.Page == p {
				crumbs = append(crumbs, HTMLPrintf("      <li>%s</li>\n", Text(title)))
			} else {
				crumbs = append(crumbs, HTMLPrintf("      <li><a href=\"%s\">%s</a></li>\n", Text(relativeURL(dir, s.Page.URL)), Text(s.Title)))
			}
		}
	}
	// Reverse the crumbs
	for i, j := 0, len(crumbs)-1; i < j; i, j = i+1, j-1 {
		crumbs[i], crumbs[j] = crumbs[j], crumbs[i]
	}
	return HTMLPrintf(`    <ul class="breadcrumbs">
%s    </ul>
`, ConcatenateHTML(crumbs, ""))
}

func generateChildSectionsNav(currentPath string, currentPage *Page) template.HTML {
	dir := filepath.Dir(currentPath)
	childSections := []template.HTML{}
	for _, section := range currentPage.Section.Children {
		childSections = append(childSections, HTMLPrintf(`      <li class="section"><a href="%s">%s</a>
`, Text(relativeURL(dir, section.Page.URL)), Text(section.Title)))
	}
	return ConcatenateHTML(childSections, "")
}

func GenerateSectionNav(currentPage *Page, linkEverything bool, class string, currentPath string) template.HTML {
	dir := filepath.Dir(currentPath)
	s := currentPage.Section
	p := s.Page

	navBuilder := &HTMLBuilder{}
	navBuilder.Printf(`    <ul class="%s">
`, Text(class))
	if (currentPage == p && !linkEverything) || p.URL == currentPath {
		navBuilder.Printf(`      <li class="sectionhome">%s</li>
`, Text(p.Title))
	} else {
		navBuilder.Printf(`      <li class="sectionhome"><a href="%s">%s</a></li>
`, Text(relativeURL(dir, p.URL)), Text(p.Title))
	}
	appendChildPagesNav(currentPage, navBuilder, p.Children, linkEverything, currentPath)
	if class != "sitemap" {
		navBuilder.WriteHTML(generateChildSectionsNav(currentPath, currentPage))
	}
	navBuilder.WriteHTML(template.HTML("    </ul>\n"))
	return navBuilder.HTML()
}

func appendChildPagesNav(currentPage *Page, navBuilder *HTMLBuilder, pages []*Page, linkEverything bool, currentPath string) {
	dir := filepath.Dir(currentPath)
	for _, page := range pages {
		if (currentPage == page && !linkEverything) || page.URL == currentPath {
			navBuilder.Printf("      <li>%s</li>\n", Text(page.Title))
		} else {
			navBuilder.Printf(`      <li><a href="%s">%s</a></li>
`, Text(relativeURL(dir, page.URL)), Text(page.Title))
		}
		if len(page.Children) > 0 {
			navBuilder.WriteHTML(template.HTML("    <ul>\n"))
			appendChildPagesNav(currentPage, navBuilder, page.Children, linkEverything, currentPath)
			navBuilder.WriteHTML(template.HTML("    </ul>\n"))
		}
	}
}

func ConvertMarkdownToHTML(mdContent []byte, currentPath string) (template.HTML, error) {
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
	if err := md.Convert(mdContent, buffer); err != nil {
		return template.HTML(""), err
	}
	b := buffer.Bytes()
	return template.HTML(AddIndentation(b[:len(b)-1], "    ")), nil
}

func (p *Page) ConvertMarkdownToHTML(pagesFS fs.FS, currentPath string) error {
	var err error
	p.once.Do(func() {
		var mdContent []byte
		mdContent, err = fs.ReadFile(pagesFS, p.Content)
		if err == nil {
			p.HTML, err = ConvertMarkdownToHTML(mdContent, currentPath)
		}
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
	html := page(pageTitle, HTMLPrintf("%s%s%s", breadcrumbs, sectionNav, content), currentPath)
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
