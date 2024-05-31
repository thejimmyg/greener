package greener

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"path/filepath"
	"strings"
)

type mdLinkTransformer struct {
	currentPath string
}

func (t *mdLinkTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	dir := filepath.Dir(t.currentPath)
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if link, ok := n.(*ast.Link); ok {
				dest := string(link.Destination)
				filename := t.currentPath[len(dir):]
				if !strings.HasPrefix(dest, "http") {
					link.Destination = []byte(strings.TrimSuffix(relative(dir, filename, dest), ".md") + ".html")
				}
			}
		}
		return ast.WalkContinue, nil
	})
}

func relative(dir, filename, dest string) string {
	if strings.HasPrefix(dest, "/") {
		relativePath, _ := filepath.Rel(dir, dest)
		return relativePath
	} else {
		// fmt.Printf("Dir: %s Filename: %s Dest: %s\n", dir, filename, dest)
		if strings.HasPrefix(dest, "./") {
			dest = dest[2:]
			if dest == "./" {
				return dest
			}
		}
		if dest == "." || dest == "" {
			if strings.HasPrefix(filename, "/") {
				filename = filename[1:]
			}
			filename = strings.TrimSuffix(filename, ".html") + ".md"
			dest = filename
		}
		return dest
	}
}
