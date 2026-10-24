package main

import "github.com/egoist/mygo/ui"

// The palette Bucketeer draws with, taken from the reference client.
//
// The window is two planes: a light sidebar, and a canvas for content.
// Controls are white plates with a hairline and a shadow so faint it only
// shows on a light page. One accent — a strong blue — marks the chosen
// place and the one action a screen wants. Nothing is bevelled, nothing
// is textured.

func bucketeerTheme(dark bool) *ui.Theme {
	if dark {
		return darkTheme()
	}
	return lightTheme()
}

func lightTheme() *ui.Theme {
	t := ui.LightTheme()
	t.Background = ui.Hex("#F7F7F7") // behind everything: the sidebar
	t.Surface = ui.Hex("#FFFFFF")    // the content canvas
	t.SurfaceHover = ui.Hex("#F4F4F5")
	t.SurfacePressed = ui.Hex("#EBEBED")
	t.Border = ui.Hex("#E4E4E7") // a hairline
	t.Text = ui.Hex("#1D1D1F")
	t.TextMuted = ui.Hex("#6C6C71")
	t.Accent = ui.Hex("#3163F4")
	t.AccentHover = ui.Hex("#2A57DC")
	t.AccentPressed = ui.Hex("#244BC4")
	t.AccentText = ui.Hex("#FFFFFF")
	t.Danger = ui.Hex("#D93025")
	t.Warning = ui.Hex("#B25000")
	t.Success = ui.Hex("#188038")
	t.Selection = ui.Hex("#E8EEFE")
	t.Focus = ui.RGBA(49, 99, 244, 0.45)
	t.Scrollbar = ui.RGBA(0, 0, 0, 0.20)
	t.Inverse = ui.Hex("#1D1D1F")
	t.InverseText = ui.Hex("#FFFFFF")
	t.Radius = 7
	t.Spacing = 4
	return t
}

func darkTheme() *ui.Theme {
	t := ui.DarkTheme()
	t.Background = ui.Hex("#1E1E20")
	t.Surface = ui.Hex("#161618")
	t.SurfaceHover = ui.Hex("#26262A")
	t.SurfacePressed = ui.Hex("#303036")
	t.Border = ui.Hex("#38383C")
	t.Text = ui.Hex("#F5F5F7")
	t.TextMuted = ui.Hex("#98989F")
	t.Accent = ui.Hex("#3E6DF6")
	t.AccentHover = ui.Hex("#527CF8")
	t.AccentPressed = ui.Hex("#325EE0")
	t.AccentText = ui.Hex("#FFFFFF")
	t.Danger = ui.Hex("#FF6B60")
	t.Warning = ui.Hex("#FFB340")
	t.Success = ui.Hex("#3DD68C")
	t.Selection = ui.Hex("#22314F")
	t.Focus = ui.RGBA(62, 109, 246, 0.60)
	t.Scrollbar = ui.RGBA(255, 255, 255, 0.22)
	t.Inverse = ui.Hex("#F5F5F7")
	t.InverseText = ui.Hex("#1D1D1F")
	t.Radius = 7
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
			sidebar: ui.Hex("#1E1E20"),
			canvas:  ui.Hex("#161618"),
			plate:   ui.Hex("#232326"),
			inset:   ui.Hex("#26262A"),
			well:    ui.Hex("#232326"),

			hairline:      ui.Hex("#38383C"),
			hairlineFaint: ui.Hex("#2A2A2E"),

			hover:   ui.RGBA(255, 255, 255, 0.07),
			pressed: ui.RGBA(255, 255, 255, 0.04),

			selected:   ui.Hex("#3E6DF6"),
			selectedOn: ui.Hex("#FFFFFF"),
			accentOn:   ui.Hex("#7FA2FF"),

			floatShadow: ui.RGBA(0, 0, 0, 0.55),
			plateShadow: ui.RGBA(0, 0, 0, 0.35),

			accentFill:    ui.Hex("#3E6DF6"),
			accentFillHov: ui.Hex("#527CF8"),
			accentFillPrs: ui.Hex("#325EE0"),
			accentOnFill:  ui.Hex("#FFFFFF"),

			dangerFill:    ui.Hex("#C9342A"),
			dangerFillHov: ui.Hex("#DC4136"),
			dangerOnFill:  ui.Hex("#FFFFFF"),

			scrim: ui.RGBA(0, 0, 0, 0.55),
		}
	}
	return chrome{
		sidebar: ui.Hex("#F7F7F7"),
		canvas:  ui.Hex("#FFFFFF"),
		plate:   ui.Hex("#FFFFFF"),
		inset:   ui.Hex("#F2F2F4"),
		well:    ui.Hex("#F2F2F4"),

		hairline:      ui.Hex("#E4E4E7"),
		hairlineFaint: ui.Hex("#EFEFF1"),

		hover:   ui.RGBA(0, 0, 0, 0.045),
		pressed: ui.RGBA(0, 0, 0, 0.085),

		selected:   ui.Hex("#3163F4"),
		selectedOn: ui.Hex("#FFFFFF"),
		accentOn:   ui.Hex("#3163F4"),

		floatShadow: ui.RGBA(0, 0, 0, 0.16),
		plateShadow: ui.RGBA(0, 0, 0, 0.05),

		accentFill:    ui.Hex("#3163F4"),
		accentFillHov: ui.Hex("#2A57DC"),
		accentFillPrs: ui.Hex("#244BC4"),
		accentOnFill:  ui.Hex("#FFFFFF"),

		dangerFill:    ui.Hex("#D93025"),
		dangerFillHov: ui.Hex("#BE2A20"),
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
