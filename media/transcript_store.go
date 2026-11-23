package media

import (
	"io"
	"os"
	"path/filepath"
)

// TranscriptStore saves a local, readable transcript and returns its path.
type TranscriptStore interface {
	Save(text string) (string, error)
}

func NewDirTranscriptStore(dir string) TranscriptStore { return dirTranscriptStore{dir: dir} }

type dirTranscriptStore struct{ dir string }

func (s dirTranscriptStore) Save(text string) (string, error) {
	dir, err := filepath.Abs(s.dir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "transcript-*.txt")
	if err != nil {
		return "", err
	}
	_, writeErr := io.WriteString(f, text)
	closeErr := f.Close()
	if err := writeErr; err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	if closeErr != nil {
		_ = os.Remove(f.Name())
		return "", closeErr
	}
	return f.Name(), nil
}
