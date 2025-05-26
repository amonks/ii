package stubquerygenerator

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"

	"monks.co/apps/movies/db"
	"monks.co/pkg/llm"
	"monks.co/pkg/tmdb"
)

// LLMClient interface for LLM operations
type LLMClient interface {
	GenerateMovieQuery(filepath string) (string, int, error)
	GenerateTVQuery(filepath string) (string, int, int, error)
}

// movieLLM extends the basic LLM client with movie-specific functionality
type movieLLM struct {
	llmClient *llm.Client
}

// GenerateMovieQuery generates a movie search query from a filepath
func (m *movieLLM) GenerateMovieQuery(filepath string) (string, int, error) {
	prompt := fmt.Sprintf("We have the following filepath, which we think is a movie file. We'd like to look it up in TMDB, and we need to construct a search query for that lookup. IMPORTANT: For the year_query, ONLY return a year if it is explicitly present in the filename. DO NOT guess or use your prior knowledge about when movies were released - if the year isn't in the filename, set year_query to 0. Here is the filepath: %q", filepath)
	schema := "title_query str, year_query int"

	result, err := m.llmClient.GenerateWithSchema(prompt, schema)
	if err != nil {
		return "", 0, err
	}

	titleQuery, ok := result["title_query"].(string)
	if !ok {
		return "", 0, fmt.Errorf("invalid title_query in LLM response: %v", result)
	}

	yearQuery, ok := result["year_query"].(float64)
	if !ok {
		return "", 0, fmt.Errorf("invalid year_query in LLM response: %v", result)
	}

	return titleQuery, int(yearQuery), nil
}

// GenerateTVQuery generates a TV show search query from a filepath
func (m *movieLLM) GenerateTVQuery(filepath string) (string, int, int, error) {
	prompt := fmt.Sprintf("We have the following filepath, which we think is a TV series folder. We'd like to look it up in TMDB, and we need to construct a search query for that lookup. Extract the TV show name and determine which season this folder likely represents. IMPORTANT: For the year_query, ONLY return a year if it is explicitly present in the filename. DO NOT guess or use your prior knowledge about when the TV series first aired - if the year isn't in the filename, set year_query to 0. Here is the filepath: %q", filepath)
	schema := "title_query str, year_query int, season_number int"

	result, err := m.llmClient.GenerateWithSchema(prompt, schema)
	if err != nil {
		return "", 0, 0, err
	}

	titleQuery, ok := result["title_query"].(string)
	if !ok {
		return "", 0, 0, fmt.Errorf("invalid title_query in LLM response: %v", result)
	}

	yearQuery, ok := result["year_query"].(float64)
	if !ok {
		// Year might be optional for TV shows
		yearQuery = 0
	}

	seasonNumber, ok := result["season_number"].(float64)
	if !ok {
		// Default to season 1 if not detected
		seasonNumber = 1
	}

	return titleQuery, int(yearQuery), int(seasonNumber), nil
}

// TMDBClient interface for TMDB operations
type TMDBClient interface {
	Search(query, year string) ([]tmdb.SearchResult, error)
	SearchTV(query, year string) ([]tmdb.TVSearchResult, error)
}

// DBClient interface for database operations
type DBClient interface {
	AllStubs() ([]*db.Stub, error)
	SaveStub(stub *db.Stub) error
	DeleteStub(stub *db.Stub) error
	GetTVShow(id int64) (*db.TVShow, error)
	GetMovie(id int64) (*db.Movie, error)
	GetTVShows() ([]*db.TVShow, error)
	GetMovies() ([]*db.Movie, error)
	IgnorePath(mediaType db.MediaType, path string) error
}

// StubQueryGenerator generates search queries for stubs without existing queries
type StubQueryGenerator struct {
	llm   LLMClient
	tmdb  TMDBClient
	db    DBClient
	mutex sync.Mutex
}

// New creates a new StubQueryGenerator
func New(llmClient *llm.Client, tmdb *tmdb.Client, db *db.DB) *StubQueryGenerator {
	// Create movie-specific LLM client that wraps the base LLM client
	movieClient := &movieLLM{
		llmClient: llmClient,
	}

	return &StubQueryGenerator{
		llm:  movieClient,
		tmdb: tmdb,
		db:   db,
	}
}

// processStub processes a single stub
func (app *StubQueryGenerator) processStub(stub *db.Stub) error {
	// Skip if already processed
	if len(stub.Results) > 0 || len(stub.TVResults) > 0 {
		// If we have results, update status if not set
		if stub.SearchStatus == "" || stub.SearchStatus == "pending" || stub.SearchStatus == "searching" {
			if len(stub.Results) == 0 && len(stub.TVResults) == 0 {
				stub.SearchStatus = "needs_manual"
			} else {
				stub.SearchStatus = "complete"
			}
			if err := app.db.SaveStub(stub); err != nil {
				log.Printf("error updating search status for stub %s: %v", stub.ImportedFromPath, err)
			}
		}
		return nil
	}

	// Check if this stub has already been imported but not deleted
	if stub.Type == db.MediaTypeMovie {
		// For movies - check if any of the IDs from Results exist in movies table
		for _, result := range stub.Results {
			id := result.ID

			movie, err := app.db.GetMovie(id)
			if err == nil && movie != nil {
				log.Printf("Movie stub %s has already been imported (TMDB ID %d). Skipping.", stub.ImportedFromPath, id)
				return nil
			}
		}
	} else if stub.Type == db.MediaTypeTV {
		// For TV shows - check if any of the IDs from TVResults exist in tv_shows table
		for _, result := range stub.TVResults {
			tv, err := app.db.GetTVShow(result.ID)
			if err == nil && tv != nil {
				log.Printf("TV show stub %s has already been imported (TMDB ID %d). Skipping.", stub.ImportedFromPath, result.ID)
				return nil
			}
		}
	}

	// Skip if we already have a query but no search done yet
	if stub.Query != "" {
		// We have a query but no results, perform search
		if stub.SearchStatus != "complete" {
			// Execute search immediately using the stored query
			if stub.Type == db.MediaTypeMovie {
				// For movies
				results, err := app.tmdb.Search(stub.Query, stub.Year)
				if err != nil {
					log.Printf("error performing search for stub %s: %v", stub.ImportedFromPath, err)
					stub.SearchStatus = "needs_manual"
				} else {
					stub.Results = results
					log.Printf("found %d results for movie stub: %s", len(results), stub.ImportedFromPath)

					if len(results) == 0 && stub.Year != "" {
						// If we got no results with year, try again without year
						log.Printf("no results found with year %s, trying again without year for: %s", stub.Year, stub.ImportedFromPath)
						results, err = app.tmdb.Search(stub.Query, "")
						if err != nil {
							log.Printf("error performing year-less search for stub %s: %v", stub.ImportedFromPath, err)
							stub.SearchStatus = "needs_manual"
						} else {
							stub.Results = results
							log.Printf("found %d results without year for movie stub: %s", len(results), stub.ImportedFromPath)

							if len(results) == 0 {
								stub.SearchStatus = "needs_manual"
							} else {
								stub.SearchStatus = "complete"
							}
						}
					} else if len(results) == 0 {
						stub.SearchStatus = "needs_manual"
					} else {
						stub.SearchStatus = "complete"
					}
				}
			} else if stub.Type == db.MediaTypeTV {
				// For TV shows
				results, err := app.tmdb.SearchTV(stub.Query, stub.Year)
				if err != nil {
					log.Printf("error performing search for stub %s: %v", stub.ImportedFromPath, err)
					stub.SearchStatus = "needs_manual"
				} else {
					// Convert from tmdb.TVSearchResult to db.TVSearchResult
					tvResults := make([]db.TVSearchResult, len(results))
					for i, r := range results {
						tvResults[i] = db.TVSearchResult{
							ID:           r.ID,
							Name:         r.Name,
							FirstAirDate: r.FirstAirDate,
						}
					}
					stub.TVResults = tvResults
					log.Printf("found %d results for TV stub: %s", len(results), stub.ImportedFromPath)

					if len(results) == 0 && stub.Year != "" {
						// If we got no results with year, try again without year
						log.Printf("no results found with year %s, trying again without year for: %s", stub.Year, stub.ImportedFromPath)
						results, err = app.tmdb.SearchTV(stub.Query, "")
						if err != nil {
							log.Printf("error performing year-less search for stub %s: %v", stub.ImportedFromPath, err)
							stub.SearchStatus = "needs_manual"
						} else {
							// Convert from tmdb.TVSearchResult to db.TVSearchResult
							tvResults := make([]db.TVSearchResult, len(results))
							for i, r := range results {
								tvResults[i] = db.TVSearchResult{
									ID:           r.ID,
									Name:         r.Name,
									FirstAirDate: r.FirstAirDate,
								}
							}
							stub.TVResults = tvResults
							log.Printf("found %d results without year for TV stub: %s", len(results), stub.ImportedFromPath)

							if len(results) == 0 {
								stub.SearchStatus = "needs_manual"
							} else {
								stub.SearchStatus = "complete"
							}
						}
					} else if len(results) == 0 {
						stub.SearchStatus = "needs_manual"
					} else {
						stub.SearchStatus = "complete"
					}
				}
			}

			if err := app.db.SaveStub(stub); err != nil {
				log.Printf("error updating search status and results for stub %s: %v", stub.ImportedFromPath, err)
			}
		}
		return nil
	}

	// Process new stubs
	log.Printf("generating query for stub: %s (type: %s)", stub.ImportedFromPath, stub.Type)

	// Mark as searching
	stub.SearchStatus = "searching"
	if err := app.db.SaveStub(stub); err != nil {
		log.Printf("error updating search status for stub %s: %v", stub.ImportedFromPath, err)
	}

	var title string
	var year int
	var searchErr error

	// Generate query based on file type
	if stub.Type == db.MediaTypeMovie {
		title, year, searchErr = app.llm.GenerateMovieQuery(stub.ImportedFromPath)
	} else if stub.Type == db.MediaTypeTV {
		var seasonNumber int
		title, year, seasonNumber, searchErr = app.llm.GenerateTVQuery(stub.ImportedFromPath)
		stub.SeasonNumber = seasonNumber
	} else {
		log.Printf("unknown media type for stub: %s (type: %s)", stub.ImportedFromPath, stub.Type)
		return nil
	}

	if searchErr != nil {
		log.Printf("error generating query for stub %s: %v", stub.ImportedFromPath, searchErr)
		stub.SearchStatus = "needs_manual"
		if err := app.db.SaveStub(stub); err != nil {
			log.Printf("error updating search status for stub %s: %v", stub.ImportedFromPath, err)
		}
		return nil
	}

	// Update the stub
	stub.Query = title
	if year > 0 {
		stub.Year = strconv.Itoa(year)
	}

	// Execute search immediately using the generated query
	if stub.Type == db.MediaTypeMovie {
		// For movies
		results, err := app.tmdb.Search(title, stub.Year)
		if err != nil {
			log.Printf("error performing search for stub %s: %v", stub.ImportedFromPath, err)
			stub.SearchStatus = "needs_manual"
		} else {
			stub.Results = results
			log.Printf("found %d results for movie stub: %s", len(results), stub.ImportedFromPath)

			if len(results) == 0 && stub.Year != "" {
				// If we got no results with year, try again without year
				log.Printf("no results found with year %s, trying again without year for: %s", stub.Year, stub.ImportedFromPath)
				results, err = app.tmdb.Search(title, "")
				if err != nil {
					log.Printf("error performing year-less search for stub %s: %v", stub.ImportedFromPath, err)
					stub.SearchStatus = "needs_manual"
				} else {
					stub.Results = results
					log.Printf("found %d results without year for movie stub: %s", len(results), stub.ImportedFromPath)

					if len(results) == 0 {
						stub.SearchStatus = "needs_manual"
					} else {
						stub.SearchStatus = "complete"
					}
				}
			} else if len(results) == 0 {
				stub.SearchStatus = "needs_manual"
			} else {
				stub.SearchStatus = "complete"
			}
		}
	} else if stub.Type == db.MediaTypeTV {
		// For TV shows
		results, err := app.tmdb.SearchTV(title, stub.Year)
		if err != nil {
			log.Printf("error performing search for stub %s: %v", stub.ImportedFromPath, err)
			stub.SearchStatus = "needs_manual"
		} else {
			// Convert from tmdb.TVSearchResult to db.TVSearchResult
			tvResults := make([]db.TVSearchResult, len(results))
			for i, r := range results {
				tvResults[i] = db.TVSearchResult{
					ID:           r.ID,
					Name:         r.Name,
					FirstAirDate: r.FirstAirDate,
				}
			}
			stub.TVResults = tvResults
			log.Printf("found %d results for TV stub: %s", len(results), stub.ImportedFromPath)

			if len(results) == 0 && stub.Year != "" {
				// If we got no results with year, try again without year
				log.Printf("no results found with year %s, trying again without year for: %s", stub.Year, stub.ImportedFromPath)
				results, err = app.tmdb.SearchTV(title, "")
				if err != nil {
					log.Printf("error performing year-less search for stub %s: %v", stub.ImportedFromPath, err)
					stub.SearchStatus = "needs_manual"
				} else {
					// Convert from tmdb.TVSearchResult to db.TVSearchResult
					tvResults := make([]db.TVSearchResult, len(results))
					for i, r := range results {
						tvResults[i] = db.TVSearchResult{
							ID:           r.ID,
							Name:         r.Name,
							FirstAirDate: r.FirstAirDate,
						}
					}
					stub.TVResults = tvResults
					log.Printf("found %d results without year for TV stub: %s", len(results), stub.ImportedFromPath)

					if len(results) == 0 {
						stub.SearchStatus = "needs_manual"
					} else {
						stub.SearchStatus = "complete"
					}
				}
			} else if len(results) == 0 {
				stub.SearchStatus = "needs_manual"
			} else {
				stub.SearchStatus = "complete"
			}
		}
	}

	// Save the updated stub with query and search results
	if err := app.db.SaveStub(stub); err != nil {
		return fmt.Errorf("error saving query and search results for stub %s: %w", stub.ImportedFromPath, err)
	}

	log.Printf("query generation and search completed for stub: %s", stub.ImportedFromPath)

	log.Printf("generated query for stub: %s, query: %s, year: %s", stub.ImportedFromPath, stub.Query, stub.Year)
	return nil
}

// RunMovies processes all movie stubs without search queries
func (app *StubQueryGenerator) RunMovies(ctx context.Context) error {
	log.Println("stubquerygenerator (movies) started")
	defer log.Println("stubquerygenerator (movies) done")

	// First, clean up any stubs that have already been imported
	if err := app.CleanupAlreadyImportedStubs(ctx); err != nil {
		log.Printf("Warning: Error cleaning up already imported stubs: %v", err)
		// Continue with processing even if cleanup fails
	}

	stubs, err := app.db.AllStubs()
	if err != nil {
		return err
	}

	for _, stub := range stubs {
		// Only process movie stubs
		if stub.Type != db.MediaTypeMovie {
			continue
		}

		if err := app.processStub(stub); err != nil {
			return err
		}
	}

	return nil
}

// RunTV processes all TV stubs without search queries
func (app *StubQueryGenerator) RunTV(ctx context.Context) error {
	log.Println("stubquerygenerator (TV) started")
	defer log.Println("stubquerygenerator (TV) done")

	// First, clean up any stubs that have already been imported
	if err := app.CleanupAlreadyImportedStubs(ctx); err != nil {
		log.Printf("Warning: Error cleaning up already imported stubs: %v", err)
		// Continue with processing even if cleanup fails
	}

	stubs, err := app.db.AllStubs()
	if err != nil {
		return err
	}

	for _, stub := range stubs {
		// Only process TV stubs
		if stub.Type != db.MediaTypeTV {
			continue
		}

		if err := app.processStub(stub); err != nil {
			return err
		}
	}

	return nil
}

// CleanupAlreadyImportedStubs removes stubs that have already been imported into the library
func (app *StubQueryGenerator) CleanupAlreadyImportedStubs(ctx context.Context) error {
	log.Println("Cleaning up already imported stubs...")

	// Get all stubs
	stubs, err := app.db.AllStubs()
	if err != nil {
		return fmt.Errorf("error getting stubs for cleanup: %w", err)
	}

	// Get all movies and TV shows for lookup
	movies, err := app.db.GetMovies()
	if err != nil {
		return fmt.Errorf("error getting movies for stub cleanup: %w", err)
	}

	tvShows, err := app.db.GetTVShows()
	if err != nil {
		return fmt.Errorf("error getting TV shows for stub cleanup: %w", err)
	}

	// Create lookup maps
	movieIDs := make(map[int64]bool)
	for _, movie := range movies {
		movieIDs[movie.ID] = true
	}

	tvShowIDs := make(map[int64]bool)
	for _, show := range tvShows {
		tvShowIDs[show.ID] = true
	}

	// Track stubs to delete
	var stubsDeleted int

	// Check each stub
	for _, stub := range stubs {
		if stub.Type == db.MediaTypeMovie && len(stub.Results) > 0 {
			// Check if any result ID matches an imported movie
			for _, result := range stub.Results {
				if movieIDs[result.ID] {
					log.Printf("Deleting movie stub %s (TMDB ID %d) as it's already imported", stub.ImportedFromPath, result.ID)
					if err := app.db.DeleteStub(stub); err != nil {
						log.Printf("Error deleting stub %s: %v", stub.ImportedFromPath, err)
					} else {
						// Add to ignore list to prevent re-creation
						if err := app.db.IgnorePath(stub.Type, stub.ImportedFromPath); err != nil {
							log.Printf("Warning: Could not add %s to ignore list: %v", stub.ImportedFromPath, err)
						}
						stubsDeleted++
					}
					break
				}
			}
		} else if stub.Type == db.MediaTypeTV && len(stub.TVResults) > 0 {
			// Check if any result ID matches an imported TV show
			for _, result := range stub.TVResults {
				if tvShowIDs[result.ID] {
					log.Printf("Deleting TV show stub %s (TMDB ID %d) as it's already imported", stub.ImportedFromPath, result.ID)
					if err := app.db.DeleteStub(stub); err != nil {
						log.Printf("Error deleting stub %s: %v", stub.ImportedFromPath, err)
					} else {
						// Add to ignore list to prevent re-creation
						if err := app.db.IgnorePath(stub.Type, stub.ImportedFromPath); err != nil {
							log.Printf("Warning: Could not add %s to ignore list: %v", stub.ImportedFromPath, err)
						}
						stubsDeleted++
					}
					break
				}
			}
		}
	}

	log.Printf("Stub cleanup complete. Deleted %d already imported stubs.", stubsDeleted)
	return nil
}

// Run processes all stubs without search queries (both movies and TV)
func (app *StubQueryGenerator) Run(ctx context.Context) error {
	log.Println("stubquerygenerator (all) started")
	defer log.Println("stubquerygenerator (all) done")

	// First, clean up any stubs that have already been imported
	if err := app.CleanupAlreadyImportedStubs(ctx); err != nil {
		log.Printf("Warning: Error cleaning up already imported stubs: %v", err)
		// Continue with processing even if cleanup fails
	}

	stubs, err := app.db.AllStubs()
	if err != nil {
		return err
	}

	for _, stub := range stubs {
		if err := app.processStub(stub); err != nil {
			return err
		}
	}

	return nil
}
