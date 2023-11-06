package main

import (
	"flag"
	"fmt"
	"net/http"

	"monks.co/pkg/email"
	"monks.co/pkg/gzip"
	"monks.co/pkg/util"
)

var (
	port int
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	flag.IntVar(&port, "port", 3000, "port")
	flag.Parse()

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.ParseForm()
		subject := req.FormValue("subject")
		if subject == "" {
			util.HTTPError("mailer", w, req, http.StatusBadRequest, "'subject' is required")
			return
		}
		body := req.FormValue("body")
		if body == "" {
			util.HTTPError("mailer", w, req, http.StatusBadRequest, "'body' is required")
			return
		}

		if err := email.EmailMe(email.Message{
			Subject: subject,
			Body:    body,
		}); err != nil {
			util.HTTPError("mailer", w, req, http.StatusInternalServerError, "%s", err)
			return
		}
	}))

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	http.ListenAndServe(addr, gzip.Middleware(mux))

	return nil
}
