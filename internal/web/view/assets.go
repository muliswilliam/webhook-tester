package view

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// staticDir is where the server's /static/ route serves files from.
const staticDir = "static"

type assetVersion struct {
	modTime time.Time
	size    int64
	hash    string
}

var assetVersions sync.Map // static path -> assetVersion

// assetURL returns the /static/ URL for path with a content-hash query
// string, so browsers refetch an asset exactly when its content changes.
// Hashes are cached until the file's size or mtime changes.
func assetURL(path string) string {
	url := "/static/" + path
	file := filepath.Join(staticDir, path)
	info, err := os.Stat(file)
	if err != nil {
		logger.Printf("asset %s: %v", path, err)
		return url
	}
	if v, ok := assetVersions.Load(path); ok {
		if v := v.(assetVersion); v.modTime.Equal(info.ModTime()) && v.size == info.Size() {
			return url + "?v=" + v.hash
		}
	}
	content, err := os.ReadFile(file)
	if err != nil {
		logger.Printf("asset %s: %v", path, err)
		return url
	}
	sum := sha256.Sum256(content)
	v := assetVersion{modTime: info.ModTime(), size: info.Size(), hash: hex.EncodeToString(sum[:])[:12]}
	assetVersions.Store(path, v)
	return url + "?v=" + v.hash
}
