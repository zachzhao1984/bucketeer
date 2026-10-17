package main

import "github.com/egoist/mygo/ui"

// The palette Bucketeer draws with, a neutral scale with one blue accent. The
// light and dark values are the same ones the design was built on, so the
// two appearances differ only in their colors, never in their layout.

// bucketeerTheme returns the theme for the system's appearance, derived from
// MyGo's so widgets the app does not style itself keep a look in step with
// it.
func bucketeerTheme(dark bool) *ui.Theme {
	if dark {
		return darkTheme()
	}
	return lightTheme()
}

func lightTheme() *ui.Theme {
	t := ui.LightTheme()
	t.Background = ui.Hex("#f7f7f7")
	t.Surface = ui.Hex("#ffffff")
	t.SurfaceHover = ui.Hex("#f0f0f0")
	t.SurfacePressed = ui.Hex("#e6e6e6")
	t.Border = ui.Hex("#e5e5e5")
	t.Text = ui.Hex("#0a0a0a")
	t.TextMuted = ui.Hex("#737373")
	t.Accent = ui.Hex("#2b7fff")
	t.AccentHover = ui.Hex("#1f6fe5")
	t.AccentPressed = ui.Hex("#1a5fc4")
	t.AccentText = ui.Hex("#ffffff")
	t.Danger = ui.Hex("#e7000b")
	t.Warning = ui.Hex("#c07c00")
	t.Success = ui.Hex("#00a63e")
	t.Selection = ui.RGBA(43, 127, 255, 0.22)
	t.Focus = ui.RGBA(43, 127, 255, 0.55)
	t.Scrollbar = ui.RGBA(0, 0, 0, 0.22)
	t.Radius = 10
	t.Spacing = 4
	return t
}

func darkTheme() *ui.Theme {
	t := ui.DarkTheme()
	t.Background = ui.Hex("#0f0f0f")
	t.Surface = ui.Hex("#171717")
	t.SurfaceHover = ui.Hex("#262626")
	t.SurfacePressed = ui.Hex("#333333")
	t.Border = ui.Hex("#262626")
	t.Text = ui.Hex("#fafafa")
	t.TextMuted = ui.Hex("#a3a3a3")
	t.Accent = ui.Hex("#2b7fff")
	t.AccentHover = ui.Hex("#4a91ff")
	t.AccentPressed = ui.Hex("#1f6fe5")
	t.AccentText = ui.Hex("#ffffff")
	t.Danger = ui.Hex("#ff6467")
	t.Warning = ui.Hex("#f0b100")
	t.Success = ui.Hex("#3dd68c")
	t.Selection = ui.RGBA(43, 127, 255, 0.28)
	t.Focus = ui.RGBA(74, 145, 255, 0.6)
	t.Scrollbar = ui.RGBA(255, 255, 255, 0.28)
	t.Radius = 10
	t.Spacing = 4
	return t
}

/* -------------------------------------------------------------------------
   Colors for the kinds of file
   ------------------------------------------------------------------------- */

// fileKind names what a key holds, for its icon and its color.
type fileKind string

const (
	kindFolder   fileKind = "folder"
	kindImage    fileKind = "image"
	kindAudio    fileKind = "audio"
	kindVideo    fileKind = "video"
	kindArchive  fileKind = "archive"
	kindDocument fileKind = "document"
	kindOther    fileKind = "other"
)

// kindTint returns the color that stands for a kind of file, so a grid of
// cards reads at a glance.
func kindTint(t *ui.Theme, k fileKind) ui.Color {
	switch k {
	case kindImage:
		return ui.Hex("#2b7fff")
	case kindAudio:
		return ui.Hex("#00c9a7")
	case kindVideo:
		return ui.Hex("#a855f7")
	case kindArchive:
		return ui.Hex("#f0b100")
	default:
		if t.Dark {
			return ui.Hex("#8a8a8a")
		}
		return ui.Hex("#6b6b6b")
	}
}
