package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Storage represents the data storage
type Storage struct {
	sync.RWMutex
	dataFile string
	data     map[string]string
}

// NewStorage creates a new storage instance
func NewStorage(dataFile string) (*Storage, error) {
	s := &Storage{
		dataFile: dataFile,
		data:     make(map[string]string),
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(dataFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	// Try to load existing data
	if _, err := os.Stat(dataFile); err == nil {
		data, err := os.ReadFile(dataFile)
		if err != nil {
			return nil, err
		}

		if len(data) > 0 {
			if err := json.Unmarshal(data, &s.data); err != nil {
				return nil, err
			}
		}
	}

	return s, nil
}

// Save persists the data to disk using a safe write pattern
func (s *Storage) Save() error {
	s.RLock()
	data, err := json.Marshal(s.data)
	s.RUnlock()


	if err != nil {
		return err
	}

	// Create a temporary file
	tempFile := s.dataFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}

	// Atomically replace the old file with the new one
	return os.Rename(tempFile, s.dataFile)
}

// Get retrieves a value from storage
func (s *Storage) Get(key string) string {
	s.RLock()
	defer s.RUnlock()
	return s.data[key]
}

// Set stores a value in storage
func (s *Storage) Set(key, value string) {
	s.Lock()
	defer s.Unlock()
	s.data[key] = value
}
