package moviecopier

import (
	"fmt"
	"io"
	"os"

	"monks.co/movietagger/db"
	"monks.co/movietagger/system"
)

type MovieCopier struct {
	*system.System
	db *db.DB
}

func New(db *db.DB) *MovieCopier {
	system := system.New("copier")
	return &MovieCopier{
		System: system,
		db: db,
	}
}

func (app *MovieCopier) Run() error {
	defer app.System.Start()()

	for {
		app.Printf("getting next movie ID...")
		id, err := app.db.GetMovieIDToImport()
		if err != nil {
			app.Println(err)
			return err
		}

		app.Println("getting next movie...")
		movie, err := app.db.GetMovie(id)
		if err != nil {
			app.Println(err)
			return err
		}
		if movie == nil {
			app.Println("no movies to import")
			return nil
		}
		app.Println("got", movie.Title)

		app.Println("copying movie...")
		if err := copyFile(movie.ImportedFromPath, movie.LibraryPath); err != nil {
			app.Println(err)
			return err
		}

		app.Println("marking as imported...")
		if err := app.db.MarkMovieAsImported(movie.ID); err != nil {
			app.Println(err)
			return err
		}
		app.Printf("imported '%s' from '%s' to '%s'", movie.Title, movie.ImportedFromPath, movie.LibraryPath)
	}
}

func copyFile(src, dest string) error {
	srcStat, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcStat.Mode().IsRegular() {
		return fmt.Errorf("cannot copy irregular file '%s'", src)
	}

	srcF, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening file '%s': %w", src, err)
	}
	defer srcF.Close()

	destF, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("error creating file '%s': %w", dest, err)
	}
	defer destF.Close()

	if _, err := io.Copy(destF, srcF); err != nil {
		return fmt.Errorf("error copying file from '%s' to '%s': %w", src, dest, err)
	}

	return nil
}
