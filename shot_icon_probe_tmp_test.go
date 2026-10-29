package main

import (
	"image/png"
	"os"
	"testing"

	"bucketeer/internal/store"
	"github.com/egoist/mygo/ui"
)

func TestIconVsImage(t *testing.T) {
	a := nativeTestApp(t)
	_ = a
	te := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Fill().Center().Gap(30).Children(func() {
			ui.Icon(c, brandIcon(store.ProviderR2)).FontSize(48)
			ui.Image(c, brandIcon(store.ProviderR2)).Size(48, 48)
			ui.Icon(c, brandIcon(store.ProviderAWS)).FontSize(48)
			ui.Image(c, brandIcon(store.ProviderAWS)).Size(48, 48)
		})
	}, 420, 160)
	te.SetDark(true)
	te.Frame()
	f, _ := os.Create("/tmp/icon-probe.png")
	png.Encode(f, te.Image())
	f.Close()
}
