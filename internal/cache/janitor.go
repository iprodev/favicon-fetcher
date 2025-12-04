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

	now := time.Now()
	expireBefore := now.Add(-ttl)
	expiredCount := 0

	// Purge expired files
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

		if info.ModTime().Before(expireBefore) {
			if err := os.Remove(p); err == nil {
				expiredCount++
			}
		}
		return nil
	})

	if expiredCount > 0 {
		logger.Info("Janitor purged %d expired files", expiredCount)
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

		files = append(files, fileEntry{
			path:  p,
			size:  info.Size(),
			mtime: info.ModTime(),
		})
		total += info.Size()
		return nil
	})

	if total > maxSize && len(files) > 0 {
		sort.Slice(files, func(i, j int) bool {
			return files[i].mtime.Before(files[j].mtime)
		})

		removedCount := 0
		for _, fe := range files {
			if total <= maxSize {
				break
			}
			if err := os.Remove(fe.path); err == nil {
				total -= fe.size
				removedCount++
			}
		}

		if removedCount > 0 {
			logger.Info("Janitor purged %d files by size limit (freed %d bytes)", removedCount, maxSize-total)
		}
	}
}

func isCacheFile(p string) bool {
	sep := string(filepath.Separator)
	return strings.Contains(p, sep+"orig"+sep) ||
		strings.Contains(p, sep+"resized"+sep) ||
		strings.Contains(p, sep+"fallback"+sep)
}
