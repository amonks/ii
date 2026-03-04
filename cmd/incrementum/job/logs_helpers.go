package job

type eventAppender interface {
	Append(Event) error
	Len() int
	String() string
}

func appendEventOutput(writer eventAppender, event Event) (string, error) {
	if writer == nil {
		return "", nil
	}
	start := writer.Len()
	if err := writer.Append(event); err != nil {
		return "", err
	}
	output := writer.String()
	if len(output) <= start {
		return "", nil
	}
	return output[start:], nil
}
