# Stub Query Generator

The Stub Query Generator is a worker service that automatically generates search queries for media stubs without existing queries. It uses an LLM to analyze filenames and folder paths to create optimal search queries for TMDB lookups.

## How it works

1. The service scans all existing stubs in the database that don't already have search queries or search results
2. For each qualifying stub, it:
   - Passes the file path to an LLM to extract the likely title and year
   - Updates the stub with the generated query and year
   - Searches TMDB using the generated query
   - Saves the search results to the stub

The operator still needs to select/confirm the correct search result in the library interface, but they no longer need to manually construct search queries.

## Implementation

The StubQueryGenerator uses:
- The `llm` package to generate search queries from file paths
- The `tmdb` package to search for movies and TV shows
- The `db` package to read/write stubs

## TV Show Handling

For TV shows, stubs are created at the show directory level, not for individual episodes. This simplifies the import workflow:

1. The TVImporter identifies TV show directories containing valid episode files
2. It creates a single stub for each show directory
3. The StubQueryGenerator then processes these show-level stubs to generate search queries
4. The operator selects the correct TV show match from the search results

## Examples

### Movie Example

Input filepath: 
```
"The.Descendants.2011.720p.BluRay.DD5.1.x264-EbP/The.Descendants.2011.720p.BluRay.DD5.1.x264-EbP.mkv"
```

Generated search query:
```
{
  "title_query": "The Descendants",
  "year_query": 2011
}
```

### TV Show Example

Input filepath:
```
"Breaking.Bad.S01.1080p.BluRay.x264-HD"
```

Generated search query:
```
{
  "title_query": "Breaking Bad",
  "year_query": 2008
}
```

The search queries are used to search TMDB, and the results are stored with the stubs.