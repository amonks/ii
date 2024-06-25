package moviecopier

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"monks.co/apps/movies/config"
	"monks.co/apps/movies/db"
)

type MovieCopier struct {
	db *db.DB
}

func New(db *db.DB) *MovieCopier {
	return &MovieCopier{
		db: db,
	}
}

func (app *MovieCopier) Run(ctx context.Context) error {
	log.Println("moviecopier started")
	defer log.Println("moviecopier done")

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		log.Printf("getting next movie ID...")
		id, err := app.db.GetMovieIDToCopy()
		if err != nil {
			log.Println(err)
			return err
		}
		if id == 0 {
			log.Printf("no movies to copy.")
			return nil
		}

		log.Println("getting next movie...")
		movie, err := app.db.GetMovie(id)
		if err != nil {
			log.Println(err)
			return err
		}
		if movie == nil {
			log.Println("no movies to import")
			return nil
		}
		log.Println("got", movie.Title)

		log.Println("copying movie...")
		if err := app.copyFile(ctx, config.MovieImportDir+"/"+movie.ImportedFromPath, config.MovieLibraryDir+"/"+movie.LibraryPath); err != nil {
			log.Println(err)
			return err
		}

		if movie.ImportedAt == "" {
			if err := app.db.SetMovieImportedAt(movie, time.Now()); err != nil {
				log.Println(err)
				return err
			}
		}
		log.Printf("imported '%s' from '%s' to '%s'", movie.Title, movie.ImportedFromPath, movie.LibraryPath)
	}
}

func (app *MovieCopier) copyFile(ctx context.Context, src, dest string) error {
	srcStat, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("copy: error reading source file '%s': %w", src, err)
	}
	if !srcStat.Mode().IsRegular() {
		return fmt.Errorf("cannot copy irregular file '%s'", src)
	}

	srcF, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening file '%s': %w", src, err)
	}
	defer srcF.Close()

	var rdr io.Reader = NewCancelReader(ctx, srcF)
	rdr = io.TeeReader(rdr, &ProgressWriter{totalSize: int(srcStat.Size())})

	destF, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("error creating file '%s': %w", dest, err)
	}
	defer destF.Close()

	buf := make([]byte, 8)
	if _, err := io.CopyBuffer(destF, rdr, buf); err != nil {
		return fmt.Errorf("error copying file from '%s' to '%s': %w", src, dest, err)
	}

	return nil
}

type ProgressWriter struct {
	nextPrint time.Time
	progress  int
	totalSize int
}

func (pw *ProgressWriter) Write(data []byte) (int, error) {
	pw.progress += len(data)
	if time.Now().After(pw.nextPrint) {
		log.Printf("progress: %.2f%%\t%s / %s",
			100*float64(pw.progress)/float64(pw.totalSize),
			byteCount(pw.progress),
			byteCount(pw.totalSize))
		pw.nextPrint = time.Now().Add(1 * time.Second)
	}
	return len(data), nil
}

func byteCount(b int) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB",
		float64(b)/float64(div), "kMGTPE"[exp])
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
