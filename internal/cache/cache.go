package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joejiang/arxs/internal/model"
)

// Cache provides same-day query caching backed by the filesystem.
type Cache struct {
	dir string
}

// New creates a cache in the given directory.
func New(dir string) *Cache {
	return &Cache{dir: dir}
}

// Get retrieves a cached result for the given query key.
// Returns nil, false on miss or if the cache is from a different day.
func (c *Cache) Get(key string) (*model.SearchResult, bool) {
	if c == nil {
		return nil, false
	}

	path := c.path(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}

	// Check same-day (UTC)
	if entry.Date != today() {
		return nil, false
	}

	return &entry.Result, true
}

// Set stores a result in the cache.
func (c *Cache) Set(key string, result *model.SearchResult) error {
	if c == nil {
		return nil
	}

	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return err
	}

	entry := cacheEntry{
		Date:   today(),
		Result: *result,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return os.WriteFile(c.path(key), data, 0644)
}

type cacheEntry struct {
	Date   string             `json:"date"`
	Result model.SearchResult `json:"result"`
}

func (c *Cache) path(key string) string {
	hash := sha256.Sum256([]byte(key))
	return filepath.Join(c.dir, fmt.Sprintf("%x.json", hash[:8]))
}

func today() string {
	return time.Now().UTC().Format("2006-01-02")
}
