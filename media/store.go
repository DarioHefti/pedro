package media

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// URL path prefixes under which the app serves stored images.
const (
	GeneratedImagesRoute = "/generated-images/"
	PastedImagesRoute    = "/pasted-images/"
)

// StoredImage is an image persisted to disk.
type StoredImage struct {
	// Path is the absolute file path.
	Path string
	// URL is the app-relative URL the frontend can load the image from.
	URL string
}

// ImageStore persists images.
type ImageStore interface {
	Save(img Image) (StoredImage, error)
	// Lookup maps a URL returned by Save back to its file path.
	Lookup(url string) (path string, ok bool)
	// Remove deletes a stored image by file path. Paths outside the store are refused.
	Remove(path string) error
	// RemoveAll deletes every stored image.
	RemoveAll() error
}

// DirImageStore keeps images as files in a single directory and serves them
// over HTTP under its route, e.g. GeneratedImagesRoute.
type DirImageStore struct {
	dir   string
	route string
}

func NewDirImageStore(dir, route string) *DirImageStore {
	return &DirImageStore{dir: dir, route: route}
}

// Route is the URL path prefix the store serves.
func (s *DirImageStore) Route() string { return s.route }

func (s *DirImageStore) Save(img Image) (StoredImage, error) {
	if len(img.Data) == 0 {
		return StoredImage{}, errors.New("image is empty")
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return StoredImage{}, fmt.Errorf("create image directory: %w", err)
	}

	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return StoredImage{}, err
	}
	name := fmt.Sprintf("%s-%s.%s", time.Now().Format("20060102-150405"), hex.EncodeToString(suffix), extension(img.Format))
	full := filepath.Join(s.dir, name)
	if err := os.WriteFile(full, img.Data, 0o644); err != nil {
		return StoredImage{}, fmt.Errorf("save image: %w", err)
	}
	return StoredImage{Path: full, URL: s.route + name}, nil
}

func (s *DirImageStore) Lookup(url string) (string, bool) {
	name, ok := s.fileName(url)
	if !ok {
		return "", false
	}
	return filepath.Join(s.dir, name), true
}

func (s *DirImageStore) Remove(path string) error {
	clean := filepath.Clean(path)
	if filepath.Dir(clean) != filepath.Clean(s.dir) {
		return fmt.Errorf("refusing to remove %s: not in %s", path, s.dir)
	}
	if _, ok := s.fileName(s.route + filepath.Base(clean)); !ok {
		return fmt.Errorf("refusing to remove %s: not a stored image", path)
	}
	if err := os.Remove(clean); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *DirImageStore) RemoveAll() error {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		if e.Type().IsRegular() {
			errs = append(errs, s.Remove(filepath.Join(s.dir, e.Name())))
		}
	}
	return errors.Join(errs...)
}

// ServeHTTP serves stored images. Every other path is answered with 404, so
// the store can be used directly as the app's fallback asset handler.
func (s *DirImageStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	name, ok := s.fileName(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.dir, name))
}

// fileName extracts the stored file name from a URL path under the store's
// route. Only plain names are accepted: no separators of either OS and no
// dot files, so lookups never leave the directory.
func (s *DirImageStore) fileName(urlPath string) (string, bool) {
	name := strings.TrimPrefix(urlPath, s.route)
	if name == urlPath || name == "" || name != path.Base(name) || name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		return "", false
	}
	return name, true
}

func extension(format string) string {
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return "jpg"
	case "webp":
		return "webp"
	default:
		return "png"
	}
}
