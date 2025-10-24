package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"monks.co/apps/movies/config"
	"monks.co/apps/movies/db"
)

type VerificationResult struct {
	Movie              *db.Movie
	DestinationExists  bool
	SourceExists       bool
	SizesMatch         bool
	Action             string
	Error              error
}

type VerificationReport struct {
	TotalMovies            int
	DestinationMissing     int
	SourceMissing          int
	SizeMismatches         int
	PathMismatches         int
	Verified               int
	Errors                 int
	Results                []VerificationResult
}

func main() {
	dryRun := flag.Bool("dryrun", true, "Run in dry-run mode (no changes made)")
	flag.Parse()

	if *dryRun {
		log.Println("Starting movie verification (DRY RUN MODE - no changes will be made)...")
	} else {
		log.Println("Starting movie verification...")
	}

	// Initialize database
	database := db.New(config.DBPath)
	if err := database.Start(); err != nil {
		log.Fatalf("Failed to start database: %v", err)
	}
	defer database.Stop()

	// Get all movies that are marked as copied
	movies, err := getCopiedMovies(database)
	if err != nil {
		log.Fatalf("Failed to get copied movies: %v", err)
	}

	log.Printf("Found %d movies marked as copied", len(movies))

	var report VerificationReport
	report.TotalMovies = len(movies)

	// Verify each movie
	for i, movie := range movies {
		if (i+1)%100 == 0 || i+1 == len(movies) {
			log.Printf("[%d/%d] Processing movies...", i+1, len(movies))
		}
		
		result := verifyMovie(database, movie, *dryRun)
		report.Results = append(report.Results, result)
		
		switch result.Action {
		case "marked_as_not_copied_missing_dest", "would_mark_as_not_copied_missing_dest":
			report.DestinationMissing++
		case "deleted_missing_both", "would_delete_missing_both":
			report.DestinationMissing++
		case "source_missing":
			report.SourceMissing++
		case "marked_as_not_copied_size_mismatch", "would_mark_as_not_copied_size_mismatch":
			report.SizeMismatches++
		case "fixed_path_mismatch", "would_fix_path_mismatch":
			report.PathMismatches++
		case "verified":
			report.Verified++
		case "error":
			report.Errors++
		}

	}

	// Print final report
	printReport(report, *dryRun)
}

func getCopiedMovies(database *db.DB) ([]*db.Movie, error) {
	var movies []*db.Movie
	if err := database.Where("is_copied = true").Find(&movies).Error; err != nil {
		return nil, err
	}
	return movies, nil
}

func verifyMovie(database *db.DB, movie *db.Movie, dryRun bool) VerificationResult {
	result := VerificationResult{
		Movie: movie,
	}

	sourcePath := filepath.Join(config.MovieImportDir, movie.ImportedFromPath)
	destPath := filepath.Join(config.MovieLibraryDir, movie.LibraryPath)

	// Check if the library path matches what we would generate now
	expectedLibraryPath := movie.BuildLibraryPath()
	if movie.LibraryPath != expectedLibraryPath {
		// Path doesn't match - need to fix it
		if dryRun {
			result.Action = "would_fix_path_mismatch"
		} else {
			result.Action = "fixed_path_mismatch"

			// Delete the old destination file if it exists
			if _, err := os.Stat(destPath); err == nil {
				if err := os.Remove(destPath); err != nil {
					result.Error = fmt.Errorf("failed to delete old destination file: %w", err)
					result.Action = "error"
					return result
				}
			}

			// Update the library path in the database
			if err := database.UpdateMovieLibraryPath(movie, expectedLibraryPath); err != nil {
				result.Error = fmt.Errorf("failed to update library path: %w", err)
				result.Action = "error"
				return result
			}

			// Mark as not copied so it will be re-copied with the correct path
			if err := setMovieIsNotCopied(database, movie); err != nil {
				result.Error = fmt.Errorf("failed to mark movie as not copied: %w", err)
				result.Action = "error"
				return result
			}
		}
		return result
	}

	// Check if destination file exists
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		result.DestinationExists = false
		
		// Check if source file exists to determine the appropriate action
		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			result.SourceExists = false
			if dryRun {
				result.Action = "would_delete_missing_both"
			} else {
				result.Action = "deleted_missing_both"
				// Delete the movie from the database entirely
				if err := database.DeleteMovie(movie); err != nil {
					result.Error = fmt.Errorf("failed to delete movie from database: %w", err)
					result.Action = "error"
				}
			}
			return result
		} else if err != nil {
			result.Error = fmt.Errorf("error checking source file: %w", err)
			result.Action = "error"
			return result
		}
		
		result.SourceExists = true
		if dryRun {
			result.Action = "would_mark_as_not_copied_missing_dest"
		} else {
			result.Action = "marked_as_not_copied_missing_dest"
			// Mark as not copied so moviecopier can handle it
			if err := setMovieIsNotCopied(database, movie); err != nil {
				result.Error = fmt.Errorf("failed to mark movie as not copied: %w", err)
				result.Action = "error"
			}
		}
		return result
	} else if err != nil {
		result.Error = fmt.Errorf("error checking destination file: %w", err)
		result.Action = "error"
		return result
	}
	result.DestinationExists = true

	// Check if source file exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		result.SourceExists = false
		result.Action = "source_missing"
		// Nothing we can do here, just log and continue
		return result
	} else if err != nil {
		result.Error = fmt.Errorf("error checking source file: %w", err)
		result.Action = "error"
		return result
	}
	result.SourceExists = true

	// Compare file sizes
	matches, err := compareFileSizes(sourcePath, destPath)
	if err != nil {
		result.Error = fmt.Errorf("error comparing file sizes: %w", err)
		result.Action = "error"
		return result
	}
	result.SizesMatch = matches

	if !matches {
		if dryRun {
			result.Action = "would_mark_as_not_copied_size_mismatch"
		} else {
			result.Action = "marked_as_not_copied_size_mismatch"
			
			// Delete the destination file
			if err := os.Remove(destPath); err != nil {
				result.Error = fmt.Errorf("failed to delete destination file: %w", err)
				result.Action = "error"
				return result
			}
			
			// Mark as not copied so moviecopier can handle it
			if err := setMovieIsNotCopied(database, movie); err != nil {
				result.Error = fmt.Errorf("failed to mark movie as not copied: %w", err)
				result.Action = "error"
				return result
			}
		}
		return result
	}

	result.Action = "verified"
	return result
}

func setMovieIsNotCopied(database *db.DB, movie *db.Movie) error {
	return database.Table("movies").
		Where("id = ?", movie.ID).
		Updates(map[string]interface{}{"is_copied": false}).
		Error
}

func compareFileSizes(file1, file2 string) (bool, error) {
	stat1, err := os.Stat(file1)
	if err != nil {
		return false, fmt.Errorf("failed to stat %s: %w", file1, err)
	}

	stat2, err := os.Stat(file2)
	if err != nil {
		return false, fmt.Errorf("failed to stat %s: %w", file2, err)
	}

	return stat1.Size() == stat2.Size(), nil
}

func printReport(report VerificationReport, dryRun bool) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	if dryRun {
		fmt.Println("MOVIE VERIFICATION REPORT (DRY RUN)")
	} else {
		fmt.Println("MOVIE VERIFICATION REPORT")
	}
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Total movies checked: %d\n", report.TotalMovies)
	fmt.Printf("Verified (no issues): %d\n", report.Verified)
	
	if dryRun {
		fmt.Printf("Destination missing (would mark as not copied): %d\n", report.DestinationMissing)
		fmt.Printf("Source missing (logged only): %d\n", report.SourceMissing)
		fmt.Printf("Size mismatches (would fix): %d\n", report.SizeMismatches)
		fmt.Printf("Path mismatches (would fix): %d\n", report.PathMismatches)
	} else {
		fmt.Printf("Destination missing (marked as not copied): %d\n", report.DestinationMissing)
		fmt.Printf("Source missing (logged only): %d\n", report.SourceMissing)
		fmt.Printf("Size mismatches (fixed): %d\n", report.SizeMismatches)
		fmt.Printf("Path mismatches (fixed): %d\n", report.PathMismatches)
	}
	
	fmt.Printf("Errors: %d\n", report.Errors)
	fmt.Println(strings.Repeat("=", 60))


	// Count movies with source exists vs missing both
	sourceExistsCount := 0
	bothMissingCount := 0
	for _, result := range report.Results {
		if result.Action == "marked_as_not_copied_missing_dest" || result.Action == "would_mark_as_not_copied_missing_dest" {
			sourceExistsCount++
		} else if result.Action == "deleted_missing_both" || result.Action == "would_delete_missing_both" {
			bothMissingCount++
		}
	}

	if sourceExistsCount > 0 {
		fmt.Println("\nMOVIES MISSING DESTINATION (source exists, can be re-copied):")
		for _, result := range report.Results {
			if result.Action == "marked_as_not_copied_missing_dest" || result.Action == "would_mark_as_not_copied_missing_dest" {
				fmt.Printf("- %s\n", result.Movie.Title)
			}
		}
	}

	if bothMissingCount > 0 {
		if dryRun {
			fmt.Println("\nMOVIES MISSING BOTH SOURCE AND DESTINATION (would delete from DB):")
		} else {
			fmt.Println("\nMOVIES MISSING BOTH SOURCE AND DESTINATION (deleted from DB):")
		}
		for _, result := range report.Results {
			if result.Action == "deleted_missing_both" || result.Action == "would_delete_missing_both" {
				fmt.Printf("- %s\n", result.Movie.Title)
				fmt.Printf("  %s/%s\n", config.MovieImportDir, result.Movie.ImportedFromPath)
			}
		}
	}

	if report.SizeMismatches > 0 {
		fmt.Println("\nMOVIES WITH SIZE MISMATCHES:")
		for _, result := range report.Results {
			if result.Action == "marked_as_not_copied_size_mismatch" || result.Action == "would_mark_as_not_copied_size_mismatch" {
				fmt.Printf("- %s (will be re-copied)\n", result.Movie.Title)
			}
		}
	}

	if report.PathMismatches > 0 {
		if dryRun {
			fmt.Println("\nMOVIES WITH PATH MISMATCHES (would fix):")
		} else {
			fmt.Println("\nMOVIES WITH PATH MISMATCHES (fixed):")
		}
		for _, result := range report.Results {
			if result.Action == "fixed_path_mismatch" || result.Action == "would_fix_path_mismatch" {
				expectedPath := result.Movie.BuildLibraryPath()
				fmt.Printf("- %s\n", result.Movie.Title)
				fmt.Printf("  Old: %s\n", result.Movie.LibraryPath)
				fmt.Printf("  New: %s\n", expectedPath)
			}
		}
	}

	if report.Errors > 0 {
		fmt.Println("\nERRORS:")
		for _, result := range report.Results {
			if result.Action == "error" {
				fmt.Printf("- %s: %v\n", result.Movie.Title, result.Error)
			}
		}
	}
}