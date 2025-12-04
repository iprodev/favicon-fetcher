// Package cache provides a three-tier caching system for favicon data.
// It supports original image caching, resized image caching, and fallback images.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Manager handles caching of favicon data across multiple tiers.
// It provides thread-safe operations for reading, writing, and maintaining cache entries.
type Manager struct {
	CacheDir string
	TTL      time.Duration
}

// OrigMeta contains metadata about cached original images.
// It stores ETags and Last-Modified headers for conditional HTTP requests.
type OrigMeta struct {
	URL          string    `json:"url"`
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"last_modified,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// New creates a new cache Manager with the specified directory and TTL.
// The cache directory will be created if it doesn't exist.
func New(cacheDir string, ttl time.Duration) *Manager {
	return &Manager{
		CacheDir: cacheDir,
		TTL:      ttl,
	}
}

// EnsureDirs creates all required cache directories if they don't exist.
// Returns an error if directory creation fails.
func (m *Manager) EnsureDirs() error {
	for _, p := range []string{
		m.OrigCacheDir(),
		m.ResizedCacheDir(),
		m.FallbackCacheDir(),
	} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// OrigCacheDir returns the path to the original images cache directory.
func (m *Manager) OrigCacheDir() string {
	return filepath.Join(m.CacheDir, "orig")
}

// ResizedCacheDir returns the path to the resized images cache directory.
func (m *Manager) ResizedCacheDir() string {
	return filepath.Join(m.CacheDir, "resized")
}

// FallbackCacheDir returns the path to the fallback images cache directory.
func (m *Manager) FallbackCacheDir() string {
	return filepath.Join(m.CacheDir, "fallback")
}

// ReadOrigFromCache attempts to read an original image from cache.
// Returns the image data and true if found and not expired, nil and false otherwise.
// Note: There's a small race window where janitor might delete the file between
// stat and read, but this is handled gracefully by returning cache miss.
func (m *Manager) ReadOrigFromCache(iconURL string) ([]byte, bool) {
	p := filepath.Join(m.OrigCacheDir(), hash("orig|"+iconURL))
	info, err := os.Stat(p)
	if err != nil {
		return nil, false
	}
	if time.Since(info.ModTime()) > m.TTL {
		return nil, false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		// File was deleted between stat and read (race with janitor)
		return nil, false
	}
	return b, true
}

// WriteOrigToCache writes an original image to cache.
// The write is atomic to prevent partial writes on failure.
func (m *Manager) WriteOrigToCache(iconURL string, b []byte) error {
	return atomicWriteFile(filepath.Join(m.OrigCacheDir(), hash("orig|"+iconURL)), b)
}

// TouchOrigCache updates the modification time of a cached original image.
// This is used to refresh TTL on cache hits with 304 Not Modified responses.
func (m *Manager) TouchOrigCache(iconURL string) error {
	p := filepath.Join(m.OrigCacheDir(), hash("orig|"+iconURL))
	now := time.Now()
	return os.Chtimes(p, now, now)
}

// ReadOrigMeta reads metadata for a cached original image.
// Returns the metadata and true if found, empty metadata and false otherwise.
func (m *Manager) ReadOrigMeta(iconURL string) (OrigMeta, bool) {
	p := filepath.Join(m.OrigCacheDir(), hash("orig|"+iconURL)+".meta")
	data, err := os.ReadFile(p)
	if err != nil {
		return OrigMeta{}, false
	}
	var meta OrigMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return OrigMeta{}, false
	}
	return meta, true
}

// WriteOrigMeta writes metadata for a cached original image.
// The write is atomic to prevent corruption.
func (m *Manager) WriteOrigMeta(iconURL string, meta OrigMeta) error {
	p := filepath.Join(m.OrigCacheDir(), hash("orig|"+iconURL)+".meta")
	data, _ := json.MarshalIndent(meta, "", "  ")
	return atomicWriteFile(p, data)
}

// ResizedCachePath returns the cache path for a resized image.
// The path includes the size and format in the hash to prevent collisions.
func (m *Manager) ResizedCachePath(iconURL string, size int, format string) string {
	ext := "." + format
	key := hash("res|" + iconURL + "|" + strconv.Itoa(size) + "|" + format)
	return filepath.Join(m.ResizedCacheDir(), key+ext)
}

// WriteResizedToCache writes a resized image to cache.
// The write is atomic to prevent partial writes on failure.
func (m *Manager) WriteResizedToCache(iconURL string, size int, format string, b []byte) error {
	return atomicWriteFile(m.ResizedCachePath(iconURL, size, format), b)
}

// ReadResizedFromCacheWithMod attempts to read a resized image from cache.
// Returns the image data, true if found and not expired, and the modification time.
func (m *Manager) ReadResizedFromCacheWithMod(iconURL string, size int, format string) ([]byte, bool, time.Time) {
	p := m.ResizedCachePath(iconURL, size, format)
	info, err := os.Stat(p)
	if err != nil {
		return nil, false, time.Time{}
	}
	if time.Since(info.ModTime()) > m.TTL {
		return nil, false, time.Time{}
	}
	b, err := os.ReadFile(p)
	if err != nil {
		// File was deleted between stat and read (race with janitor)
		return nil, false, time.Time{}
	}
	return b, true, info.ModTime()
}

func atomicWriteFile(p string, data []byte) error {
	dir := filepath.Dir(p)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	// Ensure cleanup on failure
	var success bool
	defer func() {
		if !success {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, p); err != nil {
		return err
	}
	success = true
	return nil
}

func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
