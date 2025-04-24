package db

import (
	"fmt"
	"sync"
)

// MockDB implements a simple in-memory database for testing
type MockDB struct {
	stubs       map[string]*Stub
	ignores     map[string]MediaType
	movies      map[int64]*Movie
	tvShows     map[int64]*TVShow
	tvSeasons   map[string]*TVSeason // key: "showID_seasonNumber"
	tvEpisodes  map[string]*TVEpisode // key: "showID_seasonNumber_episodeNumber"
	
	createdStubs []string
	mu           sync.Mutex
	
	// Embed the real DB to satisfy the interface
	*DB
}

// NewMockDB creates a new mock database
func NewMockDB() *MockDB {
	// Create a minimal DB to satisfy the interface
	mockDB := &MockDB{
		stubs:        make(map[string]*Stub),
		ignores:      make(map[string]MediaType),
		movies:       make(map[int64]*Movie),
		tvShows:      make(map[int64]*TVShow),
		tvSeasons:    make(map[string]*TVSeason),
		tvEpisodes:   make(map[string]*TVEpisode),
		createdStubs: make([]string, 0),
	}
	
	// Create a minimal real DB to satisfy the interface
	mockDB.DB = &DB{
		path: ":memory:", // For SQLite, in-memory database
	}
	
	return mockDB
}

// StubWasCreated checks if a stub was created for a path
func (db *MockDB) StubWasCreated(path string) bool {
	for _, stubPath := range db.createdStubs {
		if stubPath == path {
			return true
		}
	}
	return false
}

// Stub-related methods

// GetStub retrieves a stub by path
func (db *MockDB) GetStub(importedFromPath string) (*Stub, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	stub, ok := db.stubs[importedFromPath]
	if !ok {
		return nil, fmt.Errorf("stub not found")
	}
	return stub, nil
}

// AllStubs returns all stubs
func (db *MockDB) AllStubs() ([]*Stub, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	stubs := make([]*Stub, 0, len(db.stubs))
	for _, stub := range db.stubs {
		stubs = append(stubs, stub)
	}
	return stubs, nil
}

// SaveStub updates a stub
func (db *MockDB) SaveStub(stub *Stub) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	db.stubs[stub.ImportedFromPath] = stub
	return nil
}

// DeleteStub removes a stub
func (db *MockDB) DeleteStub(stub *Stub) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	delete(db.stubs, stub.ImportedFromPath)
	return nil
}

// Ignore-related methods

// IgnorePath marks a path as ignored
func (db *MockDB) IgnorePath(mediaType MediaType, path string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	db.ignores[path] = mediaType
	return nil
}

// Movie-related methods

// CreateMovie creates a movie in the mock database
func (db *MockDB) CreateMovie(movie *Movie) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	db.movies[movie.ID] = movie
	return nil
}

// GetMovie retrieves a movie by ID
func (db *MockDB) GetMovie(id int64) (*Movie, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	movie, ok := db.movies[id]
	if !ok {
		return nil, fmt.Errorf("movie not found")
	}
	return movie, nil
}

// MovieExistsFromPath checks if a movie exists for a path
func (db *MockDB) MovieExistsFromPath(path string) (bool, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	for _, movie := range db.movies {
		if movie.ImportedFromPath == path {
			return true, nil
		}
	}
	return false, nil
}

// TV-related methods

// CreateTVEpisode creates a TV episode in the mock database
func (db *MockDB) CreateTVEpisode(episode *TVEpisode) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	key := fmt.Sprintf("%d_%d_%d", episode.ShowID, episode.SeasonNumber, episode.EpisodeNumber)
	db.tvEpisodes[key] = episode
	return nil
}

// TVEpisodeExistsFromPath checks if a TV episode exists for a path
func (db *MockDB) TVEpisodeExistsFromPath(path string) (bool, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	for _, episode := range db.tvEpisodes {
		if episode.ImportedFromPath == path {
			return true, nil
		}
	}
	return false, nil
}

// Some of the interface methods we need to override from the real DB

// PathIsIgnored checks if a path is ignored (override the real DB method)
func (db *MockDB) PathIsIgnored(mediaType MediaType, path string) (bool, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	ignoreType, ok := db.ignores[path]
	if !ok {
		return false, nil
	}
	return ignoreType == mediaType, nil
}

// StubExistsFromPath checks if a stub exists for a path (override the real DB method)
func (db *MockDB) StubExistsFromPath(mediaType MediaType, path string) (bool, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	stub, ok := db.stubs[path]
	if !ok {
		return false, nil
	}
	return stub.Type == mediaType, nil
}

// CreateStub creates a stub (override the real DB method)
func (db *MockDB) CreateStub(mediaType MediaType, path string) (*Stub, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	stub := &Stub{
		ImportedFromPath: path,
		Type:             mediaType,
	}
	db.stubs[path] = stub
	db.createdStubs = append(db.createdStubs, path)
	return stub, nil
}

// Transaction implements a mock transaction
func (db *MockDB) Transaction(fn func(*DB) error) error {
	// Just call the function with our DB pointer since we're embedded
	return fn(db.DB)
}

// Subscribe returns a channel that can be used to subscribe to database changes
func (db *MockDB) Subscribe() chan *Movie {
	// Return a dummy channel
	return make(chan *Movie)
}

// Start initializes the mock database
func (db *MockDB) Start() error {
	// No real initialization needed for the mock
	return nil
}

// Stop closes the mock database
func (db *MockDB) Stop() error {
	// No real cleanup needed for the mock
	return nil
}