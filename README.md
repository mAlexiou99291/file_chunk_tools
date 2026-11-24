# file_chunk_tools

Reads fixed-size chunks from files at arbitrary offsets with bounded memory and zero-copy semantics.

## Usage

```go
package main

import (
	"fmt"
	"io"

	"file_chunk_tools"
)

func main() {
	r, err := file_chunk_tools.NewChunkReader("large.bin", 4096)
	if err != nil {
		panic(err)
	}
	defer r.Close()

	for off := int64(0); off < r.Size(); off += r.ChunkSize() {
		c, err := r.ChunkAt(off)
		if err != nil {
			panic(err)
		}
		// c.Bytes() aliases the file mapping; do not mutate it.
		fmt.Println(c.Offset(), c.Len(), len(c.Bytes()))
	}

	// A chunk is also an io.Reader and an io.ReaderAt.
	c, _ := r.ChunkAt(0)
	buf := make([]byte, 16)
	n, err := c.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		panic(err)
	}
	_ = n
}
```

## Why this exists

The problem is scanning a file in pieces without allocating a buffer per
piece. The standard library gives you `os.File.ReadAt`, which copies into a
caller-supplied slice, and `syscall.Mmap`, which is platform-specific and
leaks a `[]byte` you must unmap by hand. This package wraps the second option
behind a small, portable API: `ChunkReader` opens and reads the file once at
construction, then hands out `Chunk` values that are plain slices over the
same backing array. No per-chunk allocation, no `unsafe`, no build tags.

The trade-off is that the whole file is held in memory for the lifetime of
the reader. That is fine for the intended use case — random access over a
file that is already resident in the page cache, or a file small enough that
mapping it is cheaper than seeking — but it is not a streaming API. If the
file does not fit in memory, use `os.File.ReadAt` and accept the copy.

## The awkward edge

`ChunkAt` accepts an offset equal to the file size and returns an empty
chunk with no error. This lets the natural loop
`for off := 0; off < r.Size(); off += r.ChunkSize()` terminate without a
special case. Offsets greater than the file size return `io.EOF`; negative
offsets return an error.

A `Chunk`'s `Read` method consumes the entire chunk in one call and then
returns `io.EOF`. It is a value type with no read cursor, so it cannot be
read incrementally. Use `ReadAt` or `Bytes` when you need partial reads.

## API

- `NewChunkReader(path string, chunkSize int64) (*ChunkReader, error)`
- `(*ChunkReader) Size() int64`
- `(*ChunkReader) ChunkSize() int64`
- `(*ChunkReader) ChunkAt(offset int64) (Chunk, error)`
- `(*ChunkReader) Close() error`
- `(Chunk) Len() int`
- `(Chunk) Offset() int64`
- `(Chunk) Bytes() []byte`
- `(Chunk) Read(p []byte) (int, error)`
- `(Chunk) ReadAt(p []byte, off int64) (int, error)`
- `ErrInvalidChunkSize`
