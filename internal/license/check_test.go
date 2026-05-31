package license

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerify_OK(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"README.md", "THIRD_PARTY.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Verify(dir); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestVerify_MissingFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Verify(dir)
	if err == nil {
		t.Fatal("expected error for missing THIRD_PARTY.md")
	}
	if !strings.Contains(err.Error(), "THIRD_PARTY.md") {
		t.Errorf("expected error to mention THIRD_PARTY.md, got %v", err)
	}
}

func TestVerify_MissingDir(t *testing.T) {
	if err := Verify(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Error("expected error for missing dir")
	}
}

func TestVerify_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"readme.md", "third_party.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Verify(dir); err != nil {
		t.Errorf("expected case-insensitive match, got %v", err)
	}
}
