package tmdb

import (
	"fmt"
	"strings"
)

// MockTMDB is a mock TMDB client for testing
type MockTMDB struct {
	// Pre-defined responses
	Movies          map[int64]*Movie
	TVShows         map[int64]*TVShow
	Seasons         map[string]*Season          // key: "showID_seasonNumber"
	Episodes        map[string]*Episode         // key: "showID_seasonNumber_episodeNumber"
	SearchResults   map[string][]SearchResult   // key: "query_year"
	TVSearchResults map[string][]TVSearchResult // key: "query_year"

	// Embed the real client to satisfy the interface
	// This allows the mock to be used wherever a Client is expected
	*Client
}

// NewMockTMDB creates a new mock TMDB client
func NewMockTMDB() *MockTMDB {
	// Create a dummy client just to satisfy the interface
	dummyClient := &Client{
		apiKey:    "mock-api-key",
		readToken: "mock-read-token",
	}

	return &MockTMDB{
		Movies:          make(map[int64]*Movie),
		TVShows:         make(map[int64]*TVShow),
		Seasons:         make(map[string]*Season),
		Episodes:        make(map[string]*Episode),
		SearchResults:   make(map[string][]SearchResult),
		TVSearchResults: make(map[string][]TVSearchResult),
		Client:          dummyClient,
	}
}

// AddMovie adds a movie to the mock client
func (m *MockTMDB) AddMovie(movie *Movie) {
	m.Movies[movie.ID] = movie
}

// AddTVShow adds a TV show to the mock client
func (m *MockTMDB) AddTVShow(show *TVShow) {
	m.TVShows[show.ID] = show
}

// AddSeason adds a season to the mock client
func (m *MockTMDB) AddSeason(showID int64, season *Season) {
	key := fmt.Sprintf("%d_%d", showID, season.SeasonNumber)
	m.Seasons[key] = season
}

// AddEpisode adds an episode to the mock client
func (m *MockTMDB) AddEpisode(showID int64, episode *Episode) {
	key := fmt.Sprintf("%d_%d_%d", showID, episode.SeasonNumber, episode.EpisodeNumber)
	m.Episodes[key] = episode
}

// AddSearchResults adds movie search results to the mock client
func (m *MockTMDB) AddSearchResults(query, year string, results []SearchResult) {
	key := fmt.Sprintf("%s_%s", query, year)
	m.SearchResults[key] = results
}

// AddTVSearchResults adds TV search results to the mock client
func (m *MockTMDB) AddTVSearchResults(query, year string, results []TVSearchResult) {
	key := fmt.Sprintf("%s_%s", query, year)
	m.TVSearchResults[key] = results
}

// Implementation of TMDB client interface

// Get returns a movie by ID
func (m *MockTMDB) Get(id int64) (*Movie, error) {
	movie, ok := m.Movies[id]
	if !ok {
		return nil, fmt.Errorf("movie not found")
	}
	return movie, nil
}

// Search searches for movies
func (m *MockTMDB) Search(q, year string) ([]SearchResult, error) {
	key := fmt.Sprintf("%s_%s", q, year)
	results, ok := m.SearchResults[key]
	if !ok {
		return []SearchResult{}, nil
	}
	return results, nil
}

// GetTV returns a TV show by ID
func (m *MockTMDB) GetTV(id int64) (*TVShow, error) {
	show, ok := m.TVShows[id]
	if !ok {
		return nil, fmt.Errorf("TV show not found")
	}
	return show, nil
}

// SearchTV searches for TV shows
func (m *MockTMDB) SearchTV(q, year string) ([]TVSearchResult, error) {
	key := fmt.Sprintf("%s_%s", q, year)
	results, ok := m.TVSearchResults[key]
	if !ok {
		return []TVSearchResult{}, nil
	}
	return results, nil
}

// GetSeason returns a season and its episodes
func (m *MockTMDB) GetSeason(showID int64, seasonNumber int) (*Season, []Episode, error) {
	key := fmt.Sprintf("%d_%d", showID, seasonNumber)
	season, ok := m.Seasons[key]
	if !ok {
		return nil, nil, fmt.Errorf("season not found")
	}

	// Find all episodes for this season
	var episodes []Episode
	for epKey, episode := range m.Episodes {
		if strings.HasPrefix(epKey, fmt.Sprintf("%d_%d_", showID, seasonNumber)) {
			episodes = append(episodes, *episode)
		}
	}

	return season, episodes, nil
}

// GetEpisode returns an episode
func (m *MockTMDB) GetEpisode(showID int64, seasonNumber, episodeNumber int) (*Episode, error) {
	key := fmt.Sprintf("%d_%d_%d", showID, seasonNumber, episodeNumber)
	episode, ok := m.Episodes[key]
	if !ok {
		return nil, fmt.Errorf("episode not found")
	}
	return episode, nil
}
