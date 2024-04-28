package greener

import (
	"fmt"
	"html"
	"io"
	"strings"
)

type HTMLable interface {
	HTML() HTML
	WriteHTMLTo(w io.Writer)
}

type HTML string

func (s HTML) HTML() HTML {
	return s
}
func (s HTML) WriteHTMLTo(w io.Writer) {
	w.Write([]byte(s.HTML()))
}

type Text string

func (t Text) HTML() HTML {
	return HTML(html.EscapeString(string(t)))
}
func (t Text) WriteHTMLTo(w io.Writer) {
	w.Write([]byte(t.HTML()))
}

type HTMLSlice []HTML

func (hs HTMLSlice) HTML() HTML {
	var builder strings.Builder
	for _, h := range hs {
		builder.WriteString(string(h))
	}
	return HTML(builder.String())
}
func (hs HTMLSlice) WriteHTMLTo(w io.Writer) {
	w.Write([]byte(hs.HTML()))
}

func HTMLPrintf(h string, hws ...HTMLable) HTML {
	hs := []interface{}{}
	for _, hw := range hws {
		if hw != nil {
			hs = append(hs, hw.HTML())
		}
	}
	return HTML(fmt.Sprintf(h, hs...))
}
