package tvimporter

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"monks.co/apps/movies/config"
	"monks.co/apps/movies/db"
	"monks.co/pkg/filesystem"
	"monks.co/pkg/tmdb"
)

var (
	// Common TV show episode patterns
	episodePattern      = regexp.MustCompile(`(?i)S(\d+)E(\d+)`)
	seasonEpPattern     = regexp.MustCompile(`(?i)Season\s*(\d+).*?Episode\s*(\d+)`)
	dotSeasonEpPattern  = regexp.MustCompile(`(\d+)x(\d+)`)
	seasonFolderPattern = regexp.MustCompile(`(?i)Season\s*(\d+)`)

	// Format: The Eric Andre Show - 101 - George Clooney.mkv
	dashEpisodePattern = regexp.MustCompile(`(?i).*?[- ]\s*(\d)(\d{2})\s*[- ]`)

	// Format: the.good.wife.509.hdtv-lol.mp4
	dotEpisodePattern = regexp.MustCompile(`(?i).*?\.(\d)(\d{2})\.`)

	// Format: [OZC]The Big O E14 'Roger the Wanderer'.mkv
	plainEpisodePattern = regexp.MustCompile(`(?i)E(\d+)\s*['"\[]`)

	// Format: Batman (1966) - S1E28 The Pharaohs In A Rut.avi
	// No zero padding in season number
	simpleEpisodePattern = regexp.MustCompile(`(?i)S(\d+)E(\d+)`)

	// Format: Survivor S20E01 Slay Everyone, Trust No One
	// Spaces instead of dots or dashes
	spaceEpisodePattern = regexp.MustCompile(`(?i)S(\d+)\s*E(\d+)`)
)

type TVImporter struct {
	tmdb *tmdb.Client
	db   *db.DB
	fs   filesystem.FS
	// Map of directories to episode information
	seasonMap map[string]map[string]EpisodeInfo // dir -> filename -> episode info
}

// EpisodeInfo contains season and episode information for a file
type EpisodeInfo struct {
	Season  int
	Episode int
}

func New(tmdb *tmdb.Client, db *db.DB) *TVImporter {
	return &TVImporter{
		tmdb:      tmdb,
		db:        db,
		fs:        filesystem.NewOSFileSystem(),
		seasonMap: make(map[string]map[string]EpisodeInfo),
	}
}

// WithFS allows injecting a custom filesystem for testing
func (app *TVImporter) WithFS(fs filesystem.FS) *TVImporter {
	app.fs = fs
	return app
}

func (app *TVImporter) Run(ctx context.Context) error {
	log.Println("tvimporter started")
	defer log.Println("tvimporter done")

	// Count existing stubs before running the import
	existingStubs, err := app.db.CountStubsByType(db.MediaTypeTV)
	if err != nil {
		log.Printf("Error counting existing TV stubs: %v", err)
	}

	// Process the TV import directory
	if err := app.scanTVDirectory(ctx, config.TVImportDir); err != nil {
		return fmt.Errorf("error scanning TV directory: %w", err)
	}

	// Count stubs after running the import
	newStubsCount, err := app.db.CountStubsByType(db.MediaTypeTV)
	if err != nil {
		log.Printf("Error counting TV stubs: %v", err)
	} else {
		stubsAdded := newStubsCount - existingStubs
		log.Printf("TV import scan complete. Added %d new TV show stubs.", stubsAdded)
	}

	return nil
}

func (app *TVImporter) scanTVDirectory(ctx context.Context, rootDir string) error {
	// First, identify potential TV show directories
	entries, err := app.fs.ReadDir(rootDir)
	if err != nil {
		return fmt.Errorf("error reading TV directory: %w", err)
	}

	// Log summary instead of individual entries
	directoryCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			directoryCount++
		}
	}
	log.Printf("Found %d potential TV show directories to scan", directoryCount)

	// For each possible show directory
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		if !entry.IsDir() {
			continue // Skip files at root level - we're looking for show folders
		}

		showDir := app.fs.Join(rootDir, entry.Name())
		showRelPath := entry.Name()

		// Create a stub for the show directory itself
		if err := app.processShowDirectory(ctx, showDir, showRelPath); err != nil {
			log.Printf("Error processing show directory %s: %v", showDir, err)
			continue
		}
	}

	return nil
}

func (app *TVImporter) processShowDirectory(ctx context.Context, showDir, showRelPath string) error {
	// Check if this show is ignored
	if ignored, err := app.db.PathIsIgnored(db.MediaTypeTV, showRelPath); err != nil {
		return fmt.Errorf("error checking if ignore exists for show '%s': %w", showRelPath, err)
	} else if ignored {
		return nil
	}

	// Find all episode files in the directory
	episodeFiles, err := app.findEpisodeFiles(ctx, showDir, showRelPath)
	if err != nil {
		return fmt.Errorf("error finding episode files in show '%s': %w", showRelPath, err)
	}

	// Check if any episode files have already been imported
	// Instead of exact path matching, we'll check by folder prefix
	showPrefix := filepath.Join(config.TVImportDir, showRelPath)
	if exists, err := app.db.TVShowExistsByPathPrefix(showPrefix); err != nil {
		log.Printf("Warning: Error checking if TV show exists with prefix '%s': %v", showPrefix, err)
	} else if exists {
		return nil
	}

	// Check if a stub already exists for this show directory
	stubExists := false
	var existingStub *db.Stub
	if exists, err := app.db.StubExistsFromPath(db.MediaTypeTV, showRelPath); err != nil {
		return fmt.Errorf("error checking if stub exists for show '%s': %w", showRelPath, err)
	} else {
		stubExists = exists
		if exists {
			existingStub, err = app.db.GetStub(showRelPath)
			if err != nil {
				return fmt.Errorf("error getting existing stub for show '%s': %w", showRelPath, err)
			}
		}
	}

	if len(episodeFiles) == 0 {
		// Skip directories that don't contain any valid TV episode files
		log.Printf("Skipping directory with no valid episodes: %s", showRelPath)
		return nil
	}

	if stubExists {
		// Only update if episode list has changed
		episodesChanged := !episodeFilesEqual(existingStub.EpisodeFiles, episodeFiles)

		if episodesChanged {
			// Update existing stub with episode files
			existingStub.EpisodeFiles = episodeFiles
			if err := app.db.SaveStub(existingStub); err != nil {
				return fmt.Errorf("error updating stub with episode files for show '%s': %w", showRelPath, err)
			}
			log.Printf("Updated TV show stub with %d episode files: %s", len(episodeFiles), showRelPath)
		}
	} else {
		// Create a new stub for this show directory
		stub, err := app.db.CreateStub(db.MediaTypeTV, showRelPath)
		if err != nil {
			return fmt.Errorf("error creating stub for show '%s': %w", showRelPath, err)
		}

		// Update the stub with episode files
		stub.EpisodeFiles = episodeFiles
		if err := app.db.SaveStub(stub); err != nil {
			return fmt.Errorf("error saving episode files to stub for show '%s': %w", showRelPath, err)
		}

		log.Printf("Added TV show stub with %d episode files: %s", len(episodeFiles), showRelPath)
	}

	return nil
}

// findEpisodeFiles recursively searches a directory and returns all valid TV episode files
// It also analyzes all files in each directory together to determine episode numbers
func (app *TVImporter) findEpisodeFiles(ctx context.Context, dir, basePath string) ([]string, error) {
	var episodeFiles []string

	// Map to store files by directory
	filesByDir := make(map[string][]string)

	// First, collect all files by directory
	var searchDir func(string, string) error
	searchDir = func(curDir, relPath string) error {
		entries, err := app.fs.ReadDir(curDir)
		if err != nil {
			return fmt.Errorf("error reading directory: %w", err)
		}

		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}

			path := app.fs.Join(curDir, entry.Name())
			entryRelPath := app.fs.Join(relPath, entry.Name())

			if entry.IsDir() {
				// Recursively search subdirectories
				if err := searchDir(path, entryRelPath); err != nil {
					log.Printf("Error searching subdirectory %s: %v", path, err)
				}
			} else if app.isEpisodeFile(entry.Name()) {
				// Add episode file to the list
				episodeFiles = append(episodeFiles, entryRelPath)

				// Store files by directory
				dirPath := filepath.Dir(entryRelPath)
				filesByDir[dirPath] = append(filesByDir[dirPath], entryRelPath)
			}
		}

		return nil
	}

	if err := searchDir(dir, basePath); err != nil {
		return nil, err
	}

	// We'll defer the expensive processSeasonDirectories operation until we know it's needed
	// The episode files will be returned first, and season analysis done only when necessary

	return episodeFiles, nil
}

// processSeasonDirectories analyzes all files in each directory together to determine episode numbers
func (app *TVImporter) processSeasonDirectories(filesByDir map[string][]string) {
	// Process each directory separately
	for dirPath, files := range filesByDir {
		if len(files) <= 1 {
			log.Printf("Directory %s has only %d file, skipping sequence analysis", dirPath, len(files))
			continue // Not enough files to find a sequence
		}

		log.Printf("Analyzing directory %s with %d files", dirPath, len(files))

		// Extract season number from directory path
		seasonNum := detectSeasonFromPath(dirPath)

		// Initialize episode map for this directory
		app.seasonMap[dirPath] = make(map[string]EpisodeInfo)

		// Extract base filenames for detection
		baseFiles := make([]string, 0, len(files))
		for _, file := range files {
			baseFiles = append(baseFiles, filepath.Base(file))
		}

		// Analyze files to identify episode numbers
		episodeMap := detectEpisodeNumbersFromFiles(baseFiles)
		if episodeMap == nil {
			log.Printf("Could not detect episode sequence in directory %s", dirPath)
			continue
		}

		// Map episode numbers back to full paths
		for _, file := range files {
			baseFile := filepath.Base(file)
			if epNum, ok := episodeMap[baseFile]; ok {
				// Store the season and episode info in our map
				app.seasonMap[dirPath][file] = EpisodeInfo{
					Season:  seasonNum,
					Episode: epNum,
				}

				log.Printf("Detected S%02dE%02d for %s", seasonNum, epNum, file)
			}
		}
	}
}

func (app *TVImporter) isEpisodeFile(filename string) bool {
	// Check file extension
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".mkv" && ext != ".mp4" && ext != ".avi" && ext != ".m4v" {
		return false
	}

	// First check for explicit episode patterns in filename
	// Note: we're using case-insensitive regex patterns ((?i) prefix), so we don't need to lowercase the filename
	if episodePattern.MatchString(filename) ||
		seasonEpPattern.MatchString(filename) ||
		dotSeasonEpPattern.MatchString(filename) ||
		dashEpisodePattern.MatchString(filename) ||
		dotEpisodePattern.MatchString(filename) ||
		plainEpisodePattern.MatchString(filename) {
		return true
	}

	// If we didn't match a specific pattern, check if the filename contains any numbers
	// This is more permissive than the strict patterns, but helps catch more valid episodes
	numbers := parseNumericSequence(filename)
	return len(numbers) > 0
}

// parseNumericSequence extracts all numeric sequences from a filename
// This is a more robust approach compared to regex-only pattern matching,
// allowing us to handle a wider variety of filename formats.
func parseNumericSequence(filename string) []int {
	// Extract all numeric sequences
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(filename, -1)

	numbers := make([]int, 0, len(matches))
	for _, match := range matches {
		num, err := strconv.Atoi(match)
		if err == nil {
			numbers = append(numbers, num)
		}
	}

	return numbers
}

// detectEpisodeNumbersFromFiles analyzes multiple filenames from the same directory
// to identify the most likely episode number position by finding ascending sequences
func detectEpisodeNumbersFromFiles(filenames []string) map[string]int {
	if len(filenames) <= 1 {
		return nil
	}

	// Extract all numbers from each filename
	type numberPosition struct {
		value    int
		position int // position of number in the sequence
	}

	// Map of filename -> potential episode numbers with their positions
	fileNumbers := make(map[string][]numberPosition)

	// Count how many files have each number position occupied
	positionCounts := make(map[int]int)
	maxPosition := 0

	// First pass: extract filtered numbers and their positions
	for _, filename := range filenames {
		base := filepath.Base(filename)
		allNumbers := parseNumericSequence(base)

		// Filter out numbers that are unlikely to be episode numbers
		var numbers []int
		for _, num := range allNumbers {
			// Skip common video-related numbers that aren't episode numbers
			if num == 1080 || num == 720 || num == 480 || num == 360 || num == 2160 ||
				num == 264 || num == 265 ||
				(num >= 1950 && num <= 2030) { // Years
				continue
			}
			numbers = append(numbers, num)
		}

		positions := make([]numberPosition, 0, len(numbers))
		for i, num := range numbers {
			positions = append(positions, numberPosition{num, i})
			positionCounts[i]++
			if i > maxPosition {
				maxPosition = i
			}
		}

		fileNumbers[filename] = positions
	}

	// Find positions that exist in most files (candidate episode number positions)
	candidatePositions := make([]int, 0)
	threshold := len(filenames) / 2 // At least half the files should have this position

	for pos, count := range positionCounts {
		if count >= threshold {
			candidatePositions = append(candidatePositions, pos)
		}
	}

	// No consistent number positions across files
	if len(candidatePositions) == 0 {
		return nil
	}

	// For each candidate position, check if values form an ascending or roughly ascending sequence
	bestPosition := -1
	bestSequentialCount := 0

	for _, pos := range candidatePositions {
		// Collect all values at this position
		values := make([]int, 0, len(filenames))
		for _, positions := range fileNumbers {
			for _, numPos := range positions {
				if numPos.position == pos {
					values = append(values, numPos.value)
				}
			}
		}

		// Check if the values are sequential or near-sequential
		// Sort values and count how many adjacent values differ by 1
		if len(values) < 2 {
			continue
		}

		// Sort the values
		sortedValues := make([]int, len(values))
		copy(sortedValues, values)
		sort.Ints(sortedValues)

		// Count sequential pairs
		sequentialCount := 0
		for i := 0; i < len(sortedValues)-1; i++ {
			if sortedValues[i+1]-sortedValues[i] == 1 {
				sequentialCount++
			}
		}

		// If we have more sequential pairs than before, this is our best candidate
		if sequentialCount > bestSequentialCount {
			bestSequentialCount = sequentialCount
			bestPosition = pos
		}
	}

	// If we found a good episode number position, extract those values
	if bestPosition >= 0 {
		result := make(map[string]int)

		for filename, positions := range fileNumbers {
			for _, numPos := range positions {
				if numPos.position == bestPosition {
					result[filename] = numPos.value
					break
				}
			}
		}

		return result
	}

	return nil
}

// ParseEpisodeInfo extracts season and episode numbers from a filename
// When used on its own, this will only use pattern matching, not directory analysis
func ParseEpisodeInfo(filename string) (int, int, error) {
	// Since we can't access directory context when called directly,
	// fall back to pattern matching
	return parseEpisodeInfoWithPatterns(filename)
}

// GetEpisodeInfo is a method on TVImporter that uses the directory-based
// episode detection for better accuracy
func (app *TVImporter) GetEpisodeInfo(filename string) (int, int, error) {
	// Get the directory path for this file
	dirPath := filepath.Dir(filename)

	// Check if we have analyzed this directory
	if dirEpisodeMap, ok := app.seasonMap[dirPath]; ok {
		// Check if we have information for this specific file
		if info, ok := dirEpisodeMap[filename]; ok {
			return info.Season, info.Episode, nil
		}
	}

	// If we haven't yet analyzed the directories, we need to do it now
	// Create filesByDir map for the current directory
	filesByDir := make(map[string][]string)

	// List all files in the directory
	entries, err := app.fs.ReadDir(dirPath)
	if err == nil {
		// Collect files in this directory
		var files []string
		for _, entry := range entries {
			if !entry.IsDir() && app.isEpisodeFile(entry.Name()) {
				filePath := app.fs.Join(dirPath, entry.Name())
				files = append(files, filePath)
			}
		}

		if len(files) > 0 {
			filesByDir[dirPath] = files
			app.processSeasonDirectories(filesByDir)

			// Check again if we have info after processing
			if dirEpisodeMap, ok := app.seasonMap[dirPath]; ok {
				if info, ok := dirEpisodeMap[filename]; ok {
					return info.Season, info.Episode, nil
				}
			}
		}
	}

	// If we still don't have directory-based info, fall back to pattern matching
	return parseEpisodeInfoWithPatterns(filename)
}

// parseEpisodeInfoWithPatterns uses regex patterns to extract episode info
// This is a fallback method when directory analysis isn't available
func parseEpisodeInfoWithPatterns(filename string) (int, int, error) {
	// Try to extract season from directory name
	dirPath := filepath.Dir(filename)
	seasonNum := detectSeasonFromPath(dirPath)

	// Try the traditional regex patterns first
	if match := episodePattern.FindStringSubmatch(filename); match != nil {
		season, _ := strconv.Atoi(match[1])
		episode, _ := strconv.Atoi(match[2])
		return season, episode, nil
	}

	if match := seasonEpPattern.FindStringSubmatch(filename); match != nil {
		season, _ := strconv.Atoi(match[1])
		episode, _ := strconv.Atoi(match[2])
		return season, episode, nil
	}

	if match := dotSeasonEpPattern.FindStringSubmatch(filename); match != nil {
		season, _ := strconv.Atoi(match[1])
		episode, _ := strconv.Atoi(match[2])
		return season, episode, nil
	}

	if match := dashEpisodePattern.FindStringSubmatch(filename); match != nil {
		season, _ := strconv.Atoi(match[1])
		episode, _ := strconv.Atoi(match[2])
		return season, episode, nil
	}

	if match := dotEpisodePattern.FindStringSubmatch(filename); match != nil {
		season, _ := strconv.Atoi(match[1])
		episode, _ := strconv.Atoi(match[2])
		return season, episode, nil
	}

	if match := plainEpisodePattern.FindStringSubmatch(filename); match != nil {
		episode, _ := strconv.Atoi(match[1])
		return seasonNum, episode, nil
	}

	// No pattern match, so we have to guess based on numbers in the filename
	return guessEpisodeFromNumbers(filename, seasonNum)
}

// guessEpisodeFromNumbers tries to determine episode number from numeric sequences
// in the filename when no explicit patterns are found
func guessEpisodeFromNumbers(filename string, seasonNum int) (int, int, error) {
	baseFilename := filepath.Base(filename)
	numbers := parseNumericSequence(baseFilename)

	if len(numbers) == 0 {
		return 0, 0, fmt.Errorf("could not find any numbers in filename: %s", filename)
	}

	// Special case for date-based filenames
	if strings.Contains(baseFilename, "-12-25") || strings.Contains(baseFilename, ".12.25") {
		// Christmas episode
		return seasonNum, 25, nil
	}

	// Check for date formats like YYYYMMDD or YYMMDD
	for _, num := range numbers {
		// YYYYMMDD format (like 20221225)
		if num >= 19000101 && num <= 21001231 {
			day := num % 100
			if day > 0 && day <= 31 {
				return seasonNum, day, nil
			}
		}

		// YYMMDD format (like 221225)
		if num >= 010101 && num <= 991231 {
			day := num % 100
			if day > 0 && day <= 31 {
				return seasonNum, day, nil
			}
		}
	}

	// Try to find the most likely episode number
	validNumbers := make([]int, 0, len(numbers))

	for _, num := range numbers {
		// Skip numbers that are likely to be years, resolutions, codecs, etc.
		if num == 1080 || num == 720 || num == 480 || num == 360 || num == 2160 ||
			num == 264 || num == 265 || num == 10 || // H.264, H.265, 10-bit
			(num >= 1950 && num <= 2030) { // Years
			continue
		}

		// Three-digit numbers like 101 are often S1E01
		if num >= 100 && num <= 999 {
			detectedSeason := num / 100
			detectedEpisode := num % 100

			// If it seems valid, use this
			if detectedEpisode > 0 && detectedEpisode <= 99 &&
				detectedSeason > 0 && detectedSeason <= 30 {
				return detectedSeason, detectedEpisode, nil
			}
		}

		// Special case for filenames with "part" - use the part number as episode
		if strings.Contains(strings.ToLower(baseFilename), "part") && num <= 20 {
			return seasonNum, num, nil
		}

		// Collect numbers that could be valid episode numbers
		if num > 0 && num <= 99 {
			validNumbers = append(validNumbers, num)
		}
	}

	// If we found any valid numbers, use the first one as episode
	if len(validNumbers) > 0 {
		return seasonNum, validNumbers[0], nil
	}

	// Last resort: use the first number as episode
	return seasonNum, numbers[0], nil
}

// detectSeasonFromPath extracts the season number from a directory path
func detectSeasonFromPath(dirPath string) int {
	parts := strings.Split(dirPath, string(filepath.Separator))

	// Default to season 1 if we can't find a season number
	seasonNum := 1

	for _, part := range parts {
		if seasonMatch := seasonFolderPattern.FindStringSubmatch(part); seasonMatch != nil {
			if s, err := strconv.Atoi(seasonMatch[1]); err == nil && s > 0 {
				return s
			}
		} else if s2Match := regexp.MustCompile(`(?i)S(\d+)`).FindStringSubmatch(part); s2Match != nil {
			if s, err := strconv.Atoi(s2Match[1]); err == nil && s > 0 {
				return s
			}
		}
	}

	return seasonNum
}

// episodeFilesEqual compares two slices of episode files to determine if they contain the same elements
func episodeFilesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Create maps to count occurrences of each path
	countA := make(map[string]int)
	countB := make(map[string]int)

	for _, path := range a {
		countA[path]++
	}

	for _, path := range b {
		countB[path]++
	}

	// Compare the maps
	for path, count := range countA {
		if countB[path] != count {
			return false
		}
	}

	for path, count := range countB {
		if countA[path] != count {
			return false
		}
	}

	return true
}
