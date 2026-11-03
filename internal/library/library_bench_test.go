package library

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sentivanac/kisetsu/internal/database"
	"github.com/sentivanac/kisetsu/internal/imaging"
)

// benchSize is a large personal library: far more than most users keep.
const benchSize = 10_000

// benchStore returns a store with n library images.
func benchStore(b *testing.B, n int) *Store {
	b.Helper()
	db, err := database.Open(context.Background(), filepath.Join(b.TempDir(), database.FileName))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { db.Close() })
	s := NewStore(db)
	for i := range n {
		e := entry(i)
		e.DHash = uint64(i) * 0x9E3779B97F4A7C15 // spread over all bits, like real hashes
		if _, err := s.Add(context.Background(), e); err != nil {
			b.Fatal(err)
		}
	}
	return s
}

// One page of the Library tab.
func BenchmarkListPage(b *testing.B) {
	s := benchStore(b, benchSize)
	ctx := context.Background()
	for b.Loop() {
		if _, err := s.List(ctx, ListOptions{Limit: 60, Offset: benchSize / 2}); err != nil {
			b.Fatal(err)
		}
	}
}

// The count and size shown above the Library tab.
func BenchmarkStats(b *testing.B) {
	s := benchStore(b, benchSize)
	ctx := context.Background()
	for b.Loop() {
		if _, err := s.Stats(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

// The duplicate check after every download: all hashes are read and
// compared, as downloads.Manager does.
func BenchmarkDuplicateCheck(b *testing.B) {
	s := benchStore(b, benchSize)
	ctx := context.Background()
	for b.Loop() {
		refs, err := s.DHashes(ctx)
		if err != nil {
			b.Fatal(err)
		}
		for _, r := range refs {
			imaging.Similar(0x0123_4567_89AB_CDEF, r.DHash)
		}
	}
}

// The candidates of a rotation from the whole library.
func BenchmarkRotationCandidates(b *testing.B) {
	s := benchStore(b, benchSize)
	ctx := context.Background()
	for b.Loop() {
		if _, err := s.IDs(ctx, 0); err != nil {
			b.Fatal(err)
		}
	}
}
