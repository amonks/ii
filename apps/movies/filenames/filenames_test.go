package filenames

import (
	"testing"
)

func TestParseSeasonEpisode(t *testing.T) {
	tests := []struct {
		name            string
		path            string
		expectedSeason  int
		expectedEpisode int
		shouldError     bool
	}{
		{
			name:            "OZC format with S02 in directory and E26 in filename",
			path:            "The Big O Complete Series S02 [720p][OZC]/[OZC]The Big O E26 'The Show Must Go On' [720p].mkv",
			expectedSeason:  2,
			expectedEpisode: 26,
			shouldError:     false,
		},
		{
			name:            "Standard S01E02 format",
			path:            "Show Name/S01E02.mkv",
			expectedSeason:  1,
			expectedEpisode: 2,
			shouldError:     false,
		},
		{
			name:            "Season folder with E14 format",
			path:            "Show/Season 1/[OZC]Show E14 'Title'.mkv",
			expectedSeason:  1,
			expectedEpisode: 14,
			shouldError:     false,
		},
		{
			name:            "1x02 format",
			path:            "Show/1x02.mkv",
			expectedSeason:  1,
			expectedEpisode: 2,
			shouldError:     false,
		},
		{
			name:            "S03 directory with plain E format",
			path:            "Show Complete Series S03/E15.mkv",
			expectedSeason:  3,
			expectedEpisode: 15,
			shouldError:     false,
		},
		{
			name:            "Multi-digit episode with S pattern in directory",
			path:            "The Big O Complete Series S02 [720p][OZC]/[OZC]The Big O E26 'The Show Must Go On' [720p].mkv",
			expectedSeason:  2,
			expectedEpisode: 26,
			shouldError:     false,
		},
		{
			name:            "S01 in middle of directory name",
			path:            "Show Name S01 [1080p]/Show E05.mkv",
			expectedSeason:  1,
			expectedEpisode: 5,
			shouldError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			season, episode, err := ParseSeasonEpisode(tt.path)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if season != tt.expectedSeason {
				t.Errorf("Expected season %d but got %d", tt.expectedSeason, season)
			}

			if episode != tt.expectedEpisode {
				t.Errorf("Expected episode %d but got %d", tt.expectedEpisode, episode)
			}
		})
	}
}

func TestPlainEpisodePattern(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
		episode  string
	}{
		{
			name:     "OZC format with space",
			input:    "[OZC]The Big O E26 'The Show Must Go On' [720p].mkv",
			expected: true,
			episode:  "26",
		},
		{
			name:     "OZC format with bracket",
			input:    "[OZC]Show E14['Title'].mkv",
			expected: true,
			episode:  "14",
		},
		{
			name:     "E followed by letters - should not match",
			input:    "Episode26.mkv",
			expected: false,
		},
		{
			name:     "Plain E26 at end",
			input:    "Show E26.mkv",
			expected: true,
			episode:  "26",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := PlainEpisodePattern.FindStringSubmatch(tt.input)

			if tt.expected {
				if matches == nil {
					t.Errorf("Expected pattern to match but it didn't")
					return
				}
				if len(matches) < 2 {
					t.Errorf("Expected capture group but got none")
					return
				}
				if matches[1] != tt.episode {
					t.Errorf("Expected episode %s but got %s", tt.episode, matches[1])
				}
			} else {
				if matches != nil {
					t.Errorf("Expected pattern not to match but it did: %v", matches)
				}
			}
		})
	}
}
