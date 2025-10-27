package filenames

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Common TV show episode patterns
var (
	// Standard format: S01E02, S01.E02, S01 E02, S01-E02, S01_E02
	EpisodePattern = regexp.MustCompile(`(?i)S(\d+)[.\s_-]*E(\d+)`)

	// Format: Season 1 Episode 2
	SeasonEpPattern = regexp.MustCompile(`(?i)Season\s*(\d+).*?Episode\s*(\d+)`)

	// Format: 1x02
	DotSeasonEpPattern = regexp.MustCompile(`(\d+)x(\d+)`)

	// Format: Season 1 (for directories)
	SeasonFolderPattern = regexp.MustCompile(`(?i)Season\s*(\d+)`)

	// Format: S01, S02, etc. (for directories or filenames)
	SPattern = regexp.MustCompile(`(?i)S(\d+)`)

	// Format: The Eric Andre Show - 101 - George Clooney.mkv
	DashEpisodePattern = regexp.MustCompile(`(?i).*?[- ]\s*(\d)(\d{2})\s*[- ]`)

	// Format: the.good.wife.509.hdtv-lol.mp4
	DotEpisodePattern = regexp.MustCompile(`(?i).*?\.(\d)(\d{2})\.`)

	// Format: [OZC]The Big O E14 'Roger the Wanderer'.mkv
	// Matches E followed by digits, with a word boundary to avoid matching within words
	PlainEpisodePattern = regexp.MustCompile(`(?i)\bE(\d+)\b`)

	// Format: 520.mkv or just 520 (bare 3-digit: first digit is season, last two are episode)
	ThreeDigitPattern = regexp.MustCompile(`^(\d)(\d{2})(?:\.[a-z0-9]+)?$`)
)

// ParseSeasonEpisode extracts season and episode numbers from a file path or filename
// Returns (season, episode, error)
func ParseSeasonEpisode(path string) (int, int, error) {
	// Extract the filename from the path
	dir, filename := filepath.Split(path)

	// Try all known episode patterns on the filename
	if match := EpisodePattern.FindStringSubmatch(filename); match != nil {
		season, _ := strconv.Atoi(match[1])
		episode, _ := strconv.Atoi(match[2])
		return season, episode, nil
	}

	if match := SeasonEpPattern.FindStringSubmatch(filename); match != nil {
		season, _ := strconv.Atoi(match[1])
		episode, _ := strconv.Atoi(match[2])
		return season, episode, nil
	}

	if match := DotSeasonEpPattern.FindStringSubmatch(filename); match != nil {
		season, _ := strconv.Atoi(match[1])
		episode, _ := strconv.Atoi(match[2])
		return season, episode, nil
	}

	if match := DashEpisodePattern.FindStringSubmatch(filename); match != nil {
		season, _ := strconv.Atoi(match[1])
		episode, _ := strconv.Atoi(match[2])
		return season, episode, nil
	}

	if match := DotEpisodePattern.FindStringSubmatch(filename); match != nil {
		season, _ := strconv.Atoi(match[1])
		episode, _ := strconv.Atoi(match[2])
		return season, episode, nil
	}

	// Try bare three-digit format (e.g., "520.mkv" or "520")
	// This is less specific, so check it after the delimited patterns
	if match := ThreeDigitPattern.FindStringSubmatch(filename); match != nil {
		season, _ := strconv.Atoi(match[1])
		episode, _ := strconv.Atoi(match[2])
		return season, episode, nil
	}

	// Check for season directories in the path
	parts := strings.Split(path, "/")
	var seasonNum int

	for _, part := range parts {
		// Look for "Season X" directory pattern
		if seasonMatch := SeasonFolderPattern.FindStringSubmatch(part); seasonMatch != nil {
			seasonNum, _ = strconv.Atoi(seasonMatch[1])

			// Look for plain episode pattern in filename
			if match := PlainEpisodePattern.FindStringSubmatch(filename); match != nil {
				episode, _ := strconv.Atoi(match[1])
				return seasonNum, episode, nil
			}

			// Look for just a number as the episode
			re := regexp.MustCompile(`(\d+)`)
			if match := re.FindStringSubmatch(filename); match != nil {
				episodeNum, _ := strconv.Atoi(match[1])
				return seasonNum, episodeNum, nil
			}
		} else if sMatch := SPattern.FindStringSubmatch(part); sMatch != nil {
			// Also check for S## pattern (e.g., "S02" in directory name)
			seasonNum, _ = strconv.Atoi(sMatch[1])

			// Look for plain episode pattern in filename
			if match := PlainEpisodePattern.FindStringSubmatch(filename); match != nil {
				episode, _ := strconv.Atoi(match[1])
				return seasonNum, episode, nil
			}

			// Look for just a number as the episode
			re := regexp.MustCompile(`(\d+)`)
			if match := re.FindStringSubmatch(filename); match != nil {
				episodeNum, _ := strconv.Atoi(match[1])
				return seasonNum, episodeNum, nil
			}
		}
	}

	// Last resort: Check if the directory name is in season format and filename has numbers
	dirParts := strings.Split(dir, "/")
	for _, part := range dirParts {
		if seasonMatch := SeasonFolderPattern.FindStringSubmatch(part); seasonMatch != nil {
			seasonNum, _ := strconv.Atoi(seasonMatch[1])

			// Try to find episode number in filename
			episodeMatch := regexp.MustCompile(`(\d+)`).FindStringSubmatch(filename)
			if episodeMatch != nil {
				episodeNum, _ := strconv.Atoi(episodeMatch[1])
				return seasonNum, episodeNum, nil
			}
		} else if sMatch := SPattern.FindStringSubmatch(part); sMatch != nil {
			seasonNum, _ := strconv.Atoi(sMatch[1])

			// Try to find episode number in filename
			episodeMatch := regexp.MustCompile(`(\d+)`).FindStringSubmatch(filename)
			if episodeMatch != nil {
				episodeNum, _ := strconv.Atoi(episodeMatch[1])
				return seasonNum, episodeNum, nil
			}
		}
	}

	// If we couldn't match any pattern, return an error rather than guessing
	return 0, 0, fmt.Errorf("could not extract season and episode from path: %s", path)
}

// DetectSeasonFromPath extracts the season number from a directory path
// Returns season number (defaults to 1 if not found)
func DetectSeasonFromPath(dirPath string) int {
	parts := strings.Split(dirPath, string(filepath.Separator))

	for _, part := range parts {
		if seasonMatch := SeasonFolderPattern.FindStringSubmatch(part); seasonMatch != nil {
			if s, err := strconv.Atoi(seasonMatch[1]); err == nil && s > 0 {
				return s
			}
		} else if s2Match := SPattern.FindStringSubmatch(part); s2Match != nil {
			if s, err := strconv.Atoi(s2Match[1]); err == nil && s > 0 {
				return s
			}
		}
	}

	return 1 // Default to season 1
}

// IsEpisodeFile checks if a filename matches TV episode file patterns
func IsEpisodeFile(filename string) bool {
	// Check file extension
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".mkv" && ext != ".mp4" && ext != ".avi" && ext != ".m4v" {
		return false
	}

	// Check for explicit episode patterns in filename
	if EpisodePattern.MatchString(filename) ||
		SeasonEpPattern.MatchString(filename) ||
		DotSeasonEpPattern.MatchString(filename) ||
		DashEpisodePattern.MatchString(filename) ||
		DotEpisodePattern.MatchString(filename) ||
		PlainEpisodePattern.MatchString(filename) {
		return true
	}

	// If we didn't match a specific pattern, check if the filename contains any numbers
	re := regexp.MustCompile(`\d+`)
	return re.MatchString(filename)
}
