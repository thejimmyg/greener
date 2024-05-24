package main

import (
	"fmt"
	"io/fs"
	"log"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func init() {
	markdown, err := fs.ReadFile(pageFiles, "pages/sitemap.md")
	if err != nil {
		log.Fatalf("Error reading sitemap markdown: %s", err)
	}

	rootSection, err = parseSitemap(markdown)
	if err != nil {
		log.Fatalf("Error parsing sitemap markdown: %s", err)
	}

	pageMap = make(map[string]*Page)
	buildPageInSectionMap(rootSection)
}

// Function to parse the sitemap from markdown content
func parseSitemap(content []byte) (*Section, error) {
	md := goldmark.New()
	document := md.Parser().Parse(text.NewReader(content))

	var currentSection *Section
	var root *Section
	var currentPage *Page // Tracks the current page for nesting pages correctly

	ast.Walk(document, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := n.(type) {
		case *ast.Heading:
			level := node.Level
			headingText := string(node.Text(content))

			if level == 1 { // Skip h1
				return ast.WalkContinue, nil
			}

			// Manage sections based on heading levels
			if root == nil && level == 2 {
				root = &Section{Title: headingText}
				currentSection = root
			} else if level >= 2 {
				// Adjust current section to correct level
				newSectionLevel := level - 1 // Convert markdown heading level to depth level for handling
				for currentSection != nil && newSectionLevel <= currentSection.level() {
					currentSection = currentSection.parent
				}
				newSection := &Section{Title: headingText, parent: currentSection}
				if currentSection != nil {
					currentSection.Children = append(currentSection.Children, newSection)
				}
				currentSection = newSection
			}
			currentPage = nil // Reset current page at the start of a new section

		case *ast.ListItem:
			if firstChild := node.FirstChild(); firstChild != nil {
				linkNode := firstChild.FirstChild()
				if link, ok := linkNode.(*ast.Link); ok {
					title := string(link.Title)
					if title == "" {
						title = string(link.Text(content))
					}
					url := strings.TrimSuffix(string(link.Destination), ".md")
					contentPath := "pages" + string(link.Destination)
					page := &Page{
						Title:   title,
						URL:     url,
						Content: contentPath,
					}

					// Determine where to attach the page
					if currentPage == nil {
						currentSection.Page = page
					} else {
						currentPage.Children = append(currentPage.Children, page)
					}
					currentPage = page // Set as the current page for nesting further pages
				}
			}
		}

		return ast.WalkContinue, nil
	})

	return root, nil
}

func (s *Section) level() int {
	if s.parent == nil {
		return 1
	}
	return s.parent.level() + 1
}

// Helper function to dump page details and their children recursively
func dumpPage(p *Page, depth int) {
	if p == nil {
		return
	}

	indent := strings.Repeat(" ", depth*2) // Create indentation based on the depth
	fmt.Printf("%sPage: Title: %s, URL: %s, Content: %s\n", indent, p.Title, p.URL, p.Content)

	// Recursively dump child pages
	for _, child := range p.Children {
		dumpPage(child, depth+1)
	}
}

// Function to dump section details and traverse child sections and pages
func dumpSection(s *Section, depth int) {
	if s == nil {
		return
	}

	indent := strings.Repeat(" ", depth*2) // Create indentation based on the depth
	fmt.Printf("%sSection: %s\n", indent, s.Title)

	// If the section has an associated page, dump the page and its children
	if s.Page != nil {
		// fmt.Printf("%s  Page: Title: %s, URL: %s\n", indent, s.Page.Title, s.Page.URL)
		dumpPage(s.Page, depth+1) // Also print child pages if any
	} else {
		fmt.Printf("%s  No pages\n", indent)
	}

	// Recursively dump child sections
	for _, child := range s.Children {
		dumpSection(child, depth+1)
	}
}
