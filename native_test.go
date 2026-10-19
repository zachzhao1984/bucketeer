package main

import (
	"bucketeer/internal/store"
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func nativeTestApp(t *testing.T) *App {
	t.Helper()
	a := newApp(newTestService(t))
	t.Cleanup(a.close)
	return a
}

func TestNativeNavigationAndConnectionForm(t *testing.T) {
	a := nativeTestApp(t)
	tester := ui.NewTester(a.view, 1180, 760)
	tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
	if !tester.HasText("Connect Your Storage") {
		t.Fatal(tester.Texts())
	}
	if err := tester.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !tester.HasText("Browsing") {
		t.Fatalf("Settings did not open: %v", tester.Texts())
	}
	if err := tester.Click("Add Connection"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !tester.HasText("New Connection") {
		t.Fatal("connection form missing")
	}
	if err := tester.Click("Name"); err != nil {
		t.Fatal(err)
	}
	tester.Type("Test account")
	tester.Frame()
	if a.conn.name != "Test account" {
		t.Fatalf("input did not persist: %q", a.conn.name)
	}
	if err := tester.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if a.dlg != dialogNone {
		t.Fatal("cancel did not close dialog")
	}
}

func TestNativeViews(t *testing.T) {
	a := nativeTestApp(t)
	p := store.Profile{ID: "demo", Name: "Acme Media", Endpoint: "https://s3.example.com"}
	a.session = SessionInfo{Connected: true, Profile: &p, Host: "s3.example.com"}
	a.bucket = "acme-media"
	a.section = sectionFiles
	a.listing = store.Listing{Items: []store.Entry{{Key: "images/", Name: "images", Folder: true}, {Key: "notes.txt", Name: "notes.txt", Size: 412, LastModified: time.Now()}, {Key: "photo.png", Name: "photo.png", Size: 2048}}}
	tester := ui.NewTester(a.view, 1180, 760)
	tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
	if !tester.HasText("notes.txt") {
		t.Fatal(tester.Texts())
	}
	for _, mode := range []ViewMode{viewGrid, viewList} {
		a.mode = mode
		tester.Frame()
		tester.SetDark(true)
		tester.Frame()
		tester.SetDark(false)
	}
	a.stat = &store.Stat{Bucket: a.bucket, Objects: 3, Bytes: 2460, TopFolders: []store.FolderTotal{{Prefix: "images/", Name: "images", Bytes: 2048}}, Largest: a.listing.Items[1:]}
	a.section = sectionDashboard
	tester.Frame()
	a.transfersOpen = true
	tester.Frame()
	a.section = sectionSettings
	tester.Frame()
	a.openNewFolder()
	tester.Frame()
	a.dlg = dialogNone
	tester.Frame()
	a.openConnection(&p)
	tester.Frame()
	a.dlg = dialogNone
	tester.Frame()
	tester.SetSize(900, 560)
	tester.Frame()
	tester.SetTitleBar(ui.TitleBar{})
	tester.Frame()
}

func TestGenerateShots(t *testing.T) {
	dir := os.Getenv("PAIL_SHOTS")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	save := func(tester *ui.Tester, name string) {
		t.Helper()
		tester.Frame()
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tester.Image()); err != nil {
			t.Fatal(err)
		}
	}

	p := store.Profile{ID: "demo", Name: "Acme Media", Endpoint: "https://s3.example.com"}

	// 1. Welcome (disconnected)
	{
		a := nativeTestApp(t)
		tester := ui.NewTester(a.view, 1180, 760)
		tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
		save(tester, "welcome.png")
	}

	// 2. Files (grid, light and dark)
	{
		a := nativeTestApp(t)
		a.session = SessionInfo{Connected: true, Profile: &p, Host: "s3.example.com"}
		a.bucket = "acme-media"
		a.section = sectionFiles
		a.mode = viewGrid
		a.listing = store.Listing{Items: []store.Entry{
			{Key: "images/", Name: "images", Folder: true},
			{Key: "documents/", Name: "documents", Folder: true},
			{Key: "notes.txt", Name: "notes.txt", Size: 412, LastModified: time.Now()},
			{Key: "photo.png", Name: "photo.png", Size: 2048},
			{Key: "archive.zip", Name: "archive.zip", Size: 1048576},
		}}
		tester := ui.NewTester(a.view, 1180, 760)
		tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
		save(tester, "files.png")

		tester.SetDark(true)
		save(tester, "files-dark.png")
		tester.SetDark(false)

		// 3. List mode
		a.mode = viewList
		save(tester, "list.png")

		// 4. Transfers open
		a.transfersOpen = true
		save(tester, "transfers.png")
		a.transfersOpen = false

		// 5. Modal dialog
		a.openNewFolder()
		save(tester, "prompt.png")
		a.dlg = dialogNone

		a.openConnection(&p)
		save(tester, "dialog.png")
		a.dlg = dialogNone

		// Detail
		a.detail = detailState{
			key: "photo.png",
			info: store.ObjectInfo{
				Key:          "photo.png",
				Name:         "photo.png",
				Size:         2048,
				ContentType:  "image/png",
				StorageClass: "STANDARD",
				LastModified: time.Now(),
			},
		}
		a.dlg = dialogDetail
		save(tester, "detail.png")
		a.dlg = dialogNone

		// Confirm
		a.ask("Delete “photo.png”?", "This cannot be undone.", "Delete", func() {})
		save(tester, "confirm.png")
		a.confirm.open = false

		// 8. The narrowest the window is allowed to get.
		a.mode = viewGrid
		tester.SetSize(900, 560)
		save(tester, "files-compact.png")
		a.mode = viewList
		save(tester, "list-compact.png")
		tester.SetSize(1180, 760)
	}

	// 6. Dashboard (light and dark)
	{
		a := nativeTestApp(t)
		a.session = SessionInfo{Connected: true, Profile: &p, Host: "s3.example.com"}
		a.bucket = "acme-media"
		a.section = sectionDashboard
		a.stat = &store.Stat{
			Bucket:  a.bucket,
			Objects: 124,
			Bytes:   1024 * 1024 * 420,
			Folders: 8,
			TopFolders: []store.FolderTotal{
				{Prefix: "images/", Name: "images", Bytes: 1024 * 1024 * 280},
				{Prefix: "videos/", Name: "videos", Bytes: 1024 * 1024 * 110},
				{Prefix: "backups/", Name: "backups", Bytes: 1024 * 1024 * 30},
			},
			ByClass: []store.ClassTotal{
				{Class: "STANDARD", Bytes: 1024 * 1024 * 380, Count: 110},
				{Class: "GLACIER", Bytes: 1024 * 1024 * 40, Count: 14},
			},
			Largest: []store.Entry{
				{Key: "videos/launch.mp4", Name: "launch.mp4", Size: 1024 * 1024 * 85},
				{Key: "images/hero.raw", Name: "hero.raw", Size: 1024 * 1024 * 45},
			},
		}
		tester := ui.NewTester(a.view, 1180, 760)
		tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
		save(tester, "dashboard.png")

		tester.SetDark(true)
		save(tester, "dashboard-dark.png")
	}

	// 7. Settings
	{
		a := nativeTestApp(t)
		a.connections = []store.Profile{p}
		a.section = sectionSettings
		tester := ui.NewTester(a.view, 1180, 760)
		tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
		save(tester, "settings.png")
	}
}
