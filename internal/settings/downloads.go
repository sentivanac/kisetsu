package settings

import (
	"context"
	"errors"
	"fmt"
)

const downloadSizeKey = "downloads.size"

// DownloadSize says what a download stores.
type DownloadSize string

const (
	// DownloadOriginal stores the file as the source has it.
	DownloadOriginal DownloadSize = "original"
	// DownloadScreen stores a copy reduced to cover the screen.
	DownloadScreen DownloadSize = "screen"
)

// Valid reports whether d is a known size.
func (d DownloadSize) Valid() bool { return d == DownloadOriginal || d == DownloadScreen }

// DownloadSize returns the chosen size; the original unless the user chose
// otherwise. Unusable stored values also give the original.
func (s *Store) DownloadSize(ctx context.Context) (DownloadSize, error) {
	var d DownloadSize
	found, err := s.Load(ctx, downloadSizeKey, &d)
	switch {
	case errors.Is(err, ErrInvalidValue):
		return DownloadOriginal, nil
	case err != nil:
		return DownloadOriginal, err
	case !found || !d.Valid():
		return DownloadOriginal, nil
	}
	return d, nil
}

// SaveDownloadSize stores the chosen size.
func (s *Store) SaveDownloadSize(ctx context.Context, d DownloadSize) error {
	if !d.Valid() {
		return fmt.Errorf("invalid download size %q", d)
	}
	return s.Save(ctx, downloadSizeKey, d)
}
