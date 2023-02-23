package moviecopier

import (
	"log"
	"os"

	"monks.co/movietagger/system"
)

type MovieCopier struct {
	system.System
}

func New(system system.System) *MovieCopier {
	return &MovieCopier{system}
}

func (app *MovieCopier) Run() error {
	logfile, err := os.OpenFile("moviecopier.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer logfile.Close()
	logger := log.New(logfile, "", log.Ldate | log.Ltime)

	for {
		logger.Println("getting next movie ID...")
		id, err := app.DB.GetMovieIDToImport()
		if err != nil {
			logger.Println(err)
			return err
		}

		logger.Println("getting next movie...")
		movie, err := app.DB.Get(id)
		if err != nil {
			logger.Println(err)
			return err
		}
		if movie == nil {
			logger.Println("no movies to import")
			return nil
		}
		logger.Println("got", movie.Title)

		logger.Println("copying movie...")
		if err := app.CopyFile(movie.ImportedFromPath, movie.LibraryPath); err != nil {
			logger.Println(err)
			return err
		}

		logger.Println("marking as imported...")
		if err := app.DB.MarkMovieAsImported(movie.ID); err != nil {
			logger.Println(err)
			return err
		}
		logger.Printf("imported '%s' from '%s' to '%s'", movie.Title, movie.ImportedFromPath, movie.LibraryPath)
	}
}
