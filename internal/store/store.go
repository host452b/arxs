package store

import (
	"encoding/json"
	"os"

	"github.com/joejiang/arxs/internal/model"
)

// WriteResults writes search results to a JSON file.
func WriteResults(path string, result *model.SearchResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ReadResults reads search results from a JSON file.
func ReadResults(path string) (*model.SearchResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result model.SearchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
