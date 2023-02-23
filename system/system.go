package system

import (
	"fmt"
	"io"
	"os"

	"monks.co/movietagger/db"
	"monks.co/movietagger/tmdb"
)

type System struct {
	DB      *db.DB
	TMDB    *tmdb.Client
}

func (s *System) Start() error {
	return s.DB.Start()
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
