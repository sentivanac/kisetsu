package settings

import (
	"context"
	"testing"
)

func TestDownloadSize(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)
	if d, err := s.DownloadSize(ctx); err != nil || d != DownloadOriginal {
		t.Errorf("default = %q, %v", d, err)
	}
	if err := s.SaveDownloadSize(ctx, DownloadScreen); err != nil {
		t.Fatal(err)
	}
	if d, _ := s.DownloadSize(ctx); d != DownloadScreen {
		t.Errorf("saved = %q", d)
	}
	if err := s.SaveDownloadSize(ctx, "huge"); err == nil {
		t.Error("unknown size saved")
	}
	for _, bad := range []any{"huge", 3} {
		if err := s.Save(ctx, downloadSizeKey, bad); err != nil {
			t.Fatal(err)
		}
		if d, err := s.DownloadSize(ctx); err != nil || d != DownloadOriginal {
			t.Errorf("stored %v: %q, %v", bad, d, err)
		}
	}
}
