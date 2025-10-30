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
	// Use word boundaries and limit season/episode to reasonable ranges to avoid matching resolutions like 1008x720
	DotSeasonEpPattern = regexp.MustCompile(`\b(\d{1,2})x(\d{1,3})\b`)

	// Format: Season 1 (for directories)
	SeasonFolderPattern = regexp.MustCompile(`(?i)Season\s*(\d+)`)

	// Format: S01, S02, etc. (for directories or filenames)
	SPattern = regexp.MustCompile(`(?i)S(\d+)`)

	// Format: The Eric Andre Show - 101 - George Clooney.mkv
	DashEpisodePattern = regexp.MustCompile(`(?i).*?[- ]\s*(\d)(\d{2})\s*[- ]`)

	// Format: the.good.wife.509.hdtv-lol.mp4
	// Match exactly 3 digits between dots (not 2, not 4+)
	// Avoid matching resolutions like 720p, 1080p by excluding 'p' and 'i' as the following character
	DotEpisodePattern = regexp.MustCompile(`(?i).*?\.(\d)(\d{2})(?:[^\dpi]|$)`)

	// Format: mad.men.0402.hdtv.avi (4 digits: 2 for season, 2 for episode)
	// Match exactly 4 digits between dots representing SSEE format
	FourDigitPattern = regexp.MustCompile(`(?i).*?\.(\d{2})(\d{2})\.`)

	// Format: [OZC]The Big O E14 'Roger the Wanderer'.mkv
	// Matches E followed by digits, with a word boundary to avoid matching within words
	PlainEpisodePattern = regexp.MustCompile(`(?i)\bE(\d+)\b`)

	// Format: Baccano! Ep. 14.mkv
	// Matches "Ep." or "Episode" followed by optional space/period and digits
	EpDotPattern = regexp.MustCompile(`(?i)\bEp(?:isode)?[.\s]+(\d+)\b`)

	// Format: Show_Name_01_Title.mkv or Show - 01 [tags].mkv
	// Matches underscore/dash followed by exactly 2 digits followed by delimiter (space, underscore, dash, dot, open paren/bracket)
	// This catches simple episode numbering like "_01_" or "- 05 " or "-07."
	SimpleEpisodePattern = regexp.MustCompile(`[_-]\s*(\d{2})\s*[_\-\.\(\[\s]`)

	// Format: 520.mkv, 520, or "112 Untitled.avi" (bare 3-digit: first digit is season, last two are episode)
	// Match 3 digits at start followed by space, dot, or end of string
	ThreeDigitPattern = regexp.MustCompile(`^(\d)(\d{2})(?:\s|\.|\b)`)
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

	// Try 4-digit SSEE format (e.g., "mad.men.0402.hdtv.avi" -> S04E02)
	if match := FourDigitPattern.FindStringSubmatch(filename); match != nil {
		season, _ := strconv.Atoi(match[1])
		episode, _ := strconv.Atoi(match[2])
		return season, episode, nil
	}

	// Try "Ep. N" pattern (e.g., "Baccano! Ep. 14.mkv")
	if match := EpDotPattern.FindStringSubmatch(filename); match != nil {
		episode, _ := strconv.Atoi(match[1])
		// Use season from directory path, default to 1
		seasonNum := DetectSeasonFromPath(filepath.Dir(path))
		return seasonNum, episode, nil
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

	// Try simple two-digit episode pattern (e.g., "_01_" or "-05-")
	// Check this before falling back to directory-based detection
	if match := SimpleEpisodePattern.FindStringSubmatch(filename); match != nil {
		episode, _ := strconv.Atoi(match[1])
		// Use season from directory path
		seasonNum := DetectSeasonFromPath(filepath.Dir(path))
		return seasonNum, episode, nil
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

			// Check for 3-digit episode codes where first digit matches season
			// e.g., "601 - Title.mkv" in "S06" directory -> S06E01
			// Look for 3 digits at start or after delimiter
			threeDigitMatch := regexp.MustCompile(`^(\d)(\d{2})\b`).FindStringSubmatch(filename)
			if threeDigitMatch != nil {
				detectedSeason, _ := strconv.Atoi(threeDigitMatch[1])
				if detectedSeason == seasonNum {
					episode, _ := strconv.Atoi(threeDigitMatch[2])
					return seasonNum, episode, nil
				}
			}

			if match := DashEpisodePattern.FindStringSubmatch(filename); match != nil {
				detectedSeason, _ := strconv.Atoi(match[1])
				if detectedSeason == seasonNum {
					episode, _ := strconv.Atoi(match[2])
					return seasonNum, episode, nil
				}
			}
			if match := DotEpisodePattern.FindStringSubmatch(filename); match != nil {
				detectedSeason, _ := strconv.Atoi(match[1])
				if detectedSeason == seasonNum {
					episode, _ := strconv.Atoi(match[2])
					return seasonNum, episode, nil
				}
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

			// Check for 3-digit episode codes where first digit matches season
			// e.g., "601 - Title.mkv" in "S06" directory -> S06E01
			threeDigitMatch := regexp.MustCompile(`^(\d)(\d{2})\b`).FindStringSubmatch(filename)
			if threeDigitMatch != nil {
				detectedSeason, _ := strconv.Atoi(threeDigitMatch[1])
				if detectedSeason == seasonNum {
					episode, _ := strconv.Atoi(threeDigitMatch[2])
					return seasonNum, episode, nil
				}
			}

			if match := DashEpisodePattern.FindStringSubmatch(filename); match != nil {
				detectedSeason, _ := strconv.Atoi(match[1])
				if detectedSeason == seasonNum {
					episode, _ := strconv.Atoi(match[2])
					return seasonNum, episode, nil
				}
			}
			if match := DotEpisodePattern.FindStringSubmatch(filename); match != nil {
				detectedSeason, _ := strconv.Atoi(match[1])
				if detectedSeason == seasonNum {
					episode, _ := strconv.Atoi(match[2])
					return seasonNum, episode, nil
				}
			}

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
		FourDigitPattern.MatchString(filename) ||
		EpDotPattern.MatchString(filename) ||
		DashEpisodePattern.MatchString(filename) ||
		DotEpisodePattern.MatchString(filename) ||
		PlainEpisodePattern.MatchString(filename) ||
		SimpleEpisodePattern.MatchString(filename) {
		return true
	}

	// If we didn't match a specific pattern, check if the filename contains any numbers
	re := regexp.MustCompile(`\d+`)
	return re.MatchString(filename)
}
