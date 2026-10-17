package main

import "github.com/egoist/mygo/ui"

// The palette Bucketeer draws with, following Apple macOS Human Interface
// Guidelines with high-clarity typography, subtle layered surfaces, and
// semantic file-type accent colors.

func bucketeerTheme(dark bool) *ui.Theme {
	if dark {
		return darkTheme()
	}
	return lightTheme()
}

func lightTheme() *ui.Theme {
	t := ui.LightTheme()
	t.Background = ui.Hex("#f5f5f7")
	t.Surface = ui.Hex("#ffffff")
	t.SurfaceHover = ui.Hex("#f0f1f4")
	t.SurfacePressed = ui.Hex("#e5e7eb")
	t.Border = ui.Hex("#e4e4e8")
	t.Text = ui.Hex("#1c1c1e")
	t.TextMuted = ui.Hex("#6e6e73")
	t.Accent = ui.Hex("#0071e3")
	t.AccentHover = ui.Hex("#0077ed")
	t.AccentPressed = ui.Hex("#005bb5")
	t.AccentText = ui.Hex("#ffffff")
	t.Danger = ui.Hex("#ff3b30")
	t.Warning = ui.Hex("#ff9500")
	t.Success = ui.Hex("#34c759")
	t.Selection = ui.RGBA(0, 113, 227, 0.16)
	t.Focus = ui.RGBA(0, 113, 227, 0.45)
	t.Scrollbar = ui.RGBA(0, 0, 0, 0.20)
	t.Radius = 10
	t.Spacing = 4
	return t
}

func darkTheme() *ui.Theme {
	t := ui.DarkTheme()
	t.Background = ui.Hex("#161618")
	t.Surface = ui.Hex("#202024")
	t.SurfaceHover = ui.Hex("#2b2b30")
	t.SurfacePressed = ui.Hex("#37373e")
	t.Border = ui.Hex("#2d2d32")
	t.Text = ui.Hex("#f5f5f7")
	t.TextMuted = ui.Hex("#9c9ca4")
	t.Accent = ui.Hex("#0a84ff")
	t.AccentHover = ui.Hex("#2e95ff")
	t.AccentPressed = ui.Hex("#0070e0")
	t.AccentText = ui.Hex("#ffffff")
	t.Danger = ui.Hex("#ff453a")
	t.Warning = ui.Hex("#ff9f0a")
	t.Success = ui.Hex("#30d158")
	t.Selection = ui.RGBA(10, 132, 255, 0.24)
	t.Focus = ui.RGBA(10, 132, 255, 0.55)
	t.Scrollbar = ui.RGBA(255, 255, 255, 0.22)
	t.Radius = 10
	t.Spacing = 4
	return t
}

/* -------------------------------------------------------------------------
   Colors for the kinds of file
   ------------------------------------------------------------------------- */

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

// kindTint returns the color that stands for a kind of file.
func kindTint(t *ui.Theme, k fileKind) ui.Color {
	switch k {
	case kindFolder:
		if t.Dark {
			return ui.Hex("#4aa3df")
		}
		return ui.Hex("#2a85c8")
	case kindImage:
		if t.Dark {
			return ui.Hex("#7d7aff")
		}
		return ui.Hex("#5856d6")
	case kindAudio:
		if t.Dark {
			return ui.Hex("#30d158")
		}
		return ui.Hex("#248a3d")
	case kindVideo:
		if t.Dark {
			return ui.Hex("#ff375f")
		}
		return ui.Hex("#d70035")
	case kindArchive:
		if t.Dark {
			return ui.Hex("#ff9f0a")
		}
		return ui.Hex("#c97800")
	case kindDocument:
		if t.Dark {
			return ui.Hex("#64d2ff")
		}
		return ui.Hex("#0071e3")
	default:
		return t.TextMuted
	}
}

// kindSoftBg returns a soft translucent background wash corresponding to the file kind.
func kindSoftBg(t *ui.Theme, k fileKind) ui.Color {
	tint := kindTint(t, k)
	if t.Dark {
		return tint.Alpha(0.14)
	}
	return tint.Alpha(0.08)
}
