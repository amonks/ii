package ui

import (
	"os"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"golang.org/x/term"
)

const tableCellMaxWidth = 50
const tableCellEllipsis = "..."
const tableColumnPadding = 2

var tableViewportWidth = detectTableViewportWidth
var tableIsTerminal = term.IsTerminal
var tableGetSize = term.GetSize

// TableViewportWidth reports the configured viewport width.
func TableViewportWidth() int {
	return tableViewportWidth()
}

// OverrideTableViewportWidth replaces the viewport width provider.
func OverrideTableViewportWidth(fn func() int) func() {
	original := tableViewportWidth
	tableViewportWidth = fn
	return func() {
		tableViewportWidth = original
	}
}

// TableCellWidth reports the display width of a table cell.
func TableCellWidth(value string) int {
	return displayWidth(normalizeTableCell(value))
}

// TableCellMaxWidth reports the default maximum width for table cells.
func TableCellMaxWidth() int {
	return tableCellMaxWidth
}

// TableColumnPaddingWidth reports the padding between columns.
func TableColumnPaddingWidth() int {
	return tableColumnPadding
}

// TableBuilder collects rows and renders a formatted table.
type TableBuilder struct {
	headers []string
	rows    [][]string
}

// NewTableBuilder returns a builder with preallocated rows.
func NewTableBuilder(headers []string, capacity int) *TableBuilder {
	return &TableBuilder{headers: headers, rows: make([][]string, 0, capacity)}
}

// AddRow appends a row to the table.
func (builder *TableBuilder) AddRow(row []string) {
	builder.rows = append(builder.rows, row)
}

// String renders the table output.
func (builder *TableBuilder) String() string {
	return FormatTable(builder.headers, builder.rows)
}

// FormatTable renders headers and rows as an aligned table.
func FormatTable(headers []string, rows [][]string) string {
	normalizedHeaders := make([]string, len(headers))
	for i, header := range headers {
		normalizedHeaders[i] = normalizeTableCell(header)
	}

	normalizedRows := make([][]string, 0, len(rows))
	columnCount := len(normalizedHeaders)
	for _, row := range rows {
		normalizedRow := make([]string, len(row))
		for i, cell := range row {
			normalizedRow[i] = normalizeTableCell(cell)
		}
		normalizedRows = append(normalizedRows, normalizedRow)
		if len(normalizedRow) > columnCount {
			columnCount = len(normalizedRow)
		}
	}

	builder := table.New().
		Headers(normalizedHeaders...).
		Rows(normalizedRows...).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderHeader(false).
		BorderColumn(false).
		BorderRow(false).
		Wrap(false).
		StyleFunc(func(_, col int) lipgloss.Style {
			if columnCount > 1 && col < columnCount-1 {
				return lipgloss.NewStyle().PaddingRight(tableColumnPadding)
			}
			return lipgloss.NewStyle()
		})

	if width := tableViewportWidth(); width > 0 {
		builder.Width(width)
	}

	output := builder.String()
	if !strings.HasSuffix(output, "\n") {
		output += "\n"
	}
	return output
}

// TruncateTableCell limits cell width while preserving visible characters.
func TruncateTableCell(value string) string {
	return TruncateTableCellToWidth(value, tableCellMaxWidth)
}

// TruncateTableCellToWidth limits cell width while preserving visible characters.
func TruncateTableCellToWidth(value string, max int) string {
	value = normalizeTableCell(value)
	if max <= 0 {
		return ""
	}
	if displayWidth(value) <= max {
		return value
	}

	ellipsisWidth := displayWidth(tableCellEllipsis)
	if max <= ellipsisWidth {
		return truncateVisible(tableCellEllipsis, max)
	}

	maxVisible := max - ellipsisWidth
	return truncateVisible(value, maxVisible) + tableCellEllipsis
}

func displayWidth(value string) int {
	return lipgloss.Width(stripANSICodes(value))
}

func normalizeTableCell(value string) string {
	return strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ").Replace(value)
}

func truncateVisible(value string, max int) string {
	if max <= 0 {
		return ""
	}

	var builder strings.Builder
	visible := 0
	for i := 0; i < len(value); {
		if value[i] == '\x1b' {
			end := i + 1
			if end < len(value) && value[end] == '[' {
				end++
				for end < len(value) && value[end] != 'm' {
					end++
				}
				if end < len(value) {
					end++
				}
				builder.WriteString(value[i:end])
				i = end
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(value[i:])
		if r == utf8.RuneError && size == 1 {
			if visible >= max {
				break
			}
			builder.WriteByte(value[i])
			visible++
			i++
			continue
		}
		width := lipgloss.Width(string(r))
		if visible+width > max {
			break
		}
		builder.WriteRune(r)
		visible += width
		i += size
	}
	return builder.String()
}

func stripANSICodes(input string) string {
	var builder strings.Builder
	inEscape := false
	for i := 0; i < len(input); i++ {
		char := input[i]
		if inEscape {
			if char == 'm' {
				inEscape = false
			}
			continue
		}
		if char == '\x1b' {
			inEscape = true
			continue
		}
		builder.WriteByte(char)
	}
	return builder.String()
}

func detectTableViewportWidth() int {
	if width := detectTerminalWidth(os.Stdout.Fd()); width > 0 {
		return width
	}
	return detectTerminalWidth(os.Stderr.Fd())
}

func detectTerminalWidth(fd uintptr) int {
	if !tableIsTerminal(int(fd)) {
		return 0
	}
	width, _, err := tableGetSize(int(fd))
	if err != nil || width <= 0 {
		return 0
	}
	return width
}
