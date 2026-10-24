package main

import "github.com/egoist/mygo/ui"

// The palette Bucketeer draws with, taken from the reference client.
//
// The window is two planes: a sidebar, and a canvas for content. The
// palette is Midnight: deep warm surfaces, one emerald accent, hierarchy
// carried by weight rather than color, and progress that glows — the
// night-shift workbench. The light appearance is its daylight: warm
// paper, the same emerald, the same quiet. The connection's mark is the
// one deliberately physical thing in the window, because a bucket is a
// place things live.

func bucketeerTheme(dark bool) *ui.Theme {
	if dark {
		return darkTheme()
	}
	return lightTheme()
}

func lightTheme() *ui.Theme {
	t := ui.LightTheme()
	t.Background = ui.Hex("#F1EFE7") // behind everything: the sidebar
	t.Surface = ui.Hex("#FBFAF6")    // the content canvas
	t.SurfaceHover = ui.Hex("#F3F2EA")
	t.SurfacePressed = ui.Hex("#EAE8DD")
	t.Border = ui.Hex("#E0DED2") // a hairline
	t.Text = ui.Hex("#1F2421")
	t.TextMuted = ui.Hex("#5C6158")
	t.Accent = ui.Hex("#0A7D58")
	t.AccentHover = ui.Hex("#086B4B")
	t.AccentPressed = ui.Hex("#065A3F")
	t.AccentText = ui.Hex("#FFFFFF")
	t.Danger = ui.Hex("#C53727")
	t.Warning = ui.Hex("#9A5B00")
	t.Success = ui.Hex("#0A7D58")
	t.Selection = ui.Hex("#D7F0E3")
	t.Focus = ui.RGBA(10, 125, 88, 0.40)
	t.Scrollbar = ui.RGBA(31, 36, 33, 0.20)
	t.Inverse = ui.Hex("#1F2421")
	t.InverseText = ui.Hex("#FBFAF6")
	t.Radius = 8
	t.Spacing = 4
	return t
}

func darkTheme() *ui.Theme {
	t := ui.DarkTheme()
	t.Background = ui.Hex("#1A1714") // behind everything: the sidebar
	t.Surface = ui.Hex("#151312")    // the content canvas
	t.SurfaceHover = ui.Hex("#26221E")
	t.SurfacePressed = ui.Hex("#2D2925")
	t.Border = ui.Hex("#2E2A25")
	t.Text = ui.Hex("#EDE7DC")
	t.TextMuted = ui.Hex("#A69E8F")
	t.Accent = ui.Hex("#34D399")
	t.AccentHover = ui.Hex("#4AE0A8")
	t.AccentPressed = ui.Hex("#2BC485")
	t.AccentText = ui.Hex("#0B241A")
	t.Danger = ui.Hex("#FF6B60")
	t.Warning = ui.Hex("#FFB340")
	t.Success = ui.Hex("#34D399")
	t.Selection = ui.Hex("#1F3A2C")
	t.Focus = ui.RGBA(52, 211, 153, 0.50)
	t.Scrollbar = ui.RGBA(232, 228, 218, 0.22)
	t.Inverse = ui.Hex("#EDE7DC")
	t.InverseText = ui.Hex("#151312")
	t.Radius = 8
	t.Spacing = 4
	return t
}

/* -------------------------------------------------------------------------
   chrome is the set of surfaces the app draws on
   ------------------------------------------------------------------------- */

type chrome struct {
	// sidebar is the plane the navigation sits on, behind the window's
	// own controls; canvas is the plane files and figures sit on.
	sidebar ui.Color
	canvas  ui.Color
	// plate is the face of a control standing on the canvas: a white
	// card with a hairline and a shadow faint enough to read as depth.
	plate ui.Color
	// inset is a recessed plane: table headers, summary bands, a
	// thumbnail's plate, a field that takes typing.
	inset ui.Color
	// well is the fill of a text field.
	well ui.Color

	hairline      ui.Color
	hairlineFaint ui.Color

	hover   ui.Color
	pressed ui.Color

	// selected is the fill of the chosen row, and selectedOn its label.
	selected   ui.Color
	selectedOn ui.Color
	// accentOn is a glyph or a count drawn in the accent.
	accentOn ui.Color

	floatShadow ui.Color
	plateShadow ui.Color

	accentFill    ui.Color
	accentFillHov ui.Color
	accentFillPrs ui.Color
	accentOnFill  ui.Color

	dangerFill    ui.Color
	dangerFillHov ui.Color
	dangerOnFill  ui.Color

	scrim ui.Color
}

func chromeOf(t *ui.Theme) chrome {
	if t.Dark {
		return chrome{
			sidebar: ui.Hex("#1A1714"),
			canvas:  ui.Hex("#151312"),
			plate:   ui.Hex("#1F1C19"),
			inset:   ui.Hex("#211E1A"),
			well:    ui.Hex("#1F1C19"),

			hairline:      ui.Hex("#2E2A25"),
			hairlineFaint: ui.Hex("#262320"),

			hover:   ui.RGBA(255, 255, 255, 0.05),
			pressed: ui.RGBA(255, 255, 255, 0.03),

			selected:   ui.Hex("#34D399"),
			selectedOn: ui.Hex("#0B241A"),
			accentOn:   ui.Hex("#6EE7B7"),

			floatShadow: ui.RGBA(0, 0, 0, 0.50),
			plateShadow: ui.RGBA(0, 0, 0, 0.30),

			accentFill:    ui.Hex("#34D399"),
			accentFillHov: ui.Hex("#4AE0A8"),
			accentFillPrs: ui.Hex("#2BC485"),
			accentOnFill:  ui.Hex("#0B241A"),

			dangerFill:    ui.Hex("#C9342A"),
			dangerFillHov: ui.Hex("#DC4136"),
			dangerOnFill:  ui.Hex("#FFFFFF"),

			scrim: ui.RGBA(0, 0, 0, 0.55),
		}
	}
	return chrome{
		sidebar: ui.Hex("#F1EFE7"),
		canvas:  ui.Hex("#FBFAF6"),
		plate:   ui.Hex("#FFFFFF"),
		inset:   ui.Hex("#F3F2EA"),
		well:    ui.Hex("#F3F2EA"),

		hairline:      ui.Hex("#E0DED2"),
		hairlineFaint: ui.Hex("#ECEAE0"),

		hover:   ui.RGBA(31, 36, 33, 0.045),
		pressed: ui.RGBA(31, 36, 33, 0.085),

		selected:   ui.Hex("#0A7D58"),
		selectedOn: ui.Hex("#FFFFFF"),
		accentOn:   ui.Hex("#0A7D58"),

		floatShadow: ui.RGBA(31, 36, 33, 0.16),
		plateShadow: ui.RGBA(31, 36, 33, 0.05),

		accentFill:    ui.Hex("#0A7D58"),
		accentFillHov: ui.Hex("#086B4B"),
		accentFillPrs: ui.Hex("#065A3F"),
		accentOnFill:  ui.Hex("#FFFFFF"),

		dangerFill:    ui.Hex("#C53727"),
		dangerFillHov: ui.Hex("#A82D20"),
		dangerOnFill:  ui.Hex("#FFFFFF"),

		scrim: ui.RGBA(0, 0, 0, 0.22),
	}
}

/* -------------------------------------------------------------------------
   The type scale
   ------------------------------------------------------------------------- */

const (
	typeMicro    = 10.0 // a badge, a key hint
	typeCaption  = 11.5 // counts, hints, column headings
	typeSmall    = 12.0 // the second line of a row
	typeBody     = 13.0 // everything a person reads
	typeNav      = 14.0 // the sidebar's rows
	typeHeadline = 15.0 // dialog titles, section headings
	typeTitle    = 20.0 // the place being browsed
	typeDisplay  = 26.0 // a figure on the dashboard
)

const (
	weightMedium = 500
	weightSemi   = 600
)

// monoFamily is the face data reads in: sizes, speeds, percentages.
const monoFamily = "Menlo"

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

// kindTint returns the color that stands for a kind of file: the color of
// its plate, and of its glyph.
func kindTint(t *ui.Theme, k fileKind) ui.Color {
	switch k {
	case kindFolder:
		if t.Dark {
			return ui.Hex("#5B8DFF")
		}
		return ui.Hex("#3B7DF6")
	case kindImage:
		if t.Dark {
			return ui.Hex("#B18CFF")
		}
		return ui.Hex("#8B5CF6")
	case kindAudio:
		if t.Dark {
			return ui.Hex("#4FC98C")
		}
		return ui.Hex("#16A34A")
	case kindVideo:
		if t.Dark {
			return ui.Hex("#FF8A80")
		}
		return ui.Hex("#EF5350")
	case kindArchive:
		if t.Dark {
			return ui.Hex("#E0A94A")
		}
		return ui.Hex("#F59E0B")
	case kindDocument:
		if t.Dark {
			return ui.Hex("#6FC3E8")
		}
		return ui.Hex("#0EA5E9")
	default:
		return t.TextMuted
	}
}

// kindWash is the plate a kind's glyph sits on: the kind's color at a
// strength that stays quiet behind a white glyph.
func kindWash(t *ui.Theme, k fileKind) ui.Color {
	if t.Dark {
		return kindTint(t, k).Alpha(0.18)
	}
	return kindTint(t, k).Alpha(0.10)
}
