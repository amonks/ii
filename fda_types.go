package creamery

// Label represents an FDA nutrition label parsed from .fda format.
type Label struct {
	ID string
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
