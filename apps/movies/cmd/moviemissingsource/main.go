package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"monks.co/apps/movies/config"
	"monks.co/apps/movies/db"
)

func main() {
	log.Println("Checking for movies that need copying but have missing source files...")

	// Initialize database
	database := db.New(config.DBPath)
	if err := database.Start(); err != nil {
		log.Fatalf("Failed to start database: %v", err)
	}
	defer database.Stop()

	// Get all movies that are NOT copied yet
	movies, err := getNotCopiedMovies(database)
	if err != nil {
		log.Fatalf("Failed to get not-copied movies: %v", err)
	}

	log.Printf("Found %d movies marked as not copied\n", len(movies))

	missingSourceMovies := []struct {
		Movie      *db.Movie
		SourcePath string
	}{}

	// Check each movie's source file
	for i, movie := range movies {
		if (i+1)%100 == 0 || i+1 == len(movies) {
			log.Printf("[%d/%d] Checking movies...", i+1, len(movies))
		}

		sourcePath := filepath.Join(config.MovieImportDir, movie.ImportedFromPath)

		// Check if source file exists
		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			missingSourceMovies = append(missingSourceMovies, struct {
				Movie      *db.Movie
				SourcePath string
			}{
				Movie:      movie,
				SourcePath: sourcePath,
			})
		}
	}

	// Print report
	printReport(len(movies), missingSourceMovies)
}

func getNotCopiedMovies(database *db.DB) ([]*db.Movie, error) {
	var movies []*db.Movie
	if err := database.Where("is_copied = false").Find(&movies).Error; err != nil {
		return nil, err
	}
	return movies, nil
}

func printReport(totalMovies int, missingSourceMovies []struct {
	Movie      *db.Movie
	SourcePath string
}) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("MOVIES TO-BE-COPIED WITH MISSING SOURCE FILES")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Total movies to be copied: %d\n", totalMovies)
	fmt.Printf("Movies with missing source: %d\n", len(missingSourceMovies))
	fmt.Println(strings.Repeat("=", 60))

	if len(missingSourceMovies) > 0 {
		fmt.Println("\nMOVIES WITH MISSING SOURCE FILES:")
		for _, item := range missingSourceMovies {
			fmt.Printf("%s\n", item.SourcePath)
		}
	} else {
		fmt.Println("\nAll movies to-be-copied have valid source files!")
	}
}
