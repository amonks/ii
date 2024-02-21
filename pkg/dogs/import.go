package dogs

import (
	"archive/zip"
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

	"github.com/PuerkitoBio/goquery"
	"gorm.io/gorm/clause"
)

const downloadURL = "https://docs.google.com/spreadsheets/d/1qWoxtqSUGb4qnO_ZVyxV_yJUG01kpIBbEJByOXgjEVI/export?format=zip&id=1qWoxtqSUGb4qnO_ZVyxV_yJUG01kpIBbEJByOXgjEVI"

func (db *DB) Import(archiveDir string) error {
	imagesOnDisk, err := collectImages(archiveDir)
	if err != nil {
		return err
	}

	filename, err := downloadFile("archive", downloadURL)
	if err != nil {
		return err
	}

	var htmlFile *zip.File
	z, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}
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
		log.Printf("entry %d", entry.Number)
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
					if imageOnDisk, has := imagesOnDisk[filenameRoot]; has {
						filename = imageOnDisk.filename
						imageOnDisk.isUsed = true
					} else {
						f, err := downloadFile(filepath.Join(archiveDir, filenameRoot), url)
						if err != nil {
							colsLoopErr = fmt.Errorf("error downloading image in row %d: %w", r, err)
							return false
						}
						filename = f
					}
					entry.PhotoFilename = filename
					entry.PhotoURL = url
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

	for _, imageOnDisk := range imagesOnDisk {
		if !imageOnDisk.isUsed {
			log.Printf("delete %s", imageOnDisk.filename)
			if err := os.Remove(filepath.Join(archiveDir, imageOnDisk.filename)); err != nil {
				return fmt.Errorf("removing stale image: %w", err)
			}
		}
	}

	return nil
}

var sizeRE = regexp.MustCompile(`=w\d+-h\d+$`)

// key includes "images/" but not file extension
type imagesOnDisk map[string]*imageOnDisk
type imageOnDisk struct {
	// includes both "images/" and file extension
	filename string
	isUsed   bool
}

func collectImages(archiveDir string) (imagesOnDisk, error) {
	dirEntries, err := os.ReadDir(filepath.Join(archiveDir, "images"))
	if err != nil {
		return nil, err
	}
	images := imagesOnDisk{}
	for _, entry := range dirEntries {
		if entry.IsDir() {
			continue
		}
		parts := strings.Split(entry.Name(), ".")
		images["images/"+parts[0]] = &imageOnDisk{
			filename: "images/" + entry.Name(),
		}
	}
	return images, nil
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
