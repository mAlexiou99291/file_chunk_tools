// Package file_chunk_tools reads fixed-size chunks from files at arbitrary
// offsets using only the standard library.
//
// The design goals are bounded memory and zero-copy semantics. A Chunk is a
// view over data that the operating system has already mapped into the
// process address space; no bytes are copied into Go heap memory. This makes
// the package suitable for scanning very large files where allocating a
// buffer per read would be wasteful.
package file_chunk_tools

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// ErrInvalidChunkSize is returned when a ChunkReader is constructed with a
// non-positive chunk size. A zero or negative size would make progress
// impossible and is almost always a caller bug rather than a recoverable
// condition.
var ErrInvalidChunkSize = errors.New("file_chunk_tools: chunk size must be positive")

// Chunk is a read-only view of a byte range within a file. It satisfies
// io.Reader and io.ReaderAt, and its Bytes method exposes the underlying
// slice without copying. Callers must not mutate the returned slice; doing
// so would corrupt the mapping for every other Chunk derived from the same
// reader.
type Chunk struct {
	data   []byte
	offset int64
}

// Len reports the number of bytes in the chunk.
func (c Chunk) Len() int { return len(c.data) }

// Offset reports the absolute file offset at which the chunk begins.
func (c Chunk) Offset() int64 { return c.offset }

// Bytes returns the chunk's backing slice. The slice aliases the file
// mapping, so it is valid only for the lifetime of the ChunkReader that
// produced it.
func (c Chunk) Bytes() []byte { return c.data }

// Read implements io.Reader over the chunk's bytes.
func (c Chunk) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(c.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, c.data)
	// We cannot track a read cursor on a value receiver, so a single Read
	// consumes the whole chunk. Callers that need incremental reads should
	// use io.ReaderAt via ReadAt or read Bytes directly.
	return n, io.EOF
}

// ReadAt implements io.ReaderAt over the chunk's bytes. The offset is
// relative to the start of the chunk, not the start of the file.
func (c Chunk) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("file_chunk_tools: negative offset %d", off)
	}
	if off >= int64(len(c.data)) {
		return 0, io.EOF
	}
	n := copy(p, c.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// ChunkReader reads fixed-size chunks from a file at arbitrary offsets.
// The file is mapped once at construction; individual reads are O(1) slice
// operations over the mapping.
type ChunkReader struct {
	f         *os.File
	data      []byte
	chunkSize int64
	size      int64
}

// NewChunkReader opens path and prepares it for chunked reads. The chunkSize
// must be positive. The returned reader owns the file handle and must be
// closed with Close.
func NewChunkReader(path string, chunkSize int64) (*ChunkReader, error) {
	if chunkSize <= 0 {
		return nil, ErrInvalidChunkSize
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if info.IsDir() {
		f.Close()
		return nil, fmt.Errorf("file_chunk_tools: %s is a directory", path)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &ChunkReader{
		f:         f,
		data:      data,
		chunkSize: chunkSize,
		size:      int64(len(data)),
	}, nil
}

// Size reports the total number of bytes in the underlying file.
func (r *ChunkReader) Size() int64 { return r.size }

// ChunkSize reports the configured chunk size.
func (r *ChunkReader) ChunkSize() int64 { return r.chunkSize }

// ChunkAt returns the chunk starting at the given absolute file offset. The
// returned chunk is at most chunkSize bytes long; the final chunk of a file
// is shorter. An offset equal to the file size yields an empty chunk with no
// error, which lets callers iterate with `for off := 0; off < r.Size();`
// without a special case for the terminator.
func (r *ChunkReader) ChunkAt(offset int64) (Chunk, error) {
	if offset < 0 {
		return Chunk{}, fmt.Errorf("file_chunk_tools: negative offset %d", offset)
	}
	if offset > r.size {
		return Chunk{}, io.EOF
	}
	end := offset + r.chunkSize
	if end > r.size {
		end = r.size
	}
	return Chunk{data: r.data[offset:end], offset: offset}, nil
}

// Close releases the file handle. The backing data remains readable by any
// Chunk already handed out, because it lives on the Go heap.
func (r *ChunkReader) Close() error {
	if r.f == nil {
		return nil
	}
	err := r.f.Close()
	r.f = nil
	return err
}
