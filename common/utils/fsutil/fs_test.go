package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenResumeFileAppends(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "resume.bin")

	// First pass: write initial content and close.
	f1, size, err := OpenResumeFile(fp)
	if err != nil {
		t.Fatal(err)
	}
	if size != 0 {
		t.Fatalf("initial size = %d, want 0", size)
	}
	if _, err := f1.WriteString("hello"); err != nil {
		t.Fatal(err)
	}
	if err := f1.Close(); err != nil {
		t.Fatal(err)
	}

	// Second pass: reopen and append; must not overwrite existing bytes.
	f2, size, err := OpenResumeFile(fp)
	if err != nil {
		t.Fatal(err)
	}
	if size != 5 {
		t.Fatalf("resumed size = %d, want 5", size)
	}
	if _, err := f2.WriteString("world"); err != nil {
		t.Fatal(err)
	}
	if err := f2.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(fp)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "helloworld" {
		t.Fatalf("content = %q, want %q", got, "helloworld")
	}
	if len(got) != 10 {
		t.Fatalf("final size = %d, want 10", len(got))
	}
}
