package org

import (
	"fmt"
	"html"
	"strings"
	"unicode"

	h "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type HTMLWriter struct {
	stringBuilder
	HighlightCodeBlock    func(source, lang string) string
	FootnotesHeadingTitle string
	htmlEscape            bool
}

var emphasisTags = map[string][]string{
	"/":   []string{"<em>", "</em>"},
	"*":   []string{"<strong>", "</strong>"},
	"+":   []string{"<del>", "</del>"},
	"~":   []string{"<code>", "</code>"},
	"=":   []string{`<code class="verbatim">`, "</code>"},
	"_":   []string{`<span style="text-decoration: underline;">`, "</span>"},
	"_{}": []string{"<sub>", "</sub>"},
	"^{}": []string{"<sup>", "</sup>"},
}

var listTags = map[string][]string{
	"unordered":   []string{"<ul>", "</ul>"},
	"ordered":     []string{"<ol>", "</ol>"},
	"descriptive": []string{"<dl>", "</dl>"},
}

var listItemStatuses = map[string]string{
	" ": "unchecked",
	"-": "indeterminate",
	"X": "checked",
}

func NewHTMLWriter() *HTMLWriter {
	return &HTMLWriter{
		htmlEscape:            true,
		FootnotesHeadingTitle: "Footnotes",
		HighlightCodeBlock: func(source, lang string) string {
			return fmt.Sprintf("%s\n<pre>\n%s\n</pre>\n</div>", `<div class="highlight">`, html.EscapeString(source))
		},
	}
}

func (w *HTMLWriter) emptyClone() *HTMLWriter {
	wcopy := *w
	wcopy.stringBuilder = stringBuilder{}
	return &wcopy
}

func (w *HTMLWriter) nodesAsString(nodes ...Node) string {
	tmp := w.emptyClone()
	tmp.writeNodes(nodes...)
	return tmp.String()
}

func (w *HTMLWriter) before(d *Document) {}

func (w *HTMLWriter) after(d *Document) {
	w.writeFootnotes(d)
}

func (w *HTMLWriter) writeNodes(ns ...Node) {
	for _, n := range ns {
		switch n := n.(type) {
		case Keyword:
			w.writeKeyword(n)
		case Include:
			w.writeInclude(n)
		case Comment:
			continue
		case NodeWithMeta:
			w.writeNodeWithMeta(n)
		case Headline:
			w.writeHeadline(n)
		case Block:
			w.writeBlock(n)
		case Drawer:
			w.writeDrawer(n)

		case FootnoteDefinition:
			w.writeFootnoteDefinition(n)

		case List:
			w.writeList(n)
		case ListItem:
			w.writeListItem(n)
		case DescriptiveListItem:
			w.writeDescriptiveListItem(n)

		case Table:
			w.writeTable(n)

		case Paragraph:
			w.writeParagraph(n)
		case Example:
			w.writeExample(n)
		case HorizontalRule:
			w.writeHorizontalRule(n)
		case Text:
			w.writeText(n)
		case Emphasis:
			w.writeEmphasis(n)
		case StatisticToken:
			w.writeStatisticToken(n)
		case ExplicitLineBreak:
			w.writeExplicitLineBreak(n)
		case LineBreak:
			w.writeLineBreak(n)
		case RegularLink:
			w.writeRegularLink(n)
		case FootnoteLink:
			w.writeFootnoteLink(n)
		default:
			if n != nil {
				panic(fmt.Sprintf("bad node %#v", n))
			}
		}
	}
}

func (w *HTMLWriter) writeBlock(b Block) {
	content := ""
	if isRawTextBlock(b.Name) {
		exportWriter := w.emptyClone()
		exportWriter.htmlEscape = false
		exportWriter.writeNodes(b.Children...)
		content = strings.TrimRightFunc(exportWriter.String(), unicode.IsSpace)
	} else {
		content = w.nodesAsString(b.Children...)
	}
	switch name := b.Name; {
	case name == "SRC":
		lang := "text"
		if len(b.Parameters) >= 1 {
			lang = strings.ToLower(b.Parameters[0])
		}
		w.WriteString(w.HighlightCodeBlock(content, lang) + "\n")
	case name == "EXAMPLE":
		w.WriteString(`<pre class="example">` + "\n" + content + "\n</pre>\n")
	case name == "EXPORT" && len(b.Parameters) >= 1 && strings.ToLower(b.Parameters[0]) == "html":
		w.WriteString(content + "\n")
	case name == "QUOTE":
		w.WriteString("<blockquote>\n" + content + "</blockquote>\n")
	case name == "CENTER":
		w.WriteString(`<div class="center-block" style="text-align: center; margin-left: auto; margin-right: auto;">` + "\n")
		w.WriteString(content + "</div>\n")
	default:
		w.WriteString(fmt.Sprintf(`<div class="%s-block">`, strings.ToLower(b.Name)) + "\n")
		w.WriteString(content + "</div>\n")
	}
}

func (w *HTMLWriter) writeDrawer(d Drawer) {
	w.writeNodes(d.Children...)
}

func (w *HTMLWriter) writeKeyword(kw Keyword) {
	if k, v := kw.Key, kw.Value; k == "HTML" {
		w.WriteString(v + "\n")
	} else if k == "HUGO" && v == "more" {
		w.WriteString("<!--more-->\n")
	}
}

func (w *HTMLWriter) writeInclude(i Include) {
	w.writeNodes(i.Resolve())
}

func (w *HTMLWriter) writeFootnoteDefinition(f FootnoteDefinition) {
	n := f.Name
	w.WriteString(`<div class="footnote-definition">` + "\n")
	w.WriteString(fmt.Sprintf(`<sup id="footnote-%s"><a href="#footnote-reference-%s">%s</a></sup>`, n, n, n) + "\n")
	w.WriteString(`<div class="footnote-body">` + "\n")
	w.writeNodes(f.Children...)
	w.WriteString("</div>\n</div>\n")
}

func (w *HTMLWriter) writeFootnotes(d *Document) {
	fs := d.Footnotes
	if len(fs.Definitions) == 0 {
		return
	}
	w.WriteString(`<div class="footnotes">` + "\n")
	w.WriteString(`<h1 class="footnotes-title">` + fs.Title + `</h1>` + "\n")
	w.WriteString(`<div class="footnote-definitions">` + "\n")
	for _, definition := range d.Footnotes.Ordered() {
		w.writeNodes(definition)
	}
	w.WriteString("</div>\n</div>\n")
}

func (w *HTMLWriter) writeHeadline(h Headline) {
	title := w.nodesAsString(h.Title...)
	if h.Lvl == 1 && title == w.FootnotesHeadingTitle {
		return
	}
	w.WriteString(fmt.Sprintf("<h%d>\n", h.Lvl))
	if h.Status != "" {
		w.WriteString(fmt.Sprintf(`<span class="todo">%s</span>`, h.Status) + "\n")
	}
	if h.Priority != "" {
		w.WriteString(fmt.Sprintf(`<span class="priority">[%s]</span>`, h.Priority) + "\n")
	}

	w.WriteString(title)
	if len(h.Tags) != 0 {
		tags := make([]string, len(h.Tags))
		for i, tag := range h.Tags {
			tags[i] = fmt.Sprintf(`<span>%s</span>`, tag)
		}
		w.WriteString("&#xa0;&#xa0;&#xa0;")
		w.WriteString(fmt.Sprintf(`<span class="tags">%s</span>`, strings.Join(tags, "&#xa0;")))
	}
	w.WriteString(fmt.Sprintf("\n</h%d>\n", h.Lvl))
	w.writeNodes(h.Children...)
}

func (w *HTMLWriter) writeText(t Text) {
	if !w.htmlEscape {
		w.WriteString(t.Content)
	} else if t.IsRaw {
		w.WriteString(html.EscapeString(t.Content))
	} else {
		w.WriteString(html.EscapeString(htmlEntityReplacer.Replace(t.Content)))
	}
}

func (w *HTMLWriter) writeEmphasis(e Emphasis) {
	tags, ok := emphasisTags[e.Kind]
	if !ok {
		panic(fmt.Sprintf("bad emphasis %#v", e))
	}
	w.WriteString(tags[0])
	w.writeNodes(e.Content...)
	w.WriteString(tags[1])
}

func (w *HTMLWriter) writeStatisticToken(s StatisticToken) {
	w.WriteString(fmt.Sprintf(`<code class="statistic">[%s]</code>`, s.Content))
}

func (w *HTMLWriter) writeLineBreak(l LineBreak) {
	w.WriteString(strings.Repeat("\n", l.Count))
}

func (w *HTMLWriter) writeExplicitLineBreak(l ExplicitLineBreak) {
	w.WriteString("<br>\n")
}

func (w *HTMLWriter) writeFootnoteLink(l FootnoteLink) {
	n := html.EscapeString(l.Name)
	w.WriteString(fmt.Sprintf(`<sup class="footnote-reference"><a id="footnote-reference-%s" href="#footnote-%s">%s</a></sup>`, n, n, n))
}

func (w *HTMLWriter) writeRegularLink(l RegularLink) {
	url := html.EscapeString(l.URL)
	if l.Protocol == "file" {
		url = url[len("file:"):]
	}
	description := url
	if l.Description != nil {
		description = w.nodesAsString(l.Description...)
	}
	switch l.Kind() {
	case "image":
		w.WriteString(fmt.Sprintf(`<img src="%s" alt="%s" title="%s" />`, url, description, description))
	case "video":
		w.WriteString(fmt.Sprintf(`<video src="%s" title="%s">%s</video>`, url, description, description))
	default:
		w.WriteString(fmt.Sprintf(`<a href="%s">%s</a>`, url, description))
	}
}

func (w *HTMLWriter) writeList(l List) {
	tags, ok := listTags[l.Kind]
	if !ok {
		panic(fmt.Sprintf("bad list kind %#v", l))
	}
	w.WriteString(tags[0] + "\n")
	w.writeNodes(l.Items...)
	w.WriteString(tags[1] + "\n")
}

func (w *HTMLWriter) writeListItem(li ListItem) {
	if li.Status != "" {
		w.WriteString(fmt.Sprintf("<li class=\"%s\">\n", listItemStatuses[li.Status]))
	} else {
		w.WriteString("<li>\n")
	}
	w.writeNodes(li.Children...)
	w.WriteString("</li>\n")
}

func (w *HTMLWriter) writeDescriptiveListItem(di DescriptiveListItem) {
	if di.Status != "" {
		w.WriteString(fmt.Sprintf("<dt class=\"%s\">\n", listItemStatuses[di.Status]))
	} else {
		w.WriteString("<dt>\n")
	}

	if len(di.Term) != 0 {
		w.writeNodes(di.Term...)
	} else {
		w.WriteString("?")
	}
	w.WriteString("<dd>\n")
	w.writeNodes(di.Details...)
	w.WriteString("<dd>\n")
}

func (w *HTMLWriter) writeParagraph(p Paragraph) {
	if isEmptyLineParagraph(p) {
		return
	}
	w.WriteString("<p>")
	if _, ok := p.Children[0].(LineBreak); !ok {
		w.WriteString("\n")
	}
	w.writeNodes(p.Children...)
	w.WriteString("\n</p>\n")
}

func (w *HTMLWriter) writeExample(e Example) {
	w.WriteString(`<pre class="example">` + "\n")
	if len(e.Children) != 0 {
		for _, n := range e.Children {
			w.writeNodes(n)
			w.WriteString("\n")
		}
	}
	w.WriteString("</pre>\n")
}

func (w *HTMLWriter) writeHorizontalRule(h HorizontalRule) {
	w.WriteString("<hr>\n")
}

func (w *HTMLWriter) writeNodeWithMeta(n NodeWithMeta) {
	out := w.nodesAsString(n.Node)
	if p, ok := n.Node.(Paragraph); ok {
		if len(p.Children) == 1 && isImageOrVideoLink(p.Children[0]) {
			out = w.nodesAsString(p.Children[0])
		}
	}
	for _, attributes := range n.Meta.HTMLAttributes {
		out = withHTMLAttributes(out, attributes...) + "\n"
	}
	if len(n.Meta.Caption) != 0 {
		caption := ""
		for i, ns := range n.Meta.Caption {
			if i != 0 {
				caption += " "
			}
			caption += w.nodesAsString(ns...)
		}
		out = fmt.Sprintf("<figure>\n%s<figcaption>\n%s\n</figcaption>\n</figure>\n", out, caption)
	}
	w.WriteString(out)
}

func (w *HTMLWriter) writeTable(t Table) {
	w.WriteString("<table>\n")
	beforeFirstContentRow := true
	for i, row := range t.Rows {
		if row.IsSpecial || len(row.Columns) == 0 {
			continue
		}
		if beforeFirstContentRow {
			beforeFirstContentRow = false
			if i+1 < len(t.Rows) && len(t.Rows[i+1].Columns) == 0 {
				w.WriteString("<thead>\n")
				w.writeTableColumns(row.Columns, "th")
				w.WriteString("</thead>\n<tbody>\n")
				continue
			} else {
				w.WriteString("<tbody>\n")
			}
		}
		w.writeTableColumns(row.Columns, "td")
	}
	w.WriteString("</tbody>\n</table>\n")
}

func (w *HTMLWriter) writeTableColumns(columns []Column, tag string) {
	w.WriteString("<tr>\n")
	for _, column := range columns {
		if column.Align == "" {
			w.WriteString(fmt.Sprintf("<%s>", tag))
		} else {
			w.WriteString(fmt.Sprintf(`<%s class="align-%s">`, tag, column.Align))
		}
		w.writeNodes(column.Children...)
		w.WriteString(fmt.Sprintf("</%s>\n", tag))
	}
	w.WriteString("</tr>\n")
}

func withHTMLAttributes(input string, kvs ...string) string {
	if len(kvs)%2 != 0 {
		panic(fmt.Sprintf("len of kvs must be even: %#v", kvs))
	}
	context := &h.Node{Type: h.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := h.ParseFragment(strings.NewReader(strings.TrimSpace(input)), context)
	if err != nil || len(nodes) != 1 {
		panic(fmt.Sprintf("could not extend html attributes of %s: %v (%s)", input, len(nodes), err))
	}
	out, node := strings.Builder{}, nodes[0]
	for i := 0; i < len(kvs)-1; i += 2 {
		node.Attr = setHTMLAttribute(node.Attr, strings.TrimPrefix(kvs[i], ":"), kvs[i+1])
	}
	err = h.Render(&out, nodes[0])
	if err != nil {
		panic(fmt.Sprintf("could not extend html attributes of %s: %#v (%s)", input, nodes, err))
	}
	return out.String()
}

func setHTMLAttribute(attributes []h.Attribute, k, v string) []h.Attribute {
	for i, a := range attributes {
		if strings.ToLower(a.Key) == strings.ToLower(k) {
			switch strings.ToLower(k) {
			case "class", "style":
				attributes[i].Val += " " + v
			default:
				attributes[i].Val = v
			}
			return attributes
		}
	}
	return append(attributes, h.Attribute{Namespace: "", Key: k, Val: v})
}
