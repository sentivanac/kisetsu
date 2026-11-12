package main

import (
	"testing"

	"github.com/sentivanac/kisetsu/internal/apperr"
)

func TestHideAndShowAgain(t *testing.T) {
	app, _, id := appWithFolder(t)
	page, err := app.ListFolder(id, 0, 10)
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("folder: %d items, %v", len(page.Items), err)
	}
	img := page.Items[0].ID
	if err := app.HideImage(img); err != nil {
		t.Fatal(err)
	}
	page, _ = app.ListFolder(id, 0, 10)
	hidden := 0
	for _, it := range page.Items {
		if it.Hidden {
			hidden++
		}
	}
	if hidden != 1 {
		t.Errorf("%d items marked hidden, want 1", hidden)
	}
	list, err := app.ListHidden()
	if err != nil || len(list) != 1 || list[0].ID != img || !list[0].Hidden {
		t.Errorf("hidden list: %+v, %v", list, err)
	}
	if err := app.ShowImageAgain(img); err != nil {
		t.Fatal(err)
	}
	if list, _ := app.ListHidden(); len(list) != 0 {
		t.Errorf("hidden after showing again: %+v", list)
	}
	if err := app.HideImage(99999); apperr.ToPublic(err).Code != apperr.InvalidInput {
		t.Errorf("hiding a missing image: %v", err)
	}
}

func TestHideCurrentWallpaper(t *testing.T) {
	app, _, id := appWithFolder(t)
	if _, err := app.HideCurrentWallpaper(); apperr.ToPublic(err).Code != apperr.InvalidInput {
		t.Errorf("no wallpaper yet: %v", err)
	}
	page, _ := app.ListFolder(id, 0, 10)
	first := page.Items[0].ID
	if _, err := app.SetWallpaper(first); err != nil {
		t.Fatal(err)
	}
	// Rotation from the folder, so the next image is the other one.
	s := RotationSettings{Interval: "1h", Order: "random", FolderID: id}
	if _, err := app.SaveRotation(s); err != nil {
		t.Fatal(err)
	}
	state, err := app.HideCurrentWallpaper()
	if err != nil {
		t.Fatal(err)
	}
	if state.LastImageID == first {
		t.Errorf("the hidden image is still the wallpaper")
	}
	if list, _ := app.ListHidden(); len(list) != 1 || list[0].ID != first {
		t.Errorf("hidden list %+v, want image %d", list, first)
	}
}
