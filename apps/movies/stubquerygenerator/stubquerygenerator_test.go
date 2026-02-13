package stubquerygenerator

import (
	"context"
	"testing"

	"monks.co/apps/movies/db"
	"monks.co/pkg/tmdb"
)

// MockLLM is a mock implementation of the LLM client
type MockLLM struct {
	movieTitle     string
	movieYear      int
	tvTitle        string
	tvYear         int
	tvSeasonNumber int
	err            error
}

func (m *MockLLM) GenerateMovieQuery(filepath string) (string, int, error) {
	if m.err != nil {
		return "", 0, m.err
	}
	return m.movieTitle, m.movieYear, nil
}

func (m *MockLLM) GenerateTVQuery(filepath string) (string, int, int, error) {
	if m.err != nil {
		return "", 0, 0, m.err
	}
	return m.tvTitle, m.tvYear, m.tvSeasonNumber, nil
}

// MockDB is a mock implementation of the database
type MockDB struct {
	stubs   []*db.Stub
	saved   []*db.Stub
	deleted []*db.Stub
	movies  []*db.Movie
	tvShows []*db.TVShow
}

func (m *MockDB) AllStubs() ([]*db.Stub, error) {
	return m.stubs, nil
}

func (m *MockDB) SaveStub(stub *db.Stub) error {
	m.saved = append(m.saved, stub)
	return nil
}

func (m *MockDB) DeleteStub(stub *db.Stub) error {
	m.deleted = append(m.deleted, stub)
	return nil
}

func (m *MockDB) GetTVShow(id int64) (*db.TVShow, error) {
	for _, show := range m.tvShows {
		if show.ID == id {
			return show, nil
		}
	}
	return nil, nil
}

func (m *MockDB) GetMovie(id int64) (*db.Movie, error) {
	for _, movie := range m.movies {
		if movie.ID == id {
			return movie, nil
		}
	}
	return nil, nil
}

func (m *MockDB) GetTVShows() ([]*db.TVShow, error) {
	return m.tvShows, nil
}

func (m *MockDB) GetMovies() ([]*db.Movie, error) {
	return m.movies, nil
}

func (m *MockDB) IgnorePath(mediaType db.MediaType, path string) error {
	return nil
}

// MockTMDB is a mock implementation of the TMDB client
type MockTMDB struct {
	movieResults []tmdb.SearchResult
	tvResults    []tmdb.TVSearchResult
	err          error
}

func (m *MockTMDB) Search(query, year string) ([]tmdb.SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.movieResults, nil
}

func (m *MockTMDB) SearchTV(query, year string) ([]tmdb.TVSearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tvResults, nil
}

func TestStubQueryGeneratorRun(t *testing.T) {
	mockLLM := &MockLLM{
		movieTitle:     "Test Movie",
		movieYear:      2022,
		tvTitle:        "Test TV Show",
		tvYear:         2022,
		tvSeasonNumber: 1,
	}

	mockTMDB := &MockTMDB{
		movieResults: []tmdb.SearchResult{
			{
				ID:          1,
				Title:       "Test Movie",
				ReleaseDate: "2022-01-01",
			},
		},
		tvResults: []tmdb.TVSearchResult{
			{
				ID:           2,
				Name:         "Test TV Show",
				FirstAirDate: "2022-01-01",
			},
		},
	}

	testCases := []struct {
		name              string
		stubs             []*db.Stub
		expectedPaths     []string // Paths we expect to be processed
		checkSeasonNumber bool     // Whether to check for season number (for TV stubs)
	}{
		{
			name: "Process movie stub without query",
			stubs: []*db.Stub{
				{
					ImportedFromPath: "/path/to/movie.mkv",
					Type:             db.MediaTypeMovie,
				},
			},
			expectedPaths: []string{"/path/to/movie.mkv"},
		},
		{
			name: "Process TV stub without query",
			stubs: []*db.Stub{
				{
					ImportedFromPath: "/path/to/tvshow",
					Type:             db.MediaTypeTV,
				},
			},
			expectedPaths:     []string{"/path/to/tvshow"},
			checkSeasonNumber: true,
		},
		{
			name: "Process multiple stubs",
			stubs: []*db.Stub{
				{
					ImportedFromPath: "/path/to/movie1.mkv",
					Type:             db.MediaTypeMovie,
				},
				{
					ImportedFromPath: "/path/to/tvshow",
					Type:             db.MediaTypeTV,
				},
			},
			expectedPaths:     []string{"/path/to/movie1.mkv", "/path/to/tvshow"},
			checkSeasonNumber: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDB := &MockDB{
				stubs: tc.stubs,
			}

			// Create generator with interface implementations
			generator := &StubQueryGenerator{
				llm:  mockLLM,
				tmdb: mockTMDB,
				db:   mockDB,
			}

			err := generator.Run(context.Background())
			if err != nil {
				t.Fatalf("Run returned error: %v", err)
			}

			// Build a map of all paths that have been processed
			processedPaths := make(map[string]bool)
			for _, stub := range mockDB.saved {
				processedPaths[stub.ImportedFromPath] = true
			}

			// Check that expected paths were processed
			for _, path := range tc.expectedPaths {
				if !processedPaths[path] {
					t.Errorf("Expected path %s to be processed, but it wasn't", path)
				}
			}

			// Check for TV season number if needed
			if tc.checkSeasonNumber {
				foundSeasonNumber := false
				for _, stub := range mockDB.saved {
					if stub.Type == db.MediaTypeTV && stub.SeasonNumber == mockLLM.tvSeasonNumber {
						foundSeasonNumber = true
						break
					}
				}

				if !foundSeasonNumber {
					t.Errorf("Expected at least one TV stub to have season number %d", mockLLM.tvSeasonNumber)
				}
			}
		})
	}
}
