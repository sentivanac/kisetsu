package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sentivanac/kisetsu/internal/apperr"
	"github.com/sentivanac/kisetsu/internal/downloads"
	"github.com/sentivanac/kisetsu/internal/images"
	"github.com/sentivanac/kisetsu/internal/imaging"
)

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for x := range 320 {
		for y := range 200 {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 100, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func downloadable(srv *httptest.Server, id, path string) images.Image {
	return images.Image{
		Provider:     "stub",
		ProviderID:   id,
		PageURL:      "https://example.org/" + id,
		ThumbnailURL: srv.URL + "/thumb.jpg",
		DownloadURL:  srv.URL + path,
		Width:        320,
		Height:       200,
		License:      images.License{Status: images.LicenseUnknown},
	}
}

func TestDownloadEmitsProgressUntilDone(t *testing.T) {
	data := jpegBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer srv.Close()
	app := newTestApp(t, &stubProvider{})
	ev := captureEvents(app)

	key, err := app.Download(downloadable(srv, "abc", "/a.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if key != "stub/abc" {
		t.Errorf("key = %q", key)
	}

	done := ev.wait(t)
	if done.Key != key || done.State != "done" || done.EntryID == 0 || done.Error != nil {
		t.Errorf("final event = %+v", done)
	}
	ev.mu.Lock()
	first := ev.list[0]
	ev.mu.Unlock()
	if first.State != "queued" {
		t.Errorf("first event = %+v, want queued", first)
	}

	_, err = app.Download(downloadable(srv, "abc", "/a.jpg"))
	if got := apperr.ToPublic(err).Code; got != AlreadyInLibrary {
		t.Errorf("second download: code = %q, want %q", got, AlreadyInLibrary)
	}

	status, err := app.LibraryStatus([]string{"stub/abc", "stub/other", "malformed"})
	if err != nil {
		t.Fatal(err)
	}
	if len(status) != 1 || status["stub/abc"] != done.EntryID {
		t.Errorf("LibraryStatus = %v, want stub/abc -> %d", status, done.EntryID)
	}
}

func TestDownloadRejectsInvalidImage(t *testing.T) {
	app := newTestApp(t, &stubProvider{})

	_, err := app.Download(images.Image{Provider: "stub", ProviderID: "x"})
	if got := apperr.ToPublic(err).Code; got != apperr.InvalidInput {
		t.Errorf("code = %q, want %q", got, apperr.InvalidInput)
	}
}

func TestFailedDownloadEventHasUserMessage(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	app := newTestApp(t, &stubProvider{})
	ev := captureEvents(app)

	if _, err := app.Download(downloadable(srv, "missing", "/gone.jpg")); err != nil {
		t.Fatal(err)
	}
	done := ev.wait(t)
	if done.State != "failed" || done.Error == nil || !strings.Contains(done.Error.Message, "did not provide the file") {
		t.Errorf("final event = %+v (error %+v)", done, done.Error)
	}
	if strings.Contains(done.Error.Message, "404") {
		t.Errorf("HTTP status leaked into %q", done.Error.Message)
	}
}

func TestDownloadErrorMessages(t *testing.T) {
	tests := []struct {
		err  error
		code apperr.Code
		msg  string
	}{
		{fmt.Errorf("x: %w", downloads.ErrFileTooLarge), apperr.InvalidInput, "100 MB"},
		{imaging.ErrUnsupportedFormat, apperr.InvalidInput, "JPEG, PNG or WebP"},
		{imaging.ErrCorrupt, apperr.InvalidInput, "damaged"},
		{imaging.ErrTooLarge, apperr.InvalidInput, "too many pixels"},
		{downloads.ErrHTTP, apperr.ProviderUnavailable, "did not provide"},
		{context.DeadlineExceeded, apperr.ProviderUnavailable, "internet connection"},
		{errors.New("disk on fire"), apperr.Internal, "log file"},
	}
	for _, tt := range tests {
		pub := apperr.ToPublic(downloadError(tt.err))
		if pub.Code != tt.code || !strings.Contains(pub.Message, tt.msg) {
			t.Errorf("%v: got %q %q, want %q containing %q", tt.err, pub.Code, pub.Message, tt.code, tt.msg)
		}
	}
}

func TestNotifyBeforeStartupDoesNotCrash(t *testing.T) {
	app := newTestApp(t, &stubProvider{})
	// The default emitter must be a no-op; the Wails runtime would exit
	// the process without its context.
	app.notifyDownload(downloads.Progress{Key: "stub/x", State: downloads.Queued})
}
