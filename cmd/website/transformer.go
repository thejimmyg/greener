package main

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"strings"
)

type mdLinkTransformer struct{}

func (t *mdLinkTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	ast.Walk(node, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		if !enter {
			return ast.WalkContinue, nil
		}
		if link, ok := n.(*ast.Link); ok {
			linkDest := string(link.Destination)
			if strings.HasSuffix(linkDest, "/index.md") {
				link.Destination = []byte(strings.TrimSuffix(linkDest, "index.md"))
			} else if strings.HasSuffix(linkDest, ".md") {
				link.Destination = []byte(strings.TrimSuffix(linkDest, ".md"))
			}
		}
		return ast.WalkContinue, nil
	})
}
