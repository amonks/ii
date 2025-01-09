package main

import (
	"fmt"
	"net/http"

	"monks.co/pkg/email"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/ports"
	"monks.co/pkg/util"
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	port := ports.Apps["mailer"]

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

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	http.ListenAndServe(addr, gzip.Middleware(mux))

	return nil
}
