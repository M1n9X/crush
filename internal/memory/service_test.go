package memory

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteAndRead(t *testing.T) {
	svc := NewService(t.TempDir())

	written, err := svc.Write("coder", "notes.md", "hello memory")
	require.NoError(t, err)
	require.Equal(t, "memory/coder/notes.md", written.Path)

	read, err := svc.Read("coder", "notes.md")
	require.NoError(t, err)
	require.Equal(t, written.Path, read.Path)
	require.Equal(t, "hello memory", read.Content)
	require.False(t, read.Truncated)
}

func TestPathTraversalRejected(t *testing.T) {
	svc := NewService(t.TempDir())

	_, err := svc.Read("coder", "../secrets.txt")
	require.Error(t, err)

	_, err = svc.Write("coder", "../secrets.txt", "nope")
	require.Error(t, err)
}

func TestListTruncatesLargeFiles(t *testing.T) {
	svc := NewService(t.TempDir())
	big := strings.Repeat("a", maxSingleMemoryFileSize+10)

	_, err := svc.Write("coder", "big.md", big)
	require.NoError(t, err)

	entries, err := svc.List("coder")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.True(t, entries[0].Truncated)
	require.Equal(t, maxSingleMemoryFileSize, len(entries[0].Content))
	require.Equal(t, filepath.ToSlash("memory/coder/big.md"), entries[0].Path)
}
