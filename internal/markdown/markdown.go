package markdown

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"monks.co/ii/internal/text"
)

type termRenderer interface {
	Render(string) (string, error)
}

var (
	rendererMu sync.Mutex
	renderers  = map[int]termRenderer{}
)

var errRendererPanic = errors.New("markdown renderer panicked")

// Render formats markdown text for terminal output.
func Render(width, indent int, input []byte) []byte {
	return SafeRender(width, indent, input)
}

// SafeRender formats markdown text for terminal output, recovering from renderer panics.
//
// glamour has historically panicked on some markdown inputs; we prefer a best-effort
// render over crashing the CLI.
func SafeRender(width, indent int, input []byte) []byte {
	value := text.NormalizeNewlines(string(input))
	value = text.TrimTrailingNewlines(value)
	if text.IsBlank(value) {
		return nil
	}
	if width < 1 {
		width = 1
	}
	if indent < 0 {
		indent = 0
	}
	renderWidth := max(width-indent, 1)

	renderer := markdownRenderer(renderWidth)
	rendered := value
	if renderer != nil {
		formatted, err := safeRender(func() (string, error) {
			return renderer.Render(value)
		})
		if err == nil {
			rendered = formatted
		}
	}
	rendered = text.TrimLeadingNewlines(rendered)
	rendered = text.TrimTrailingNewlines(rendered)
	rendered = cleanRenderedMarkdown(rendered)
	if text.IsBlank(rendered) {
		return nil
	}
	if indent <= 0 {
		return []byte(rendered)
	}
	return []byte(text.IndentBlock(rendered, indent))
}

func safeRender(render func() (string, error)) (out string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			out = ""
			err = fmt.Errorf("%w: %v", errRendererPanic, recovered)
		}
	}()
	return render()
}

func cleanRenderedMarkdown(value string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = text.TrimTrailingWhitespace(line)
	}
	cleaned := strings.Join(lines, "\n")
	if text.IsBlank(cleaned) {
		return ""
	}
	return cleaned
}

func markdownRenderer(width int) termRenderer {
	rendererMu.Lock()
	defer rendererMu.Unlock()
	if cached, ok := renderers[width]; ok {
		return cached
	}
	style := styles.ASCIIStyleConfig
	style.Item.BlockPrefix = "- "
	style.ImageText.Format = "Image: {{.text}} ->"
	created, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil
	}
	renderers[width] = created
	return created
}
