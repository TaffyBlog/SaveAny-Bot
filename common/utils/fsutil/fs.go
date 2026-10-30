package fsutil

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/gabriel-vasile/mimetype"
)

// 删除文件夹内的所有文件和子目录, 但不删除文件夹本身
func RemoveAllInDir(dirPath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		entryPath := filepath.Join(dirPath, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			return err
		}
	}
	return nil
}

func DetectFileExt(fp string) string {
	mt, err := mimetype.DetectFile(fp)
	if err != nil {
		return ""
	}
	return mt.Extension()
}

type File struct {
	*os.File
}

func (f *File) Remove() error {
	return os.Remove(f.Name())
}

func (f *File) CloseAndRemove() error {
	closeErr := f.Close()
	if errors.Is(closeErr, os.ErrClosed) {
		closeErr = nil
	}
	return errors.Join(closeErr, f.Remove())
}

func CreateFile(fp string) (*File, error) {
	if err := os.MkdirAll(filepath.Dir(fp), os.ModePerm); err != nil {
		return nil, err
	}
	file, err := os.Create(fp)
	if err != nil {
		return nil, err
	}
	return &File{File: file}, nil
}

// OpenResumeFile opens (or creates) the file without truncating it and
// returns the file plus its current size, so callers can resume a partial
// download by appending to it. The write offset is positioned at the end
// of the file, so plain Write calls append instead of overwriting the
// existing bytes from offset 0.
func OpenResumeFile(fp string) (*File, int64, error) {
	if err := os.MkdirAll(filepath.Dir(fp), os.ModePerm); err != nil {
		return nil, 0, err
	}
	file, err := os.OpenFile(fp, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, 0, err
	}
	fi, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, err
	}
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		file.Close()
		return nil, 0, err
	}
	return &File{File: file}, fi.Size(), nil
}

func NormalizePathname(s string) string {
	specials := `\/:*?"<>|` + "\n\r\t"
	var builder strings.Builder
	for _, ch := range s {
		if strings.ContainsRune(specials, ch) || unicode.IsControl(ch) {
			builder.WriteRune('_')
		} else {
			builder.WriteRune(ch)
		}
	}

	result := strings.TrimRightFunc(builder.String(), func(r rune) bool {
		return r == '.' || r == '_' || unicode.IsSpace(r)
	})

	return result
}
