package main

import (
	"monks.co/pkg/database"
)

type model struct {
	*database.DB
}

func NewModel() (*model, error) {
	db, err := database.Open("/data/tank/mirror/reddit/.reddit.db")
	if err != nil {
		return nil, err
	}
	return &model{db}, nil
}

func (m *model) getPosts(limit, offset int, subreddit, author string) ([]*Post, error) {
	posts := []*Post{}
	query := m.DB.Table("posts").
		Where("status = ? AND created IS NOT NULL", "archived")

	// Apply filters if provided
	if subreddit != "" {
		query = query.Where("subreddit = ?", subreddit)
	}
	if author != "" {
		query = query.Where("author = ?", author)
	}

	if err := query.
		Order("created DESC").
		Offset(offset - 1). // Convert from 1-based to 0-based
		Limit(limit).
		Find(&posts).
		Error; err != nil {
		return nil, err
	}
	return posts, nil
}

func (m *model) getPostsByCreated(subreddit, author string) ([]*Post, error) {
	posts := []*Post{}
	query := m.DB.Table("posts").
		Where("status = ? AND created IS NOT NULL", "archived")

	// Apply filters if provided
	if subreddit != "" {
		query = query.Where("subreddit = ?", subreddit)
	}
	if author != "" {
		query = query.Where("author = ?", author)
	}

	if err := query.
		Order("created DESC").
		Find(&posts).
		Error; err != nil {
		return nil, err
	}
	return posts, nil
}

// getPostCount returns the total number of posts matching the given filters
func (m *model) getPostCount(subreddit, author string) (int64, error) {
	var count int64
	query := m.DB.Table("posts").
		Where("status = ? AND created IS NOT NULL", "archived")

	// Apply filters if provided
	if subreddit != "" {
		query = query.Where("subreddit = ?", subreddit)
	}
	if author != "" {
		query = query.Where("author = ?", author)
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// SubredditCount represents a subreddit with its post count
type SubredditCount struct {
	Subreddit string
	Count     int64
}

// GetSubredditCounts returns all subreddits with their post counts
func (m *model) getSubredditCounts() ([]SubredditCount, error) {
	var results []SubredditCount
	query := `
		SELECT subreddit, COUNT(*) as count 
		FROM posts 
		WHERE status = 'archived' AND created IS NOT NULL
		GROUP BY subreddit 
		ORDER BY count DESC
	`

	rows, err := m.DB.Raw(query).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var sc SubredditCount
		if err := rows.Scan(&sc.Subreddit, &sc.Count); err != nil {
			return nil, err
		}
		results = append(results, sc)
	}

	return results, nil
}

// AuthorCount represents an author with their post count
type AuthorCount struct {
	Author string
	Count  int64
}

// GetAuthorCounts returns all authors with their post counts
func (m *model) getAuthorCounts() ([]AuthorCount, error) {
	var results []AuthorCount
	query := `
		SELECT author, COUNT(*) as count 
		FROM posts 
		WHERE status = 'archived' AND created IS NOT NULL
		GROUP BY author 
		ORDER BY count DESC
	`

	rows, err := m.DB.Raw(query).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ac AuthorCount
		if err := rows.Scan(&ac.Author, &ac.Count); err != nil {
			return nil, err
		}
		results = append(results, ac)
	}

	return results, nil
}
