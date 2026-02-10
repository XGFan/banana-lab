package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveDelete(t *testing.T) {
	root := t.TempDir()
	store := Storage{Root: root}

	data := []byte("hello-storage")
	relPath, size, err := store.SaveBytes("images/sub", "file.txt", data)
	if err != nil {
		t.Fatalf("SaveBytes error: %v", err)
	}
	if size != int64(len(data)) {
		t.Fatalf("size mismatch: got %d want %d", size, len(data))
	}
	if filepath.IsAbs(relPath) {
		t.Fatalf("relPath should be relative: %s", relPath)
	}

	absPath := filepath.Join(root, relPath)
	fileData, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if !bytes.Equal(fileData, data) {
		t.Fatalf("file content mismatch: got %q want %q", string(fileData), string(data))
	}

	if err := store.Delete(relPath); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if _, err := os.Stat(absPath); !os.IsNotExist(err) {
		t.Fatalf("file should be deleted, stat err: %v", err)
	}

	if err := store.Delete("../evil.txt"); err == nil {
		t.Fatalf("expected path traversal error")
	}
}
