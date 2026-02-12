package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"monks.co/pkg/dogs"
	"monks.co/pkg/env"
	"monks.co/pkg/errlogger"
	"monks.co/pkg/gzip"
	"monks.co/pkg/serve"
	"monks.co/pkg/sigctx"
	"monks.co/pkg/tailnet"
	"monks.co/pkg/templib"
)

func main() {
	log.Printf("start")
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		log.Fatal(err)
	}
	log.Printf("done")
}

//go:embed migrate.sql
var migrateSQL string

var (
	archiveDir = "/data/tank/hotdogs"
)

func run() error {
	if _, err := os.Stat(archiveDir); os.IsNotExist(err) {
		archiveDir = env.InMonksData("dogs")
	} else if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(archiveDir, "images"), os.ModePerm); err != nil {
		return err
	}

	flag.Parse()

	log.Printf("opening db")
	db, err := dogs.NewDB(archiveDir)
	if err != nil {
		return err
	}

	log.Printf("migrating db")
	if err := db.DB.Exec(migrateSQL).Error; err != nil {
		return err
	}

	imp := dogs.NewImporter(db, archiveDir)

	mux := serve.NewMux()
	mux.Handle("GET /images/", http.StripPrefix("/images/", http.FileServer(http.Dir(filepath.Join(archiveDir, "images")))))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, req *http.Request) {
		q := req.URL.Query()

		qOpts := dogs.QueryOptions{}

		selectedCombatantSet := map[string]struct{}{}
		for _, c := range q["combatants"] {
			log.Printf("has: %s", c)
			selectedCombatantSet[c] = struct{}{}
			qOpts.Combatants = append(qOpts.Combatants, c)
		}

		sortBy := "number"
		if len(q["sortBy"]) > 0 {
			sortBy = q["sortBy"][0]
		}
		sortOrder := "descending"
		if len(q["sortOrder"]) > 0 {
			sortOrder = q["sortOrder"][0]
		}

		filters := dogs.FilterData{
			Combatants: []templib.Checkbox{
				{Label: "Monks"},
				{Label: "Ben"},
				{Label: "Chris"},
				{Label: "Fenn"},
				{Label: "Savely"},
				{Label: "ellie"},
			},
			SortBy: []templib.Checkbox{
				{Label: "number", IsSelected: sortBy == "number"},
				{Label: "count", IsSelected: sortBy == "count"},
				{Label: "wordcount", IsSelected: sortBy == "wordcount"},
			},
			SortOrder: []templib.Checkbox{
				{Label: "ascending", IsSelected: sortOrder == "ascending"},
				{Label: "descending", IsSelected: sortOrder == "descending"},
			},
		}
		for i, f := range filters.Combatants {
			if _, has := selectedCombatantSet[f.Label]; has {
				filters.Combatants[i].IsSelected = true
			}
		}
		qOpts.Sort = fmt.Sprintf("%s %s", sortBy, map[string]string{"ascending": "asc", "descending": "desc"}[sortOrder])

		entries, err := db.All(qOpts)
		if err != nil {
			log.Println(err)
			return
		}

		if err := dogs.Page(entries, filters, imp.String()).Render(req.Context(), w); err != nil {
			log.Println(err)
		}
	})

	ctx := sigctx.New()
	ctx, cancel := context.WithCancel(ctx)
	errs := make(chan error)

	go func() {
		log.Println("starting importer")
		err := imp.Start(ctx)
		if !errors.Is(err, context.Canceled) {
			cancel()
		}
		errs <- err
	}()

	go func() {
		log.Println("starting server")
		err := tailnet.ListenAndServe(ctx, gzip.Middleware(mux))
		if !errors.Is(err, context.Canceled) {
			cancel()
		}
		errs <- err
	}()

	err = <-errs
	err = errors.Join(err, <-errs)

	return err
}
