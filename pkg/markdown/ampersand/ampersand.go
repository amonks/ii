package ampersand

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Ampersand is a goldmark extension that wraps ampersands in a span with class
// "ampersand", so that one can follow the "use the best available ampersand"
// principle.
var Ampersand = &ampersand{}

type ampersand struct{}

var _ goldmark.Extender = &ampersand{}

func (*ampersand) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		util.Prioritized(&inlineParser{}, 500),
	))
}

type inlineParser struct{}

var _ parser.InlineParser = &inlineParser{}

func (*inlineParser) Trigger() []byte {
	return []byte{'&'}
}

func (*inlineParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	c := line[0]
	if c == '&' && (len(line) == 1 || line[1] == ' ') {
		node := ast.NewString([]byte(`<span class="ampersand">&</span>`))
		node.SetCode(true)
		block.Advance(1)
		return node
	}
	return nil
}
