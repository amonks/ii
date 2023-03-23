package moviecopier

import (
	"context"
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
		db:     db,
	}
}

func (app *MovieCopier) Run(ctx context.Context) error {
	defer app.System.Start()()

	fmt.Println("moviecopier: start")

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		app.Printf("getting next movie ID...")
		id, err := app.db.GetMovieIDToImport()
		if err != nil {
			app.Println(err)
			return err
		}
		if id == 0 {
			app.Printf("no movies to copy.")
			return nil
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
		if err := copyFile(ctx, movie.ImportedFromPath, movie.LibraryPath); err != nil {
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

func copyFile(ctx context.Context, src, dest string) error {
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

	srcFReader := NewCancelReader(ctx, srcF)

	destF, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("error creating file '%s': %w", dest, err)
	}
	defer destF.Close()

	if _, err := io.Copy(destF, srcFReader); err != nil {
		return fmt.Errorf("error copying file from '%s' to '%s': %w", src, dest, err)
	}

	return nil
}

type CancelReader struct {
	base io.Reader
	ctx  context.Context
}

func NewCancelReader(ctx context.Context, base io.Reader) *CancelReader {
	return &CancelReader{
		base: base,
		ctx:  ctx,
	}
}

func (cr *CancelReader) Read(b []byte) (int, error) {
	select {
	case <-cr.ctx.Done():
		return 0, cr.ctx.Err()
	default:
		return cr.base.Read(b)
	}
}

