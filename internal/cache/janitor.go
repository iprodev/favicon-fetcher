package cache

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"faviconsvc/pkg/logger"
)

type fileEntry struct {
	path  string
	size  int64
	mtime time.Time
}

func RunJanitor(ctx context.Context, interval time.Duration, root string, ttl time.Duration, maxSize int64) {
	t := time.NewTicker(interval)
	defer t.Stop()

	// Initial delay
	select {
	case <-time.After(2 * time.Second):
	case <-ctx.Done():
		return
	}

	logger.Info("Janitor started: interval=%v, ttl=%v, maxSize=%d", interval, ttl, maxSize)
	purgeOnce(root, ttl, maxSize)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Janitor stopped")
			return
		case <-t.C:
			purgeOnce(root, ttl, maxSize)
		}
	}
}

func purgeOnce(root string, ttl time.Duration, maxSize int64) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Janitor panic: %v", r)
		}
	}()

	expireBefore := time.Now().Add(-ttl)
	expiredCount := 0
	orphanMetaCount := 0
	tempFileCount := 0

	// Collect all cache files
	var dataFiles []string
	var tempFiles []string
	metaFiles := make(map[string]string) // base path -> meta path

	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		if !isCacheFile(p) {
			return nil
		}

		base := filepath.Base(p)
		
		// Detect leftover temp files from atomic writes
		if strings.HasPrefix(base, ".tmp-") {
			tempFiles = append(tempFiles, p)
			return nil
		}

		if strings.HasSuffix(p, ".meta") {
			baseWithoutMeta := strings.TrimSuffix(p, ".meta")
			metaFiles[baseWithoutMeta] = p
		} else {
			dataFiles = append(dataFiles, p)
		}
		return nil
	})

	// Create set of existing data files for quick lookup
	dataFileSet := make(map[string]struct{}, len(dataFiles))
	for _, f := range dataFiles {
		dataFileSet[f] = struct{}{}
	}

	// Purge expired data files and their meta files
	for _, p := range dataFiles {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}

		if info.ModTime().Before(expireBefore) {
			if err := os.Remove(p); err == nil {
				expiredCount++
				// Also remove associated meta file
				if metaPath, ok := metaFiles[p]; ok {
					_ = os.Remove(metaPath)
					delete(metaFiles, p)
				}
			}
		}
	}

	// Purge orphan meta files (meta without data file)
	for base, metaPath := range metaFiles {
		if _, exists := dataFileSet[base]; !exists {
			if err := os.Remove(metaPath); err == nil {
				orphanMetaCount++
			}
		}
	}

	// Purge leftover temp files (older than 5 minutes)
	tempExpire := time.Now().Add(-5 * time.Minute)
	for _, p := range tempFiles {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.ModTime().Before(tempExpire) {
			if err := os.Remove(p); err == nil {
				tempFileCount++
			}
		}
	}

	if expiredCount > 0 || orphanMetaCount > 0 || tempFileCount > 0 {
		logger.Info("Janitor purged %d expired, %d orphan meta, %d temp files", 
			expiredCount, orphanMetaCount, tempFileCount)
	}

	// Purge by size if needed
	if maxSize > 0 {
		purgeBySizeLimit(root, maxSize)
	}
}

func purgeBySizeLimit(root string, maxSize int64) {
	var files []fileEntry
	var total int64

	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}

		if !isCacheFile(p) {
			return nil
		}

		// Skip meta files and temp files in size calculation
		base := filepath.Base(p)
		if strings.HasSuffix(p, ".meta") || strings.HasPrefix(base, ".tmp-") {
			return nil
		}

		files = append(files, fileEntry{
			path:  p,
			size:  info.Size(),
			mtime: info.ModTime(),
		})
		total += info.Size()
		return nil
	})

	if total <= maxSize || len(files) == 0 {
		return
	}

	// Sort by oldest first (LRU eviction)
	sort.Slice(files, func(i, j int) bool {
		return files[i].mtime.Before(files[j].mtime)
	})

	removedCount := 0
	freedBytes := int64(0)

	for _, fe := range files {
		if total <= maxSize {
			break
		}
		if err := os.Remove(fe.path); err == nil {
			total -= fe.size
			freedBytes += fe.size
			removedCount++

			// Also remove associated meta file
			metaPath := fe.path + ".meta"
			if info, err := os.Stat(metaPath); err == nil {
				freedBytes += info.Size()
				_ = os.Remove(metaPath)
			}
		}
	}

	if removedCount > 0 {
		logger.Info("Janitor purged %d files by size limit (freed %d bytes, current size: %d bytes)",
			removedCount, freedBytes, total)
	}
}

func isCacheFile(p string) bool {
	sep := string(filepath.Separator)
	return strings.Contains(p, sep+"orig"+sep) ||
		strings.Contains(p, sep+"resized"+sep) ||
		strings.Contains(p, sep+"fallback"+sep)
}
