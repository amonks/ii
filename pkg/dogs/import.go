package dogs

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"gorm.io/gorm/clause"
)

const importInterval = time.Hour

type Importer struct {
	db             *DB
	archiveDir     string
	lastImportFile string

	mu    sync.Mutex
	state importerState
}

type importerState struct {
	// "", "importing", "waiting"
	label        string
	importedAt   time.Time
	startedAt    time.Time
	nextImportAt time.Time
}

func (imp *Importer) set(state importerState) {
	imp.mu.Lock()
	defer imp.mu.Unlock()
	imp.state = state
}

func (imp *Importer) get() importerState {
	imp.mu.Lock()
	defer imp.mu.Unlock()
	return imp.state
}

func (imp *Importer) importNow() error {
	last := imp.get().importedAt
	imp.set(importerState{
		label:        "importing",
		importedAt:   last,
		startedAt:    time.Now(),
		nextImportAt: time.Time{},
	})

	err := imp.db.Import(imp.archiveDir)
	if err == nil {
		last = time.Now()
		// Touch marker file to record import time on disk.
		os.WriteFile(imp.lastImportFile, nil, 0666)
	}
	imp.set(importerState{
		label:        "waiting",
		importedAt:   last,
		startedAt:    time.Time{},
		nextImportAt: time.Now().Add(importInterval),
	})
	return err
}

func (imp *Importer) String() string {
	imp.mu.Lock()
	defer imp.mu.Unlock()

	switch imp.state.label {
	case "":
		return "starting"
	case "waiting":
		return fmt.Sprintf("imported at %s, next import in %s",
			imp.state.importedAt.Format(time.Stamp),
			time.Until(imp.state.nextImportAt).Round(time.Second))
	case "importing":
		return fmt.Sprintf("last import at %s, import started %s ago",
			imp.state.importedAt.Format("Jan _2 15:04:05 MST"),
			time.Since(imp.state.startedAt).Round(time.Second))
	default:
		panic("importer error")
	}
}

func NewImporter(db *DB, archiveDir string) *Importer {
	return &Importer{
		db:             db,
		archiveDir:     archiveDir,
		lastImportFile: filepath.Join(archiveDir, "last_import"),
	}
}

func (imp *Importer) Start(ctx context.Context) error {
	// Check marker file to see if a recent import already happened (e.g.
	// across process restarts). If so, skip the initial import and just
	// wait for the next interval.
	if info, err := os.Stat(imp.lastImportFile); err == nil && time.Since(info.ModTime()) < importInterval {
		log.Printf("skipping initial import: last import was %s ago", time.Since(info.ModTime()).Round(time.Second))
		imp.set(importerState{
			label:        "waiting",
			importedAt:   info.ModTime(),
			nextImportAt: info.ModTime().Add(importInterval),
		})
	} else {
		log.Printf("running initial import")
		if err := imp.importNow(); err != nil {
			return err
		}
	}

	for {
		next := imp.get().nextImportAt

		select {
		case <-ctx.Done():
			return context.Canceled

		case <-time.After(time.Until(next)):
		}

		if err := imp.importNow(); err != nil {
			return err
		}
	}
}

const downloadURL = "https://docs.google.com/spreadsheets/d/1qWoxtqSUGb4qnO_ZVyxV_yJUG01kpIBbEJByOXgjEVI/export?format=zip&id=1qWoxtqSUGb4qnO_ZVyxV_yJUG01kpIBbEJByOXgjEVI"

func (db *DB) Import(archiveDir string) error {
	newImages := map[string]struct{}{}

	filename, err := downloadFile("archive", downloadURL)
	if err != nil {
		return err
	}

	z, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}
	defer z.Close()

	var htmlFile *zip.File
	for _, f := range z.File {
		if f.Name == `🗄️ (DO NOT EDIT) Archive.html` {
			htmlFile = f
			break
		}
	}
	if htmlFile == nil {
		return fmt.Errorf("archive sheet not found in zip")
	}
	r, err := htmlFile.Open()
	if err != nil {
		return fmt.Errorf("error opening html file: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return fmt.Errorf("error parsing html: %w", err)
	}

	done := errors.New("done")
	var entries []Entry
	var rowsLoopErr error
	doc.Find("tr").EachWithBreak(func(r int, row *goquery.Selection) bool {
		// skip header
		if r < 2 {
			return true
		}

		var entry Entry
		entry.Number = r - 1
		var colsLoopErr error
		row.Find("td").EachWithBreak(func(c int, col *goquery.Selection) bool {
			switch c {
			case 0:
				date := col.Text()
				if date == "" {
					colsLoopErr = done
					return false
				}
				entry.Date = date
				return true
			case 1:
				count, err := strconv.ParseFloat(col.Text(), 64)
				if err != nil {
					colsLoopErr = err
					return false
				}
				entry.Count = count
				return true
			case 2:
				entry.Eater = col.Text()
				return true
			case 3:
				url, exists := col.Find("img").Attr("src")
				if exists {
					url = sizeRE.ReplaceAllString(url, "")
					filenameRoot := fmt.Sprintf("images/%d-%s", entry.Number, entry.Eater)
					var filename string
					if strings.HasPrefix(url, "http") {
						f, err := downloadFile(filepath.Join(archiveDir, filenameRoot), url)
						if err != nil {
							log.Printf("warning: error downloading image in row %d: %v", r, err)
						} else {
							// downloadFile returns the absolute path; trim to relative.
							filename, _ = filepath.Rel(archiveDir, f)
						}
					} else {
						// Image is embedded in the zip (relative path).
						f, err := extractFromZip(z, url, filepath.Join(archiveDir, filenameRoot))
						if err != nil {
							log.Printf("warning: error extracting image in row %d: %v", r, err)
						} else {
							filename = f
						}
					}
					entry.PhotoFilename = filename
					entry.PhotoURL = url
					newImages[filename] = struct{}{}
				}
				return true
			case 4:
				notes, err := col.Html()
				if err != nil {
					colsLoopErr = fmt.Errorf("error extracting notes in row %d: %w", r, err)
					return false
				}
				notes = html.UnescapeString(notes)
				notes = strings.ReplaceAll(notes, "<br/>", "\n")
				entry.Notes = notes
				return false
			}
			return false
		})
		if colsLoopErr != nil {
			rowsLoopErr = colsLoopErr
			return false
		}

		notes := strings.ReplaceAll(entry.Notes, "-", " ")
		notes = strings.ReplaceAll(notes, "—", " ")
		words := strings.Fields(notes)
		entry.Wordcount = len(words)
		entries = append(entries, entry)
		return true
	})
	if rowsLoopErr != nil && rowsLoopErr != done {
		return rowsLoopErr
	}

	log.Printf("%d entries", len(entries))

	for _, entry := range entries {
		if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(entry).Error; err != nil {
			return err
		}
	}

	dirEntries, err := os.ReadDir(filepath.Join(archiveDir, "images"))
	if err != nil {
		return err
	}
	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		fn := "images/" + de.Name()
		if _, inUse := newImages[fn]; !inUse {
			log.Printf("delete %s", fn)
			if err := os.Remove(filepath.Join(archiveDir, fn)); err != nil {
				return fmt.Errorf("removing stale image: %w", err)
			}
		}
	}

	return nil
}

var sizeRE = regexp.MustCompile(`=w\d+-h\d+$`)


func extractFromZip(z *zip.ReadCloser, zipPath string, destRoot string) (string, error) {
	var zf *zip.File
	for _, f := range z.File {
		if f.Name == zipPath {
			zf = f
			break
		}
	}
	if zf == nil {
		return "", fmt.Errorf("file %s not found in zip", zipPath)
	}

	ext := filepath.Ext(zf.Name)
	destFilename := destRoot + ext

	rc, err := zf.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	out, err := os.Create(destFilename)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return "", err
	}

	// Return just the relative part (images/N-Name.ext).
	return filepath.Base(filepath.Dir(destRoot)) + "/" + filepath.Base(destFilename), nil
}

func downloadFile(filenameRoot, url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-type")
	var extension string
	switch contentType {
	case "image/jpeg":
		extension = "jpg"
	case "image/png":
		extension = "png"
	case "application/zip":
		extension = "zip"
	default:
		return "", fmt.Errorf("unknown content type: '%s'", contentType)
	}
	filename := fmt.Sprintf("%s.%s", filenameRoot, extension)
	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}

	return filename, nil
}
