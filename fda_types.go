package creamery

import "strconv"

// Label represents an FDA nutrition label parsed from .fda format.
type Label struct {
	ID            string
	Name          string
	PintMassGrams float64
}

func (p *fdaParser) parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// ParseLabel parses an FDA label from the given content string.
func ParseLabel(content string) (Label, error) {
	p := &fdaParser{Buffer: content}
	if err := p.Init(); err != nil {
		return Label{}, err
	}
	if err := p.Parse(); err != nil {
		return Label{}, err
	}
	p.Execute()
	return p.label, nil
}
