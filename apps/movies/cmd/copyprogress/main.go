package main

import (
	"fmt"
	"log"
	"strings"

	"monks.co/apps/movies/config"
	"monks.co/apps/movies/db"
)

func main() {
	log.Println("Checking copy progress...")

	// Initialize database
	database := db.New(config.DBPath)
	if err := database.Start(); err != nil {
		log.Fatalf("Failed to start database: %v", err)
	}
	defer database.Stop()

	// Get movie statistics
	totalMovies, notCopiedMovies, err := getMovieStats(database)
	if err != nil {
		log.Fatalf("Failed to get movie statistics: %v", err)
	}

	// Get TV episode statistics
	totalEpisodes, notCopiedEpisodes, totalShows, notCopiedShows, err := getTVStats(database)
	if err != nil {
		log.Fatalf("Failed to get TV statistics: %v", err)
	}

	// Print report
	printReport(totalMovies, notCopiedMovies, totalEpisodes, notCopiedEpisodes, totalShows, notCopiedShows)
}

func getMovieStats(database *db.DB) (int64, int64, error) {
	var total int64
	if err := database.Table("movies").Count(&total).Error; err != nil {
		return 0, 0, err
	}

	var notCopied int64
	if err := database.Table("movies").Where("is_copied = false").Count(&notCopied).Error; err != nil {
		return 0, 0, err
	}

	return total, notCopied, nil
}

func getTVStats(database *db.DB) (totalEpisodes, notCopiedEpisodes, totalShows, notCopiedShows int64, err error) {
	// Count total episodes
	if err = database.Table("tv_episodes").Count(&totalEpisodes).Error; err != nil {
		return
	}

	// Count not-copied episodes
	if err = database.Table("tv_episodes").Where("is_copied = false").Count(&notCopiedEpisodes).Error; err != nil {
		return
	}

	// Count total shows
	if err = database.Table("tv_shows").Count(&totalShows).Error; err != nil {
		return
	}

	// Count shows with at least one uncoped episode
	if err = database.Table("tv_shows").
		Where("id IN (SELECT DISTINCT show_id FROM tv_episodes WHERE is_copied = false)").
		Count(&notCopiedShows).Error; err != nil {
		return
	}

	return
}

func printReport(totalMovies, notCopiedMovies, totalEpisodes, notCopiedEpisodes, totalShows, notCopiedShows int64) {
	copiedMovies := totalMovies - notCopiedMovies
	copiedEpisodes := totalEpisodes - notCopiedEpisodes
	copiedShows := totalShows - notCopiedShows

	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("COPY PROGRESS REPORT")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()

	// Movies section
	fmt.Println("MOVIES:")
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("  Total movies:       %6d\n", totalMovies)
	fmt.Printf("  Copied:             %6d", copiedMovies)
	if totalMovies > 0 {
		moviePercent := float64(copiedMovies) / float64(totalMovies) * 100
		fmt.Printf("  (%5.1f%%)\n", moviePercent)
	} else {
		fmt.Println()
	}
	fmt.Printf("  Remaining:          %6d", notCopiedMovies)
	if totalMovies > 0 {
		movieRemainingPercent := float64(notCopiedMovies) / float64(totalMovies) * 100
		fmt.Printf("  (%5.1f%%)\n", movieRemainingPercent)
	} else {
		fmt.Println()
	}
	fmt.Println()

	// TV Shows section
	fmt.Println("TV SHOWS:")
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("  Total shows:        %6d\n", totalShows)
	fmt.Printf("  Fully copied:       %6d", copiedShows)
	if totalShows > 0 {
		showPercent := float64(copiedShows) / float64(totalShows) * 100
		fmt.Printf("  (%5.1f%%)\n", showPercent)
	} else {
		fmt.Println()
	}
	fmt.Printf("  Partially copied:   %6d", notCopiedShows)
	if totalShows > 0 {
		showRemainingPercent := float64(notCopiedShows) / float64(totalShows) * 100
		fmt.Printf("  (%5.1f%%)\n", showRemainingPercent)
	} else {
		fmt.Println()
	}
	fmt.Println()

	// TV Episodes section
	fmt.Println("TV EPISODES:")
	fmt.Println(strings.Repeat("-", 70))
	fmt.Printf("  Total episodes:     %6d\n", totalEpisodes)
	fmt.Printf("  Copied:             %6d", copiedEpisodes)
	if totalEpisodes > 0 {
		episodePercent := float64(copiedEpisodes) / float64(totalEpisodes) * 100
		fmt.Printf("  (%5.1f%%)\n", episodePercent)
	} else {
		fmt.Println()
	}
	fmt.Printf("  Remaining:          %6d", notCopiedEpisodes)
	if totalEpisodes > 0 {
		episodeRemainingPercent := float64(notCopiedEpisodes) / float64(totalEpisodes) * 100
		fmt.Printf("  (%5.1f%%)\n", episodeRemainingPercent)
	} else {
		fmt.Println()
	}
	fmt.Println()

	// Overall summary
	fmt.Println("OVERALL:")
	fmt.Println(strings.Repeat("-", 70))
	totalItems := totalMovies + totalEpisodes
	copiedItems := copiedMovies + copiedEpisodes
	remainingItems := notCopiedMovies + notCopiedEpisodes
	fmt.Printf("  Total items:        %6d  (movies + episodes)\n", totalItems)
	fmt.Printf("  Copied:             %6d", copiedItems)
	if totalItems > 0 {
		overallPercent := float64(copiedItems) / float64(totalItems) * 100
		fmt.Printf("  (%5.1f%%)\n", overallPercent)
	} else {
		fmt.Println()
	}
	fmt.Printf("  Remaining:          %6d", remainingItems)
	if totalItems > 0 {
		overallRemainingPercent := float64(remainingItems) / float64(totalItems) * 100
		fmt.Printf("  (%5.1f%%)\n", overallRemainingPercent)
	} else {
		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
}
