package system

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"monks.co/movietagger/db"
	"monks.co/movietagger/fzf"
	"monks.co/movietagger/tmdb"
)

type System struct {
	DB   *db.DB
	TMDB *tmdb.Client
}


func (a *System) BuildSearchQuery(path string) (string, int64, error) {
	fmt.Println("locating " + path)

	var yearQ string
	var titleQ string

	fmt.Printf("enter year: ")
	fmt.Scanln(&yearQ) // skip error to allow empty input

	fmt.Printf("enter query: ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		titleQ = scanner.Text()
	}

	var year int64
	if yearQ != "" {
		i, err := strconv.Atoi(yearQ)
		if err == nil {
			year = int64(i)
		}
	}

	return titleQ, year, nil
}

var ErrRetry = errors.New("retry")
var ErrSkip = errors.New("skip")

func (a *System) Search(titleQ string, yearQ int64) (int64, error) {
	ress, err := a.TMDB.Search(titleQ, yearQ)
	if err != nil {
		return 0, err
	}

	fzfTerms := []string{}
	idsByTerm := make(map[string]int64)
	for _, res := range ress {
		tmdbURL := fmt.Sprintf("https://www.themoviedb.org/movie/%d", res.ID)
		term := fmt.Sprintf("%s: %s %s", res.ReleaseDate, res.Title, tmdbURL)
		fzfTerms = append(fzfTerms, term)
		idsByTerm[term] = res.ID
	}
	fzfTerms = append(fzfTerms, "retry", "skip", "manual entry")

	term, err := fzf.Select(fzfTerms)
	if err != nil {
		return 0, err
	}

	if term == "manual entry" {
		var idQ string
		fmt.Printf("enter ID: ")
		if _, err := fmt.Scanln(&idQ); err != nil {
			return 0, err
		}
		id, err := strconv.Atoi(idQ)
		if err != nil {
			return 0, err
		}
		return int64(id), nil
	}

	if term == "retry" {
		return 0, ErrRetry
	}
	if term == "skip" {
		return 0, ErrSkip
	}

	id := idsByTerm[term]

	return id, nil
}

func (_ *System) CopyFile(src, dest string) error {
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

	if _, err := io.Copy(destF, srcF); err != nil {
		return fmt.Errorf("error copying file from '%s' to '%s': %w", src, dest, err)
	}

	return nil
}
