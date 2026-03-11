package stubquerygenerator

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"monks.co/apps/movies/db"
	"monks.co/pkg/llm"
	"monks.co/pkg/tmdb"
)

// LLMClient interface for LLM operations
type LLMClient interface {
	GenerateMovieQuery(filepath string) (string, int, error)
	GenerateTVQuery(filepath string) (string, int, int, error)
}

// movieLLM uses the pkg/llm streaming API with tool calling for structured output.
type movieLLM struct {
	model llm.Model
}

// generate sends a prompt to the LLM with a tool definition and returns the tool call arguments.
func (m *movieLLM) generate(prompt string, tools []llm.Tool) (map[string]any, error) {
	maxTokens := 200
	req := llm.Request{
		Messages: []llm.Message{
			llm.UserMessage{
				Role:      "user",
				Content:   []llm.ContentBlock{llm.TextContent{Type: "text", Text: prompt}},
				Timestamp: time.Now(),
			},
		},
		Tools: tools,
	}
	opts := llm.StreamOptions{
		MaxTokens:      &maxTokens,
		CacheRetention: llm.CacheNone,
	}

	handle, err := llm.Stream(context.Background(), m.model, req, opts)
	if err != nil {
		return nil, fmt.Errorf("llm stream: %w", err)
	}

	msg, err := handle.Wait()
	if err != nil {
		return nil, fmt.Errorf("llm wait: %w", err)
	}

	for _, block := range msg.Content {
		if tc, ok := block.(llm.ToolCall); ok {
			return tc.Arguments, nil
		}
	}
	return nil, fmt.Errorf("no tool call in LLM response")
}

type movieQueryParams struct {
	TitleQuery string `json:"title_query" jsonschema:"description=The movie title to search for"`
	YearQuery  int    `json:"year_query" jsonschema:"description=The release year (0 if not in filename)"`
}

type tvQueryParams struct {
	TitleQuery   string `json:"title_query" jsonschema:"description=The TV show title to search for"`
	YearQuery    int    `json:"year_query" jsonschema:"description=The first air year (0 if not in filename)"`
	SeasonNumber int    `json:"season_number" jsonschema:"description=The season number"`
}

// GenerateMovieQuery generates a movie search query from a filepath
func (m *movieLLM) GenerateMovieQuery(filepath string) (string, int, error) {
	prompt := fmt.Sprintf("We have the following filepath, which we think is a movie file. We'd like to look it up in TMDB, and we need to construct a search query for that lookup. IMPORTANT: For the year_query, ONLY return a year if it is explicitly present in the filename. DO NOT guess or use your prior knowledge about when movies were released - if the year isn't in the filename, set year_query to 0. Here is the filepath: %q. Call the extract_movie_query tool with the results.", filepath)

	result, err := m.generate(prompt, []llm.Tool{{
		Name:        "extract_movie_query",
		Description: "Extract a movie title and year from a filepath for TMDB search",
		Parameters:  movieQueryParams{},
	}})
	if err != nil {
		return "", 0, err
	}

	titleQuery, ok := result["title_query"].(string)
	if !ok {
		return "", 0, fmt.Errorf("invalid title_query in LLM response: %v", result)
	}

	yearQuery, _ := result["year_query"].(float64)

	return titleQuery, int(yearQuery), nil
}

// GenerateTVQuery generates a TV show search query from a filepath
func (m *movieLLM) GenerateTVQuery(filepath string) (string, int, int, error) {
	prompt := fmt.Sprintf("We have the following filepath, which we think is a TV series folder. We'd like to look it up in TMDB, and we need to construct a search query for that lookup. Extract the TV show name and determine which season this folder likely represents. IMPORTANT: For the year_query, ONLY return a year if it is explicitly present in the filename. DO NOT guess or use your prior knowledge about when the TV series first aired - if the year isn't in the filename, set year_query to 0. Here is the filepath: %q. Call the extract_tv_query tool with the results.", filepath)

	result, err := m.generate(prompt, []llm.Tool{{
		Name:        "extract_tv_query",
		Description: "Extract a TV show title, year, and season number from a filepath for TMDB search",
		Parameters:  tvQueryParams{},
	}})
	if err != nil {
		return "", 0, 0, err
	}

	titleQuery, ok := result["title_query"].(string)
	if !ok {
		return "", 0, 0, fmt.Errorf("invalid title_query in LLM response: %v", result)
	}

	yearQuery, _ := result["year_query"].(float64)
	seasonNumber, ok := result["season_number"].(float64)
	if !ok {
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
	IgnorePath(mediaType db.MediaType, path string) error
}

// StubQueryGenerator generates search queries for stubs without existing queries
type StubQueryGenerator struct {
	llm  LLMClient
	tmdb TMDBClient
	db   DBClient
}

// New creates a new StubQueryGenerator
func New(model llm.Model, tmdb *tmdb.Client, db *db.DB) *StubQueryGenerator {
	return &StubQueryGenerator{
		llm:  &movieLLM{model: model},
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

// Run processes all stubs without search queries (both movies and TV)
func (app *StubQueryGenerator) Run(ctx context.Context) error {
	log.Println("stubquerygenerator (all) started")
	defer log.Println("stubquerygenerator (all) done")

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
