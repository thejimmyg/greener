package main

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
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

func init() {
	rootSection = &Section{
		Title: "Home",
		Page: &Page{
			Title:   "Home",
			URL:     "/",
			Content: "pages/home.md",
			Children: []*Page{
				&Page{
					Title: "Sitemap",
					URL:   "/sitemap",
				},
			},
		},
		Children: []*Section{
			{
				Title: "About",
				Page: &Page{
					Title:   "About Us",
					URL:     "/about",
					Content: "pages/about.md",
				},
				Children: []*Section{
					{
						Title: "History",
						Page: &Page{
							Title:   "Our History",
							URL:     "/about/history",
							Content: "pages/about_history.md",
						},
					},
					{
						Title: "About Team",
						Page: &Page{
							Title:   "About Team",
							URL:     "/about/team",
							Content: "pages/about_team.md",
							Children: []*Page{
								{
									Title:   "Team Awards",
									URL:     "/about/team/awards",
									Content: "pages/about_team_awards.md",
									Children: []*Page{

										{
											Title:   "2023",
											URL:     "/about/team/awards/2023",
											Content: "pages/about_team_awards_2023.md",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	pageMap = make(map[string]*Page)
	buildPageInSectionMap(rootSection)
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

func generateBreadcrumbs(p *Page) template.HTML {
	crumbs := []template.HTML{}
	for s := p.section; s != nil; s = s.parent {
		if s.Page != nil { // Ensure there is a page to link to
			title := s.Title
			if title == "" {
				title = s.Page.Title
			}
			if s == p.section && s.Page == p {
				crumbs = append(crumbs, greener.HTMLPrintf("<li>%s</li>", greener.Text(title)))
			} else {
				crumbs = append(crumbs, greener.HTMLPrintf("<li><a href='%s'>%s</a></li>", greener.Text(s.Page.URL), greener.Text(s.Title)))
			}
		}
	}
	// Reverse the crumbs
	for i, j := 0, len(crumbs)-1; i < j; i, j = i+1, j-1 {
		crumbs[i], crumbs[j] = crumbs[j], crumbs[i]
	}
	return greener.HTMLPrintf(`<ul class="breadcrumbs">%s</ul>`, greener.ConcatenateHTML(crumbs, "\n"))
}

func generateChildSectionsNav(currentPage *Page) template.HTML {
	childSections := []template.HTML{}
	for _, section := range currentPage.section.Children {
		childSections = append(childSections, greener.HTMLPrintf(`<li class="section"><a href="%s">%s</a>`, greener.Text(section.Page.URL), greener.Text(section.Title)))
	}
	return greener.ConcatenateHTML(childSections, "\n")
}

func generateSectionNav(currentPage *Page, linkEverything bool, class string, url string) template.HTML {
	s := currentPage.section
	p := s.Page

	navBuilder := &greener.HTMLBuilder{}
	navBuilder.Printf(`<ul class="%s">`, greener.Text(class))
	if (currentPage == p && !linkEverything) || p.URL == url {
		navBuilder.Printf(`<li class="sectionhome">%s</li>`, greener.Text(p.Title))
	} else {
		navBuilder.Printf(`<li class="sectionhome"><a href='%s'>%s</a></li>`, greener.Text(p.URL), greener.Text(p.Title))
	}
	appendChildPagesNav(currentPage, navBuilder, p.Children, linkEverything, url)
	if class != "sitemap" {
		navBuilder.WriteHTML(generateChildSectionsNav(currentPage))
	}
	navBuilder.WriteHTML(template.HTML("</ul>"))
	return navBuilder.HTML()
}

func appendChildPagesNav(currentPage *Page, navBuilder *greener.HTMLBuilder, pages []*Page, linkEverything bool, url string) {
	for _, page := range pages {
		if (currentPage == page && !linkEverything) || page.URL == url {
			navBuilder.Printf("<li>%s</li>", greener.Text(page.Title))
		} else {
			navBuilder.Printf("<li><a href='%s'>%s</a></li>", greener.Text(page.URL), greener.Text(page.Title))
		}
		if len(page.Children) > 0 {
			navBuilder.WriteHTML(template.HTML("<ul>"))
			appendChildPagesNav(currentPage, navBuilder, page.Children, linkEverything, url)
			navBuilder.WriteHTML(template.HTML("</ul>"))
		}
		//navBuilder.WriteHTML(template.HTML("</li>"))
	}
}

// Convert Markdown content to HTML, caching the result
func (p *Page) ConvertMarkdownToHTML() error {
	var err error
	p.once.Do(func() {
		var mdContent []byte
		mdContent, err = fs.ReadFile(pageFiles, p.Content)
		if err != nil {
			return
		}
		buffer := new(bytes.Buffer)
		if err = goldmark.Convert(mdContent, buffer); err != nil {
			return
		}
		// We trust the markdown renderer
		p.HTML = template.HTML(buffer.Bytes())
	})
	return err
}

func renderTemplate(w http.ResponseWriter, page func(string, template.HTML) template.HTML, pageTitle string, breadcrumbs, sectionNav, content template.HTML) {
	html := page(pageTitle, greener.HTMLPrintf(`%s %s %s`, breadcrumbs, sectionNav, content))
	w.Write([]byte(html))
}

func generateSitemapHTML(s *Section, depth int, url string) template.HTML {
	var builder greener.HTMLBuilder
	tag := fmt.Sprintf("h%d", depth+1)
	builder.Printf(`<%s>%s</%s>`, greener.Text(tag), greener.Text(s.Title), greener.Text(tag))
	if s.Page != nil {
		sectionNavHTML := generateSectionNav(s.Page, true, "sitemap", url)
		builder.WriteHTML(sectionNavHTML)
	}
	for _, child := range s.Children {
		childHTML := generateSitemapHTML(child, depth+1, url) // Increment depth for child sections
		builder.WriteHTML(childHTML)
	}
	return builder.HTML()
}
