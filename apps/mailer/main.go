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
		subj := req.Form.Get("subject")
		body := req.Form.Get("body")
		if err := email.EmailMe(email.Message{
			Subject: subj,
			Body:    body,
		}); err != nil {
			util.HTTPError("mailer", w, req, http.StatusInternalServerError, "%s", err)
		}
	}))

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	http.ListenAndServe(addr, gzip.Handler(mux))

	return nil
}
