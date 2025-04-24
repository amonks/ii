package tvcopier

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"monks.co/apps/movies/config"
	"monks.co/apps/movies/db"
	"monks.co/pkg/filesystem"
)

type TVCopier struct {
	mu sync.Mutex
	db *db.DB
	fs filesystem.FS
}

func New(db *db.DB) *TVCopier {
	return &TVCopier{
		db: db,
		fs: filesystem.NewOSFileSystem(),
	}
}

// WithFS allows injecting a custom filesystem for testing
func (app *TVCopier) WithFS(fs filesystem.FS) *TVCopier {
	app.fs = fs
	return app
}

func (app *TVCopier) Run(ctx context.Context) error {
	app.mu.Lock()
	defer app.mu.Unlock()

	log.Println("tvcopier started")
	defer log.Println("tvcopier done")

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		log.Printf("Getting next TV episode to copy...")
		showID, seasonNum, episodeNum, err := app.db.GetTVEpisodeIDToCopy()
		if err != nil {
			log.Println(err)
			return err
		}
		if showID == 0 {
			log.Printf("No TV episodes to copy.")
			return nil
		}

		log.Println("Getting episode details...")
		episode, err := app.db.GetTVEpisode(showID, seasonNum, episodeNum)
		if err != nil {
			return err
		}
		if episode == nil {
			log.Println("No TV episodes to import")
			return nil
		}

		// Get show and season information to create directory structure
		show, err := app.db.GetTVShow(showID)
		if err != nil {
			return err
		}

		season, err := app.db.GetTVSeason(showID, seasonNum)
		if err != nil {
			return err
		}

		// Create show directory if it doesn't exist
		showDir := filepath.Join(config.TVLibraryDir, show.LibraryPath)
		if err := os.MkdirAll(showDir, 0755); err != nil {
			return fmt.Errorf("error creating show directory: %w", err)
		}

		// Create season directory if it doesn't exist
		seasonDir := filepath.Join(config.TVLibraryDir, season.LibraryPath)
		if err := os.MkdirAll(seasonDir, 0755); err != nil {
			return fmt.Errorf("error creating season directory: %w", err)
		}

		// Now copy the episode file
		log.Println("Copying TV episode...")
		srcPath := episode.ImportedFromPath
		if srcPath != "" && !filepath.IsAbs(srcPath) {
			srcPath = filepath.Join(config.TVImportDir, srcPath)
		}
		destPath := filepath.Join(config.TVLibraryDir, episode.LibraryPath)

		if err := app.copyFile(ctx, srcPath, destPath); err != nil {
			return err
		}

		if err := app.db.SetTVEpisodeCopied(episode); err != nil {
			return err
		}
		log.Printf("Imported '%s S%02dE%02d - %s' from '%s' to '%s'", 
			show.Name, episode.SeasonNumber, episode.EpisodeNumber, episode.Name, 
			episode.ImportedFromPath, episode.LibraryPath)
	}
}

func (app *TVCopier) copyFile(ctx context.Context, src, dest string) error {
	srcStat, err := app.fs.Stat(src)
	if err != nil {
		return fmt.Errorf("copy: error reading source file '%s': %w", src, err)
	}
	if !srcStat.Mode().IsRegular() {
		return fmt.Errorf("cannot copy irregular file '%s'", src)
	}

	srcF, err := app.fs.Open(src)
	if err != nil {
		return fmt.Errorf("error opening file '%s': %w", src, err)
	}
	defer srcF.Close()

	destDir := filepath.Dir(dest)
	if err := app.fs.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("error creating destination directory '%s': %w", destDir, err)
	}

	destF, err := app.fs.Create(dest)
	if err != nil {
		return fmt.Errorf("error creating file '%s': %w", dest, err)
	}
	defer destF.Close()

	// Copy the file with context awareness
	done := make(chan error, 1)
	go func() {
		_, err := app.fs.Copy(srcF, destF)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("error copying file: %w", err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}