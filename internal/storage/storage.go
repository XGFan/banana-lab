package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Storage handles saving and deleting files under Root.
type Storage struct {
	Root string
}

// SaveBytes writes data to Root/relDir/filename and returns the relative path and size.
func (s Storage) SaveBytes(relDir, filename string, data []byte) (string, int64, error) {
	if s.Root == "" {
		return "", 0, errors.New("storage root is empty")
	}
	if filename == "" {
		return "", 0, errors.New("filename is empty")
	}

	relPath := filepath.Join(relDir, filename)
	absPath, cleanRel, err := s.safeAbsPath(relPath)
	if err != nil {
		return "", 0, err
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return "", 0, err
	}
	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		return "", 0, err
	}

	return cleanRel, int64(len(data)), nil
}

// Delete removes a file by its relative path under Root.
func (s Storage) Delete(relPath string) error {
	if s.Root == "" {
		return errors.New("storage root is empty")
	}

	absPath, _, err := s.safeAbsPath(relPath)
	if err != nil {
		return err
	}

	return os.Remove(absPath)
}

func (s Storage) safeAbsPath(relPath string) (string, string, error) {
	if relPath == "" {
		return "", "", errors.New("relative path is empty")
	}
	if filepath.IsAbs(relPath) {
		return "", "", errors.New("absolute paths are not allowed")
	}

	cleanRel := filepath.Clean(relPath)
	if cleanRel == "." || cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(os.PathSeparator)) {
		return "", "", errors.New("path traversal detected")
	}

	absPath := filepath.Join(s.Root, cleanRel)
	rel, err := filepath.Rel(s.Root, absPath)
	if err != nil {
		return "", "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", "", errors.New("path escapes root")
	}

	return absPath, rel, nil
}
