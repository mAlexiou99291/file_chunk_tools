package file_chunk_tools

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func writeTempFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestNewChunkReaderRejectsZeroChunkSize(t *testing.T) {
	path := writeTempFile(t, "data.bin", []byte("hello"))
	_, err := NewChunkReader(path, 0)
	if !errors.Is(err, ErrInvalidChunkSize) {
		t.Fatalf("expected ErrInvalidChunkSize, got %v", err)
	}
}

func TestNewChunkReaderRejectsNegativeChunkSize(t *testing.T) {
	path := writeTempFile(t, "data.bin", []byte("hello"))
	_, err := NewChunkReader(path, -1)
	if !errors.Is(err, ErrInvalidChunkSize) {
		t.Fatalf("expected ErrInvalidChunkSize, got %v", err)
	}
}

func TestNewChunkReaderRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := NewChunkReader(dir, 4)
	if err == nil {
		t.Fatal("expected error for directory, got nil")
	}
}

func TestChunkAtReadsFullChunk(t *testing.T) {
	path := writeTempFile(t, "data.bin", []byte("abcdefghij"))
	r, err := NewChunkReader(path, 4)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer r.Close()

	c, err := r.ChunkAt(0)
	if err != nil {
		t.Fatalf("ChunkAt(0): %v", err)
	}
	if got := string(c.Bytes()); got != "abcd" {
		t.Fatalf("chunk 0 = %q, want %q", got, "abcd")
	}
	if c.Offset() != 0 {
		t.Fatalf("offset = %d, want 0", c.Offset())
	}
	if c.Len() != 4 {
		t.Fatalf("len = %d, want 4", c.Len())
	}
}

func TestChunkAtReadsFinalShortChunk(t *testing.T) {
	path := writeTempFile(t, "data.bin", []byte("abcdefghij"))
	r, err := NewChunkReader(path, 4)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer r.Close()

	c, err := r.ChunkAt(8)
	if err != nil {
		t.Fatalf("ChunkAt(8): %v", err)
	}
	if got := string(c.Bytes()); got != "ij" {
		t.Fatalf("final chunk = %q, want %q", got, "ij")
	}
	if c.Len() != 2 {
		t.Fatalf("len = %d, want 2", c.Len())
	}
}

func TestChunkAtOffsetEqualToSizeIsEmpty(t *testing.T) {
	path := writeTempFile(t, "data.bin", []byte("abc"))
	r, err := NewChunkReader(path, 2)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer r.Close()

	c, err := r.ChunkAt(3)
	if err != nil {
		t.Fatalf("ChunkAt(size): %v", err)
	}
	if c.Len() != 0 {
		t.Fatalf("len = %d, want 0", c.Len())
	}
}

func TestChunkAtOffsetPastSizeReturnsEOF(t *testing.T) {
	path := writeTempFile(t, "data.bin", []byte("abc"))
	r, err := NewChunkReader(path, 2)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer r.Close()

	_, err = r.ChunkAt(4)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF, got %v", err)
	}
}

func TestChunkAtNegativeOffset(t *testing.T) {
	path := writeTempFile(t, "data.bin", []byte("abc"))
	r, err := NewChunkReader(path, 2)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer r.Close()

	_, err = r.ChunkAt(-1)
	if err == nil {
		t.Fatal("expected error for negative offset, got nil")
	}
}

func TestChunkReadExposesBytes(t *testing.T) {
	path := writeTempFile(t, "data.bin", []byte("xyz"))
	r, err := NewChunkReader(path, 2)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer r.Close()

	c, err := r.ChunkAt(0)
	if err != nil {
		t.Fatalf("ChunkAt: %v", err)
	}
	buf := make([]byte, 2)
	n, err := c.Read(buf)
	if n != 2 {
		t.Fatalf("Read n = %d, want 2", n)
	}
	if err != io.EOF {
		t.Fatalf("Read err = %v, want io.EOF", err)
	}
	if string(buf) != "xy" {
		t.Fatalf("Read buf = %q, want %q", buf, "xy")
	}
}

func TestChunkReadAtPartial(t *testing.T) {
	path := writeTempFile(t, "data.bin", []byte("abcdef"))
	r, err := NewChunkReader(path, 4)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer r.Close()

	c, err := r.ChunkAt(0)
	if err != nil {
		t.Fatalf("ChunkAt: %v", err)
	}
	buf := make([]byte, 2)
	n, err := c.ReadAt(buf, 1)
	if err != nil {
		t.Fatalf("ReadAt: %v", err)
	}
	if n != 2 {
		t.Fatalf("ReadAt n = %d, want 2", n)
	}
	if string(buf) != "bc" {
		t.Fatalf("ReadAt buf = %q, want %q", buf, "bc")
	}
}

func TestChunkReadAtPastEnd(t *testing.T) {
	path := writeTempFile(t, "data.bin", []byte("abcd"))
	r, err := NewChunkReader(path, 4)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer r.Close()

	c, err := r.ChunkAt(0)
	if err != nil {
		t.Fatalf("ChunkAt: %v", err)
	}
	buf := make([]byte, 1)
	_, err = c.ReadAt(buf, 4)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF, got %v", err)
	}
}

func TestChunkReadAtNegativeOffset(t *testing.T) {
	path := writeTempFile(t, "data.bin", []byte("abcd"))
	r, err := NewChunkReader(path, 4)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer r.Close()

	c, err := r.ChunkAt(0)
	if err != nil {
		t.Fatalf("ChunkAt: %v", err)
	}
	buf := make([]byte, 1)
	_, err = c.ReadAt(buf, -1)
	if err == nil {
		t.Fatal("expected error for negative ReadAt offset, got nil")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	path := writeTempFile(t, "data.bin", []byte("abc"))
	r, err := NewChunkReader(path, 2)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
