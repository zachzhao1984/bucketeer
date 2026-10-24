package main

import (
	"image/png"
	"os"
	"testing"

	"bucketeer/internal/store"
	"github.com/egoist/mygo/ui"
)

func TestABCShots2(t *testing.T) {
	os.MkdirAll("/tmp/abc", 0o755)
	build := func(style string) {
		folderCardStyle = style
		a := nativeTestApp(t)
		p := store.Profile{ID: "demo", Name: "R2 主力", Endpoint: "https://s3.example.com", Bucket: "media"}
		a.session = SessionInfo{Connected: true, Profile: &p, Host: "s3.example.com"}
		a.bucket = "media"
		a.section = sectionFiles
		a.rootFolders = []store.Entry{{Key: "photos/", Name: "photos", Folder: true}}
		a.prefix = "photos/"
		a.listing = store.Listing{Items: []store.Entry{
			{Key: "photos/2026-10/", Name: "2026-10", Folder: true},
			{Key: "photos/2026-09/", Name: "2026-09", Folder: true},
			{Key: "photos/2026-08/", Name: "2026-08", Folder: true},
			{Key: "photos/IMG_2417.jpg", Name: "IMG_2417.jpg", Size: 3565158},
			{Key: "photos/IMG_2418.jpg", Name: "IMG_2418.jpg", Size: 3041280},
		}}
		a.sel.Add("photos/2026-09/")
		te := ui.NewTester(a.view, 1180, 820)
		te.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
		te.SetDark(true)
		te.Frame()
		f, _ := os.Create("/tmp/abc/" + style + "-dark2.png")
		png.Encode(f, te.Image())
		f.Close()
	}
	build("A")
	build("B")
	build("C")
}
