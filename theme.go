package main

import "github.com/egoist/mygo/ui"

// The palette Bucketeer draws with.
//
// The interface is two planes. The functional plane — the sidebar and the
// bar along the top — is a material: one quiet color, separated from what
// is under it by a hairline and nothing else. The content plane is the
// canvas files and figures sit on. Depth is spent only where something
// genuinely floats above the window: menus, popovers, dialogs.
//
// Nothing here is a bevel. A surface is a fill, a hairline, and the text
// on it; the light in this app comes from the window it sits in, not from
// a lamp drawn on top of every control.

func bucketeerTheme(dark bool) *ui.Theme {
	if dark {
		return darkTheme()
	}
	return lightTheme()
}

func lightTheme() *ui.Theme {
	t := ui.LightTheme()
	t.Background = ui.Hex("#E7E7EB") // behind everything
	t.Surface = ui.Hex("#FFFFFF")    // the canvas
	t.SurfaceHover = ui.Hex("#F1F1F4")
	t.SurfacePressed = ui.Hex("#E6E6EA")
	t.Border = ui.Hex("#D3D3D9") // a hairline
	t.Text = ui.Hex("#1C1C1E")   // 17.0:1 on the canvas
	t.TextMuted = ui.Hex("#5C5C66")
	t.Accent = ui.Hex("#0A6EDB") // 4.9:1 on the canvas
	t.AccentHover = ui.Hex("#0966C6")
	t.AccentPressed = ui.Hex("#0857B0")
	t.AccentText = ui.Hex("#FFFFFF")
	t.Danger = ui.Hex("#A8201A")
	t.Warning = ui.Hex("#7A4E00")
	t.Success = ui.Hex("#186A38")
	t.Selection = ui.Hex("#DEEBFB")
	t.Focus = ui.RGBA(10, 110, 219, 0.55)
	t.Scrollbar = ui.RGBA(0, 0, 0, 0.26)
	t.Inverse = ui.Hex("#2C2C2E")
	t.InverseText = ui.Hex("#FFFFFF")
	t.Radius = 6
	t.Spacing = 4
	return t
}

func darkTheme() *ui.Theme {
	t := ui.DarkTheme()
	t.Background = ui.Hex("#121214")
	t.Surface = ui.Hex("#1B1B1E") // the canvas
	t.SurfaceHover = ui.Hex("#28282C")
	t.SurfacePressed = ui.Hex("#101012")
	t.Border = ui.Hex("#3A3A41")
	t.Text = ui.Hex("#F5F5F7") // 15.8:1 on the canvas
	t.TextMuted = ui.Hex("#A0A0A8")
	t.Accent = ui.Hex("#7CB8FF")
	t.AccentHover = ui.Hex("#93C6FF")
	t.AccentPressed = ui.Hex("#5FA8F5")
	t.AccentText = ui.Hex("#0A1A2E")
	t.Danger = ui.Hex("#FF6961")
	t.Warning = ui.Hex("#FFC53D")
	t.Success = ui.Hex("#3DD68C")
	t.Selection = ui.Hex("#1E3A5C")
	t.Focus = ui.RGBA(124, 184, 255, 0.65)
	t.Scrollbar = ui.RGBA(255, 255, 255, 0.26)
	t.Inverse = ui.Hex("#F2F2F5")
	t.InverseText = ui.Hex("#1C1C1E")
	t.Radius = 6
	t.Spacing = 4
	return t
}

/* -------------------------------------------------------------------------
   The surfaces chrome.go paints with
   ------------------------------------------------------------------------- */

// chrome is the set of surfaces the app draws on, and the marks that
// separate them. Every name here is a plane of the interface rather than
// an object standing on another one.
type chrome struct {
	// The material the sidebar and the top bar share.
	material ui.Color
	// The canvas the content sits on.
	canvas ui.Color
	// A recessed plane: table headers, summary bars, grouped settings.
	inset ui.Color
	// A field that takes typing.
	field ui.Color

	// The marks between planes.
	hairline      ui.Color // a single quiet rule
	hairlineFaint ui.Color // the same rule, for a long list
	hairlineEdge  ui.Color // the rule under the material

	// Washes for hover, selection and pressed, laid over whatever is
	// under them.
	hover      ui.Color
	pressed    ui.Color
	selected   ui.Color
	selectedOn ui.Color // text on a selected row

	// Shadow, for the things that truly float: menus, popovers, dialogs.
	floatShadow ui.Color

	// Accent and danger as filled buttons, and their own labels.
	accentFill    ui.Color
	accentFillHov ui.Color
	accentFillPrs ui.Color
	accentOnFill  ui.Color

	dangerFill    ui.Color
	dangerFillHov ui.Color
	dangerOnFill  ui.Color

	scrim ui.Color
}

// blend lays color over what is under it, at the given strength.
func blend(c ui.Color, amount float32) ui.Color {
	return c.Alpha(amount)
}

func chromeOf(t *ui.Theme) chrome {
	if t.Dark {
		return chrome{
			material: ui.Hex("#28282C"),
			canvas:   ui.Hex("#1B1B1E"),
			inset:    ui.Hex("#232327"),
			field:    ui.Hex("#141416"),

			hairline:      ui.Hex("#3A3A41"),
			hairlineFaint: ui.Hex("#2E2E34"),
			hairlineEdge:  ui.Hex("#3A3A41"),

			hover:      ui.RGBA(255, 255, 255, 0.06),
			pressed:    ui.RGBA(255, 255, 255, 0.03),
			selected:   ui.Hex("#1E3A5C"),
			selectedOn: ui.Hex("#F5F5F7"),

			floatShadow: ui.RGBA(0, 0, 0, 0.55),

			accentFill:    ui.Hex("#0A84FF"),
			accentFillHov: ui.Hex("#3D9BFF"),
			accentFillPrs: ui.Hex("#0072E6"),
			accentOnFill:  ui.Hex("#FFFFFF"),

			dangerFill:    ui.Hex("#C9342A"),
			dangerFillHov: ui.Hex("#DC4136"),
			dangerOnFill:  ui.Hex("#FFFFFF"),

			scrim: ui.RGBA(0, 0, 0, 0.45),
		}
	}
	return chrome{
		material: ui.Hex("#EAEAEE"),
		canvas:   ui.Hex("#FFFFFF"),
		inset:    ui.Hex("#F4F4F6"),
		field:    ui.Hex("#FFFFFF"),

		hairline:      ui.Hex("#D3D3D9"),
		hairlineFaint: ui.Hex("#E4E4E9"),
		hairlineEdge:  ui.Hex("#D3D3D9"),

		hover:      ui.RGBA(0, 0, 0, 0.045),
		pressed:    ui.RGBA(0, 0, 0, 0.09),
		selected:   ui.Hex("#DEEBFB"),
		selectedOn: ui.Hex("#1C1C1E"),

		floatShadow: ui.RGBA(0, 0, 0, 0.20),

		accentFill:    ui.Hex("#0064CE"), // 5.8:1 under its white label
		accentFillHov: ui.Hex("#0057B4"),
		accentFillPrs: ui.Hex("#004A9C"),
		accentOnFill:  ui.Hex("#FFFFFF"),

		dangerFill:    ui.Hex("#A8201A"),
		dangerFillHov: ui.Hex("#8E1A15"),
		dangerOnFill:  ui.Hex("#FFFFFF"),

		scrim: ui.RGBA(0, 0, 0, 0.28),
	}
}

/* -------------------------------------------------------------------------
   The type scale

   Four sizes carry the whole interface. Anything that is not one of them
   is a mistake waiting to happen.
   ------------------------------------------------------------------------- */

const (
	typeCaption  = 11.0 // counts, hints, column headings
	typeBody     = 13.0 // everything a person reads
	typeHeadline = 15.0 // dialog titles, section headings
	typeTitle    = 20.0 // the bucket on the dashboard
)

// Weights, named so that a reader can tell at a glance whether a size
// is being asked to carry hierarchy on its own or with a heavier stroke.
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

// kindTint returns the color that stands for a kind of file. Each one is
// at least 4.5:1 against the canvas, because a glyph may be the only
// thing telling two kinds of file apart.
func kindTint(t *ui.Theme, k fileKind) ui.Color {
	switch k {
	case kindFolder:
		if t.Dark {
			return ui.Hex("#5FB3EE")
		}
		return ui.Hex("#1E7BBD")
	case kindImage:
		if t.Dark {
			return ui.Hex("#A99BF0")
		}
		return ui.Hex("#6B4FBF")
	case kindAudio:
		if t.Dark {
			return ui.Hex("#4FC98C")
		}
		return ui.Hex("#177E4F")
	case kindVideo:
		if t.Dark {
			return ui.Hex("#FF7A90")
		}
		return ui.Hex("#BE3A56")
	case kindArchive:
		if t.Dark {
			return ui.Hex("#E0A94A")
		}
		return ui.Hex("#9A6206")
	case kindDocument:
		if t.Dark {
			return ui.Hex("#7FC3DC")
		}
		return ui.Hex("#2F6B85")
	default:
		return t.TextMuted
	}
}

// kindWash returns the faintest wash of a kind's color, for the small
// plate a glyph sits on. It carries no information of its own.
func kindWash(t *ui.Theme, k fileKind) ui.Color {
	if k == kindFolder {
		return ui.Transparent
	}
	if t.Dark {
		return kindTint(t, k).Alpha(0.16)
	}
	return kindTint(t, k).Alpha(0.10)
}
