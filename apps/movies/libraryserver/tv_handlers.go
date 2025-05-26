package libraryserver

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"monks.co/apps/movies/config"
	"monks.co/apps/movies/db"
	"monks.co/pkg/serve"
)

// These patterns are copied from tvimporter package to keep consistent behavior
var (
	episodePattern      = regexp.MustCompile(`(?i)S(\d+)E(\d+)`)
	seasonEpPattern     = regexp.MustCompile(`(?i)Season\s*(\d+).*?Episode\s*(\d+)`)
	dotSeasonEpPattern  = regexp.MustCompile(`(\d+)x(\d+)`)
	seasonFolderPattern = regexp.MustCompile(`(?i)Season\s*(\d+)`)
)

// serveTVIndex handles the /tv route to show the TV shows library
func (app *LibraryServer) serveTVIndex(w http.ResponseWriter, req *http.Request) {
	log.Println("serveTVIndex")
	q := req.URL.Query()

	selectedGenres := q["genres"]
	selectedGenresSet := map[string]struct{}{}
	for _, g := range selectedGenres {
		selectedGenresSet[g] = struct{}{}
	}
	allGenresSelected := false
	if len(selectedGenres) == 0 {
		allGenresSelected = true
	}

	query := q.Get("search")

	sortBy := q.Get("sort-by")
	sortDirection := q.Get("sort-direction")
	if sortBy != "name" &&
		sortBy != "date" &&
		sortBy != "lastDate" &&
		sortBy != "importDate" {
		sortBy = "name"
	}
	if sortDirection != "asc" && sortDirection != "desc" {
		if sortBy == "date" || sortBy == "lastDate" || sortBy == "importDate" {
			sortDirection = "desc"
		} else {
			sortDirection = "asc"
		}
	}

	shows, err := app.db.AllTVShows()
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	var data TVShowsData
	data.Query = query
	data.SortBy = sortBy
	data.SortDirection = sortDirection

	// Loop through shows, applying filters and collecting genres
	allGenresSet := map[string]struct{}{}
	for _, show := range shows {
		// Filter by genre if specified
		genreMatch := false
		for _, g := range show.Genres {
			if len(g) == 0 {
				continue
			}
			allGenresSet[g] = struct{}{}
			for _, gg := range selectedGenres {
				if gg == g {
					genreMatch = true
				}
			}
		}
		if !genreMatch && !allGenresSelected {
			continue
		}

		// Filter by search query if specified
		if query != "" {
			searchString := strings.ToLower(show.Name + " " + strings.Join(show.Genres, " "))
			if !strings.Contains(searchString, strings.ToLower(query)) {
				continue
			}
		}

		data.Shows = append(data.Shows, show)
	}

	// Make genre options
	for genre := range allGenresSet {
		_, isSelected := selectedGenresSet[genre]
		data.Genres = append(data.Genres, Genre{
			Name:       genre,
			IsSelected: !allGenresSelected && isSelected,
		})
	}
	sort.Slice(data.Genres, func(a, b int) bool {
		return data.Genres[a].Name < data.Genres[b].Name
	})

	// Sort the shows
	sort.Slice(data.Shows, func(a, b int) bool {
		switch sortBy {
		case "date":
			if sortDirection == "desc" {
				return data.Shows[a].FirstAirDate > data.Shows[b].FirstAirDate
			}
			return data.Shows[a].FirstAirDate < data.Shows[b].FirstAirDate
		case "lastDate":
			if sortDirection == "desc" {
				return data.Shows[a].LastAirDate > data.Shows[b].LastAirDate
			}
			return data.Shows[a].LastAirDate < data.Shows[b].LastAirDate
		case "importDate":
			if data.Shows[a].ImportedAt == data.Shows[b].ImportedAt {
				return false
			}
			if sortDirection == "desc" {
				return data.Shows[a].ImportedAt > data.Shows[b].ImportedAt
			}
			return data.Shows[a].ImportedAt < data.Shows[b].ImportedAt
		case "name":
			fallthrough
		default:
			if sortDirection == "desc" {
				return data.Shows[a].Name > data.Shows[b].Name
			}
			return data.Shows[a].Name < data.Shows[b].Name
		}
	})

	// Render the template using templ
	if err := TVShows(&data).Render(req.Context(), w); err != nil {
		log.Println(err)
	}
}

// serveTVShow handles the /tv/show route to show details for a specific TV show
func (app *LibraryServer) serveTVShow(w http.ResponseWriter, req *http.Request) {
	log.Println("serveTVShow")

	idStr := req.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		serve.Errorf(w, req, http.StatusBadRequest, "Invalid show ID: %v", err)
		return
	}

	show, err := app.db.GetTVShow(id)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	// Get seasons for this show
	seasons, err := app.db.GetTVShowSeasons(id)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	// Sort seasons by number
	sort.Slice(seasons, func(i, j int) bool {
		return seasons[i].SeasonNumber < seasons[j].SeasonNumber
	})

	data := TVShowData{
		Show:    show,
		Seasons: seasons,
	}

	// Render the template using templ
	if err := TVShowDetails(&data).Render(req.Context(), w); err != nil {
		log.Println(err)
	}
}

// serveTVSeason handles the /tv/season route to show details for a specific season
func (app *LibraryServer) serveTVSeason(w http.ResponseWriter, req *http.Request) {
	log.Println("serveTVSeason")

	showIDStr := req.URL.Query().Get("show_id")
	showID, err := strconv.ParseInt(showIDStr, 10, 64)
	if err != nil {
		serve.Errorf(w, req, http.StatusBadRequest, "Invalid show ID: %v", err)
		return
	}

	seasonStr := req.URL.Query().Get("season")
	seasonNum, err := strconv.Atoi(seasonStr)
	if err != nil {
		serve.Errorf(w, req, http.StatusBadRequest, "Invalid season number: %v", err)
		return
	}

	show, err := app.db.GetTVShow(showID)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	season, err := app.db.GetTVSeason(showID, seasonNum)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	// Get episodes for this season
	episodes, err := app.db.GetTVSeasonEpisodes(showID, seasonNum)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	// Sort episodes by number
	sort.Slice(episodes, func(i, j int) bool {
		return episodes[i].EpisodeNumber < episodes[j].EpisodeNumber
	})

	data := TVSeasonData{
		Show:     show,
		Season:   season,
		Episodes: episodes,
	}

	// Render the template using templ
	if err := TVSeasonDetails(&data).Render(req.Context(), w); err != nil {
		log.Println(err)
	}
}

// serveTVPoster handles the /tv/poster route to serve TV show posters
func (app *LibraryServer) serveTVPoster(w http.ResponseWriter, req *http.Request) {
	idStr := req.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		serve.Errorf(w, req, http.StatusBadRequest, "error parsing ID: %s", err)
		return
	}

	show, err := app.db.GetTVShow(id)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	w.Header().Set("Cache-control", "public, max-age=604800, immutable")
	http.ServeFile(w, req, show.PosterPath)
}

// serveTVSeasonPoster handles the /tv/season/poster route to serve TV season posters
func (app *LibraryServer) serveTVSeasonPoster(w http.ResponseWriter, req *http.Request) {
	showIDStr := req.URL.Query().Get("show_id")
	showID, err := strconv.ParseInt(showIDStr, 10, 64)
	if err != nil {
		serve.Errorf(w, req, http.StatusBadRequest, "Invalid show ID: %v", err)
		return
	}

	seasonStr := req.URL.Query().Get("season")
	seasonNum, err := strconv.Atoi(seasonStr)
	if err != nil {
		serve.Errorf(w, req, http.StatusBadRequest, "Invalid season number: %v", err)
		return
	}

	season, err := app.db.GetTVSeason(showID, seasonNum)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	w.Header().Set("Cache-control", "public, max-age=604800, immutable")
	http.ServeFile(w, req, season.PosterPath)
}

// serveTVPlayButton handles the /tv/play route to play an episode
func (app *LibraryServer) serveTVPlayButton(w http.ResponseWriter, req *http.Request) {
	log.Println("serveTVPlayButton")

	if req.Method != "POST" {
		serve.Errorf(w, req, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	req.ParseForm()

	showIDStr := req.FormValue("show_id")
	showID, err := strconv.ParseInt(showIDStr, 10, 64)
	if err != nil {
		serve.Errorf(w, req, http.StatusBadRequest, "Invalid show ID: %v", err)
		return
	}

	seasonStr := req.FormValue("season")
	seasonNum, err := strconv.Atoi(seasonStr)
	if err != nil {
		serve.Errorf(w, req, http.StatusBadRequest, "Invalid season number: %v", err)
		return
	}

	episodeStr := req.FormValue("episode")
	episodeNum, err := strconv.Atoi(episodeStr)
	if err != nil {
		serve.Errorf(w, req, http.StatusBadRequest, "Invalid episode number: %v", err)
		return
	}

	episode, err := app.db.GetTVEpisode(showID, seasonNum, episodeNum)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	// Play the episode using VLC on the remote machine
	for _, cmd := range []*exec.Cmd{
		exec.Command("ssh", "lugh", fmt.Sprintf("open -a VLC.app 'sftp://ajm@thr.ss.cx%s/%s'", config.TVLibraryDir, episode.LibraryPath)),
		exec.Command("ssh", "lugh", `osascript -e 'tell application "VLC" to activate' -e 'tell application "System Events" to keystroke "f" using {command down, control down}'`),
	} {
		cmd := cmd
		if err := cmd.Start(); err != nil {
			serve.InternalServerError(w, req, err)
			return
		}
		go func() {
			if err := cmd.Wait(); err != nil {
				log.Println("start on lugh error:", err)
			}
		}()
	}

	w.WriteHeader(200)
	w.Write([]byte("ok"))
}

// serveTVSearch handles the /tv/search route to search for TV shows
func (app *LibraryServer) serveTVSearch(w http.ResponseWriter, req *http.Request) {
	log.Println("serveTVSearch")

	if req.Method != "POST" {
		serve.Errorf(w, req, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	req.ParseForm()

	path := req.FormValue("path")
	query := req.FormValue("query")
	year := req.FormValue("year")

	stub, err := app.db.GetStub(path)
	if err != nil {
		serve.Errorf(w, req, http.StatusNotFound, "no such stub: %s", err)
		return
	}

	log.Println("tv search", query, year)
	results, err := app.tmdb.SearchTV(query, year)
	if err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	// We need to convert TMDB TV search results to the format expected by the stub
	searchResults := make([]db.TVSearchResult, len(results))
	for i, result := range results {
		searchResults[i] = db.TVSearchResult{
			ID:           result.ID,
			Name:         result.Name,
			FirstAirDate: result.FirstAirDate,
		}
	}

	stub.TVResults = searchResults
	log.Printf("%d TV results", len(results))
	if err := app.db.SaveStub(stub); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	// Redirect to TV tab
	http.Redirect(w, req, "/movies/import/?tab=tv", http.StatusSeeOther)
}

// serveTVIdentify handles the /tv/identify route to identify a TV show
func (app *LibraryServer) serveTVIdentify(w http.ResponseWriter, req *http.Request) {
	log.Println("serveTVIdentify")

	if req.Method != "POST" {
		serve.Errorf(w, req, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	req.ParseForm()

	path := req.FormValue("path")
	if path == "" {
		serve.Errorf(w, req, http.StatusBadRequest, "no path given")
		return
	}

	stub, err := app.db.GetStub(path)
	if err != nil {
		serve.Errorf(w, req, http.StatusNotFound, "no such stub: %s", err)
		return
	}

	idStr := req.FormValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		serve.Errorf(w, req, http.StatusBadRequest, "error parsing ID: %s", err)
		return
	}

	// Check if a season number was provided or use the one from the stub
	seasonStr := req.FormValue("season_number")
	seasonNumber := stub.SeasonNumber
	if seasonStr != "" {
		parsedSeason, err := strconv.Atoi(seasonStr)
		if err == nil && parsedSeason > 0 {
			seasonNumber = parsedSeason
		}
	}

	// Get the show information from TMDB
	tvShow, err := app.tmdb.GetTV(id)
	if err != nil {
		serve.InternalServerErrorf(w, req, "error getting TV show metadata from tmdb %w", err)
		return
	}

	if err := app.db.Transaction(func(tx *db.DB) error {
		// Check if the show already exists
		show, err := tx.GetTVShow(tvShow.ID)
		if err != nil && err.Error() != "record not found" {
			return fmt.Errorf("error checking if TV show exists: %w", err)
		}

		if show == nil {
			// Create the show
			show, err = tx.CreateTVShow(tvShow)
			if err != nil {
				return fmt.Errorf("error creating TV show: %w", err)
			}
		}

		// Process all episode files in the stub, but only for the specified season
		for _, episodePath := range stub.EpisodeFiles {
			// Extract the season and episode numbers from the file path
			fileSeasonNum, episodeNum, err := extractSeasonEpisodeFromPath(episodePath)
			if err != nil {
				log.Printf("Warning: Couldn't extract season/episode from %s: %v", episodePath, err)
				continue
			}

			// Only process files that match the identified season number
			if fileSeasonNum != seasonNumber {
				log.Printf("Skipping episode from different season: S%02d vs. selected S%02d - %s",
					fileSeasonNum, seasonNumber, episodePath)
				continue
			}

			// Check if the season already exists
			season, err := tx.GetTVSeason(tvShow.ID, seasonNumber)
			if err != nil && err.Error() != "record not found" {
				return fmt.Errorf("error checking if season exists: %w", err)
			}

			if season == nil {
				// Get the season details from TMDB
				seasonData, _, err := app.tmdb.GetSeason(tvShow.ID, seasonNumber)
				if err != nil {
					log.Printf("Warning: Error getting season data for S%02d: %v", seasonNumber, err)
					continue
				}

				// Create the season
				season, err = tx.CreateTVSeason(tvShow.ID, seasonData)
				if err != nil {
					log.Printf("Warning: Error creating season S%02d: %v", seasonNumber, err)
					continue
				}
			}

			// Check if the episode already exists
			episode, err := tx.GetTVEpisode(tvShow.ID, seasonNumber, episodeNum)
			if err != nil && err.Error() != "record not found" {
				log.Printf("Warning: Error checking if episode S%02dE%02d exists: %v", seasonNumber, episodeNum, err)
				continue
			}

			if episode == nil {
				// Get the episode details from TMDB
				episodeData, err := app.tmdb.GetEpisode(tvShow.ID, seasonNumber, episodeNum)
				if err != nil {
					log.Printf("Warning: Error getting episode data for S%02dE%02d: %v", seasonNumber, episodeNum, err)
					continue
				}

				// Create the full path by directly using the episodePath which retains original case
				fullPath := filepath.Join(config.TVImportDir, episodePath)
				episode, err = tx.CreateTVEpisode(tvShow.ID, episodeData, fullPath)
				if err != nil {
					log.Printf("Warning: Error creating episode S%02dE%02d: %v", seasonNumber, episodeNum, err)
					continue
				}
			} else {
				// Episode exists, update the path
				// Create the full path by directly using the episodePath which retains original case
				fullPath := filepath.Join(config.TVImportDir, episodePath)
				if err := tx.UpdateTVEpisodePath(episode, fullPath); err != nil {
					log.Printf("Warning: Error updating episode path for S%02dE%02d: %v", seasonNumber, episodeNum, err)
					continue
				}
			}
		}

		// Delete the stub
		if err := tx.DeleteStub(stub); err != nil {
			return fmt.Errorf("error deleting stub: %w", err)
		}

		log.Printf("Successfully processed TV show: %s with %d episodes", tvShow.Name, len(stub.EpisodeFiles))

		return nil
	}); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	// Redirect to TV tab
	http.Redirect(w, req, "/movies/import/?tab=tv", http.StatusSeeOther)
}

// We don't need the serveTVIdentifyAll function anymore since we're now handling shows at the show level
// This function can be removed, but we're keeping the signature as a no-op for compatibility
func (app *LibraryServer) serveTVIdentifyAll(w http.ResponseWriter, req *http.Request) {
	log.Println("serveTVIdentifyAll - this endpoint is deprecated, using serveTVIdentify instead")
	http.Redirect(w, req, "/movies/import/?tab=tv", http.StatusSeeOther)
}

// serveTVIgnoreShow ignores a TV show
func (app *LibraryServer) serveTVIgnoreShow(w http.ResponseWriter, req *http.Request) {
	log.Println("serveTVIgnoreShow")

	if req.Method != "POST" {
		serve.Errorf(w, req, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return
	}

	req.ParseForm()

	// Get show path (directory)
	path := req.FormValue("path")
	if path == "" {
		serve.Errorf(w, req, http.StatusBadRequest, "no path given")
		return
	}

	// Get the stub
	stub, err := app.db.GetStub(path)
	if err != nil {
		serve.Errorf(w, req, http.StatusNotFound, "no such stub: %s", err)
		return
	}

	if err := app.db.Transaction(func(tx *db.DB) error {
		// Ignore the path
		if err := tx.IgnorePath(db.MediaTypeTV, stub.ImportedFromPath); err != nil {
			return fmt.Errorf("error ignoring path '%s': %w", stub.ImportedFromPath, err)
		}

		// Delete the stub
		if err := tx.DeleteStub(stub); err != nil {
			return fmt.Errorf("error deleting stub for '%s': %w", stub.ImportedFromPath, err)
		}

		return nil
	}); err != nil {
		serve.InternalServerError(w, req, err)
		return
	}

	log.Printf("Successfully ignored TV show: %s", path)

	// Redirect to TV tab
	http.Redirect(w, req, "/movies/import/?tab=tv", http.StatusSeeOther)
}

// Helper function to extract season and episode numbers from a file path
func extractSeasonEpisodeFromPath(path string) (int, int, error) {
	// Try to use the same function from tvimporter package (via a helper)
	season, episode, err := parseEpisodeInfoFromPath(path)
	if err == nil {
		return season, episode, nil
	}

	// If that fails, try our fallback approaches

	// Extract the filename from the path
	dir, filename := filepath.Split(path)

	// Try all known episode patterns
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

	// Also check for season directories in the path
	parts := strings.Split(path, "/")
	var seasonNum int

	for _, part := range parts {
		// Look for "Season X" directory pattern
		if seasonMatch := seasonFolderPattern.FindStringSubmatch(part); seasonMatch != nil {
			seasonNum, _ = strconv.Atoi(seasonMatch[1])

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
		if seasonMatch := seasonFolderPattern.FindStringSubmatch(part); seasonMatch != nil {
			seasonNum, _ := strconv.Atoi(seasonMatch[1])

			// Try to find episode number in filename
			episodeMatch := regexp.MustCompile(`(\d+)`).FindStringSubmatch(filename)
			if episodeMatch != nil {
				episodeNum, _ := strconv.Atoi(episodeMatch[1])
				return seasonNum, episodeNum, nil
			}
		}
	}

	// If all else fails, assume season 1 if we can at least find an episode number
	episodeMatch := regexp.MustCompile(`(\d+)`).FindStringSubmatch(filename)
	if episodeMatch != nil {
		episodeNum, _ := strconv.Atoi(episodeMatch[1])
		return 1, episodeNum, nil
	}

	return 0, 0, fmt.Errorf("could not extract season and episode from path: %s", path)
}

// Helper function that mimics ParseEpisodeInfo from tvimporter
func parseEpisodeInfoFromPath(path string) (int, int, error) {
	filename := filepath.Base(path)

	// Try various patterns to extract season and episode numbers
	if match := episodePattern.FindStringSubmatch(filename); match != nil {
		seasonNum := match[1]
		episodeNum := match[2]
		season, _ := strconv.Atoi(seasonNum)
		episode, _ := strconv.Atoi(episodeNum)
		return season, episode, nil
	}

	if match := seasonEpPattern.FindStringSubmatch(filename); match != nil {
		seasonNum := match[1]
		episodeNum := match[2]
		season, _ := strconv.Atoi(seasonNum)
		episode, _ := strconv.Atoi(episodeNum)
		return season, episode, nil
	}

	if match := dotSeasonEpPattern.FindStringSubmatch(filename); match != nil {
		seasonNum := match[1]
		episodeNum := match[2]
		season, _ := strconv.Atoi(seasonNum)
		episode, _ := strconv.Atoi(episodeNum)
		return season, episode, nil
	}

	return 0, 0, fmt.Errorf("could not parse season and episode from path: %s", path)
}
