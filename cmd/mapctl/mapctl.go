package main

import (
	"flag"
	"fmt"

	"monks.co/apps/map/model"
	"monks.co/pkg/errlogger"
)

var (
	operation = flag.String("operation", "", "'import-saved-places' or 'annotate-peoples-places'")
)

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	db, err := model.NewModel()
	if err != nil {
		return fmt.Errorf("constructing model: %w", err)
	}
	defer db.Close()

	flag.Parse()
	switch *operation {
	case "import-saved-places":
		if err := db.ImportSavedPlaces(); err != nil {
			return err
		}
	case "annotate-peoples-places":
		if err := db.AnnotatePeoplesPlaces(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("operation must be 'import-saved-places' or 'annotate-peoples-places'")
	}

	return nil
}
