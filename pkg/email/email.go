package email

type Message struct {
	Subject string
	Body    string
}

func EmailMe(message Message) error {
	return nil
}
