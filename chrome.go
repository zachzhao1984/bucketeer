package main

import (
	"bucketeer/internal/store"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

/* -------------------------------------------------------------------------
   Painting surfaces

   A surface is a fill, a hairline, and — for the things that stand on the
   canvas rather than in it — a shadow so faint it reads as depth rather
   than as an object. Only what floats above the window gets a real one.
   ------------------------------------------------------------------------- */

// paintSurface fills a plane and draws the line that separates it from
// whatever it stands on.
func paintSurface(p *ui.Painter, r ui.Rect, radius float32, fill, line ui.Color) {
	p.Fill(r, fill, radius)
	if line.A > 0 {
		p.Stroke(r, line, radius, 1)
	}
}

// paintPlate is a control standing on the canvas: white, hairline, and
// the faintest shadow under its bottom edge.
func paintPlate(p *ui.Painter, r ui.Rect, radius float32, ch chrome, fill, line ui.Color) {
	p.Shadow(r, radius, 0, 1, 2, 0, ch.plateShadow)
	p.Fill(r, fill, radius)
	if line.A > 0 {
		p.Stroke(r, line, radius, 1)
	}
}

// paintFloating paints what sits above the window: a shadow to lift it
// off the page, the fill, and a hairline for its edge.
func paintFloating(p *ui.Painter, r ui.Rect, radius float32, ch chrome, fill ui.Color) {
	p.Shadow(r, radius, 0, 8, 28, 0, ch.floatShadow)
	p.Fill(r, fill, radius)
	p.Stroke(r, ch.hairline, radius, 1)
}

// paintMaterial fills the sidebar unless the window already shows a
// material of its own.
func paintMaterial(c *ui.Context, e ui.Element) ui.Element {
	if c.Vibrancy() {
		return e.Background(ui.Transparent)
	}
	return e.Background(chromeOf(c.Theme()).sidebar)
}

// blend lays a color over what is under it, at the given strength.
func blend(c ui.Color, amount float32) ui.Color { return c.Alpha(amount) }

// rule draws a single hairline across a row.
func rule(c *ui.Context, inset float32, color ui.Color) ui.Element {
	return ui.Box(c).Grow(1).Height(1).MarginX(inset).Shrink(0).Background(color)
}

/* -------------------------------------------------------------------------
   Buttons
   ------------------------------------------------------------------------- */

const (
	btnRadius = 8.0
	btnHeight = 32.0
	btnPadX   = 12.0
)

type buttonKind int

const (
	kindStandard buttonKind = iota
	kindBare
	kindAccent
	kindDanger
	kindInset
)

// ctlButton is a standard button: a white plate with a hairline.
func ctlButton(c *ui.Context, label string) ui.Element {
	return ctlButtonKind(c, label, kindStandard)
}

// ctlIconButtonBare is the same button without the plate, for glyphs that
// stand directly on the surface.
func ctlIconButtonBare(c *ui.Context, name string) ui.Element {
	b := ctlButtonKind(c, "", kindBare)
	return b.Size(btnHeight, btnHeight).Padding(0, 0).Label(name)
}

// ctlPrimary is the one action a screen wants done.
func ctlPrimary(c *ui.Context, label string) ui.Element {
	return ctlButtonKind(c, label, kindAccent)
}

// ctlDanger is for what cannot be undone.
func ctlDanger(c *ui.Context, label string) ui.Element {
	return ctlButtonKind(c, label, kindDanger)
}

func ctlButtonKind(c *ui.Context, label string, kind buttonKind) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)

	b := ui.ButtonBase(c).Cursor(ui.CursorPointer).AlignItems(ui.Center).Justify(ui.Center).Gap(6).
		Padding(0, btnPadX).Height(btnHeight).Radius(btnRadius).Shrink(0).
		TextColor(t.Text).FontSize(typeBody)
	if kind == kindAccent || kind == kindDanger {
		b = b.TextColor(ch.accentOnFill).FontWeight(weightMedium)
	}
	if label != "" {
		b.Children(func() { ui.Text(c, label).FontSize(typeBody).SingleLine() })
	}

	hovered, pressed := b.Hovered(), b.Pressed()

	return b.Background(ui.Transparent).Border(0, ui.Transparent).
		Draw(func(p *ui.Painter, r ui.Rect) {
			switch kind {
			case kindBare:
				if hovered {
					p.Fill(r, ch.hover, btnRadius)
				}
				if pressed {
					p.Fill(r, ch.pressed, btnRadius)
				}
			case kindAccent:
				fill := ch.accentFill
				if hovered {
					fill = ch.accentFillHov
				}
				if pressed {
					fill = ch.accentFillPrs
				}
				paintSurface(p, r, btnRadius, fill, ui.Transparent)
			case kindDanger:
				fill := ch.dangerFill
				if hovered {
					fill = ch.dangerFillHov
				}
				paintSurface(p, r, btnRadius, fill, ui.Transparent)
			case kindInset:
				fill := ch.inset
				if hovered {
					fill = ch.hover
				}
				paintSurface(p, r, btnRadius, fill, ui.Transparent)
			default:
				fill := ch.plate
				if hovered {
					fill = ch.inset
				}
				if pressed {
					fill = ch.pressed
				}
				paintPlate(p, r, btnRadius, ch, fill, ch.hairline)
			}
		})
}

/* -------------------------------------------------------------------------
   Fields
   ------------------------------------------------------------------------- */

// ctlField styles a text field the way the reference client does: a soft
// gray well with no border, which takes the accent when focused.
func ctlField(c *ui.Context, e ui.Element) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)
	return e.Background(ch.inset).Border(1, ui.Transparent).Radius(btnRadius).
		Height(btnHeight).Padding(0, 10).FontSize(typeBody)
}

// ctlFieldOnInset is a text field standing on an inset group, as the
// settings pages draw one: it takes the canvas's own color and a hairline,
// so its bounds read against the card it sits on.
func ctlFieldOnInset(c *ui.Context, e ui.Element) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)
	return e.Background(ch.canvas).Border(1, ch.hairline).Radius(btnRadius).
		Height(btnHeight).Padding(0, 10).FontSize(typeBody)
}

// ctlSearchField is the filter field: a magnifier, the text, and nothing
// else. It returns the field itself, so the caller can ask it whether
// the text changed this frame. handle, when not nil, is bound to the text
// so a shortcut can put the caret in it.
func ctlSearchField(c *ui.Context, key any, value *string, width float32, label string, handle *ui.Handle) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)

	var field ui.Element
	row := ui.Row(c.Key(key)).Width(width).Height(btnHeight).Shrink(0).
		Radius(btnRadius).Padding(0, 9).Gap(7).AlignItems(ui.Center).
		Background(ch.inset).Border(0, ui.Transparent)
	row.Label(label)
	row.Children(func() {
		ui.Icon(c, iconSearch).FontSize(14).TextColor(t.TextMuted).Shrink(0)
		input := ui.TextInputBase(c, value).Placeholder("Filter…").FontSize(typeBody)
		if handle != nil {
			input.Bind(handle)
		}
		input.Grow(1).Shrink(1)
		field = input
	})
	return field
}

/* -------------------------------------------------------------------------
   Menus
   ------------------------------------------------------------------------- */

// ctlMenuButton is a button that drops a menu: a label, its value, and a
// chevron. The caller owns the popover, so the menu can be any shape.
func ctlMenuButton(c *ui.Context, label, value string, open bool, width float32) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)

	b := ui.ButtonBase(c).Width(width).Height(btnHeight).Shrink(0).
		Padding(0, 9).Radius(btnRadius).Gap(6).AlignItems(ui.Center).
		Justify(ui.SpaceBetween).Cursor(ui.CursorPointer).TextColor(t.Text)
	b.Label(label)

	hovered, pressed := b.Hovered(), b.Pressed()
	b.Children(func() {
		ui.Text(c, value).FontSize(typeBody).SingleLine().Ellipsis("…").Grow(1)
		ui.Icon(c, iconChevronDown).FontSize(12).TextColor(t.TextMuted).Shrink(0)
	})

	return b.Background(ui.Transparent).Border(0, ui.Transparent).
		Draw(func(p *ui.Painter, r ui.Rect) {
			fill := ch.plate
			switch {
			case open || pressed:
				fill = ch.inset
			case hovered:
				fill = ch.inset
			}
			paintPlate(p, r, btnRadius, ch, fill, ch.hairline)
		})
}

// ctlMenuPanel is the surface a menu drops on: one floating plate, the
// items inside it, and the room around them.
func ctlMenuPanel(c *ui.Context, width float32) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)

	panel := ui.Column(c).Width(width).Padding(5).Gap(1).Radius(10)
	return panel.Background(ui.Transparent).Border(0, ui.Transparent).
		Draw(func(p *ui.Painter, r ui.Rect) {
			paintFloating(p, r, 10, ch, ch.plate)
		})
}

// menuItem is one row of a popover menu. checked draws a check on the
// left, detail a second line, and glyph the mark beside the label.
func menuItem(c *ui.Context, label, glyph string, checked bool, detail string) bool {
	t := c.Theme()
	ch := chromeOf(t)

	row := ui.Row(c.Key(label+detail)).Padding(6, 8).Gap(8).AlignItems(ui.Center).
		Radius(6).Cursor(ui.CursorPointer).Focusable().TextColor(t.Text)
	row.Label(label)

	hovered := row.Hovered()
	clicked := row.Clicked()
	row.Children(func() {
		if checked {
			ui.Icon(c, iconCheck).FontSize(13).TextColor(t.Accent).Shrink(0).Width(14)
		} else if glyph != "" {
			ui.Icon(c, iconFor(glyph)).FontSize(14).TextColor(t.TextMuted).Shrink(0).Width(14)
		}
		if detail == "" {
			ui.Text(c, label).FontSize(typeBody).Grow(1).SingleLine().Ellipsis("…")
		} else {
			ui.Column(c).Grow(1).Gap(1).Children(func() {
				ui.Text(c, label).FontSize(typeBody).SingleLine().Ellipsis("…")
				ui.Text(c, detail).FontSize(typeCaption).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
			})
		}
	})

	row.Background(ui.Transparent).Border(0, ui.Transparent).
		Draw(func(p *ui.Painter, r ui.Rect) {
			if hovered {
				p.Fill(r, ch.hover, 6)
			}
		})
	return clicked
}

// menuSeparator is the hairline between two groups of a menu.
func menuSeparator(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(4, 8).Children(func() {
		rule(c, 0, t.Border)
	})
}

// menuHeading labels a group inside a menu.
func menuHeading(c *ui.Context, title string) {
	t := c.Theme()
	ui.Text(c, strings.ToUpper(title)).
		FontSize(typeMicro).
		FontWeight(weightSemi).
		LetterSpacing(0.5).
		TextColor(t.TextMuted).
		Padding(6, 8, 2, 8)
}

func divider(c *ui.Context) {
	menuSeparator(c)
}

/* -------------------------------------------------------------------------
   Navigation
   ------------------------------------------------------------------------- */

// navRow is a row of the sidebar. The chosen one is a filled accent bar
// with white text, as the reference client draws it.
func navRow(c *ui.Context, id, label string, glyph *ui.SVG, chosen bool, extra func(), onChoose func()) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)

	row := ui.ButtonBase(c.Key(id)).Cursor(ui.CursorPointer).AlignItems(ui.Center).Justify(ui.Start).Gap(11).
		FillWidth().Padding(0, 11).Radius(9).Shrink(0).Height(38)
	row.Label(label)
	if onChoose != nil {
		row.OnClick(onChoose)
	}
	hovered := row.Hovered()
	if chosen {
		row.TextColor(ch.selectedOn)
	} else {
		row.TextColor(t.Text)
	}
	row.Children(func() {
		if glyph != nil {
			color := t.TextMuted
			if chosen {
				color = ch.selectedOn
			}
			// One slot of the same width for every glyph keeps the labels of a
			// list on a single left edge.
			slot := ui.Box(c).Width(22).Shrink(0).Center()
			slot.Children(func() {
				ui.Icon(c, glyph).FontSize(19).TextColor(color)
			})
		}
		ui.Text(c, label).FontSize(typeNav).FontWeight(weightMedium).Grow(1).SingleLine().Ellipsis("…")
		if extra != nil {
			extra()
		}
	})

	return row.Background(ui.Transparent).Border(0, ui.Transparent).
		Draw(func(p *ui.Painter, r ui.Rect) {
			switch {
			case chosen:
				p.Fill(r, ch.selected, 9)
			case hovered:
				p.Fill(r, ch.hover, 9)
			}
		})
}

// sidebarLabel heads a group in the sidebar.
func sidebarLabel(c *ui.Context, title string) {
	t := c.Theme()
	ui.Text(c, title).
		FontSize(typeSmall).
		TextColor(t.TextMuted).
		Padding(10, 12, 4, 12).
		SingleLine().
		Ellipsis("…")
}

/* -------------------------------------------------------------------------
   Readouts
   ------------------------------------------------------------------------- */

// ctlCheckGlyph is the bare checkbox a table row and its header carry:
// no label, no layout of its own.
func ctlCheckGlyph(c *ui.Context, key any, on bool, label string) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)

	btn := ui.ButtonBase(c.Key(key)).Size(18, 18).Shrink(0).Center().Radius(5).
		Cursor(ui.CursorPointer).Label(label)
	hovered := btn.Hovered()
	btn.Children(func() {
		box := ui.Box(c).Size(16, 16).Radius(4.5).Center().Background(ui.Transparent)
		box.Draw(func(p *ui.Painter, r ui.Rect) {
			switch {
			case on:
				p.Fill(r, t.Accent, 4.5)
			case hovered:
				p.Stroke(r, t.TextMuted, 4.5, 1.5)
			default:
				p.Stroke(r, ch.hairline, 4.5, 1.5)
			}
		})
		box.Children(func() {
			if on {
				ui.Icon(c, iconCheck).FontSize(11).TextColor(ch.accentOnFill)
			}
		})
	})
	return btn
}

// ctlSwitch is a switch: a track with a knob that slides across it.
func ctlSwitch(c *ui.Context, key any, on *bool, label string) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)

	sw := ui.SwitchBase(c.Key(key), on).Size(38, 22).Radius(11).Shrink(0)
	if label != "" {
		sw.Label(label)
	}
	var target float32
	if *on {
		target = 1
	}
	pos := sw.Animate("knob", target, 160*time.Millisecond)

	sw.Draw(func(p *ui.Painter, r ui.Rect) {
		track := ch.inset
		if pos > 0.01 {
			track = blend(t.Accent, pos)
		}
		p.Fill(r, track, r.H/2)

		inset := float32(2)
		d := r.H - 2*inset
		knob := ui.Rect{X: r.X + inset + pos*(r.W-d-2*inset), Y: r.Y + inset, W: d, H: d}
		p.Shadow(knob, d/2, 0, 1, 2, 0, ch.floatShadow)
		p.Fill(knob, ui.Hex("#FFFFFF"), d/2)
	})
	return sw
}

// ctlSegment is one key of a segmented control: the chosen one sits on
// the channel as a plate of its own.
func ctlSegment(c *ui.Context, on *bool, label string) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)

	seg := ui.ToggleBase(c, on).Center().Padding(0, 9).Height(26).Radius(6).Shrink(0)
	seg.Label(label)
	chosen := *on
	hovered := seg.Hovered()
	if chosen {
		seg.TextColor(t.Text)
	} else {
		seg.TextColor(t.TextMuted)
	}

	return seg.Background(ui.Transparent).Border(0, ui.Transparent).
		Draw(func(p *ui.Painter, r ui.Rect) {
			switch {
			case chosen:
				p.Shadow(r, 6, 0, 1, 2, 0, ch.plateShadow)
				p.Fill(r, ch.plate, 6)
			case hovered:
				p.Fill(r, ch.hover, 6)
			}
		})
}

// ctlSegmentTrack is the channel a segmented control's keys sit in.
func ctlSegmentTrack(c *ui.Context) ui.Element {
	ch := chromeOf(c.Theme())
	track := ui.ToggleGroup(c, func() {}).Padding(2).Radius(8).Gap(0).
		Background(ui.Transparent).Border(0, ui.Transparent)
	return track.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, ch.inset, 8)
	})
}

// ctlBadge is a quiet tag: a count or a class name set beside a label.
func ctlBadge(c *ui.Context, text string, tint ui.Color) ui.Element {
	t := c.Theme()
	b := ui.Box(c).Padding(1.5, 7).Radius(5).Shrink(0).Background(chromeOf(t).inset)
	b.Children(func() {
		ui.Text(c, text).FontSize(typeCaption).FontWeight(weightMedium).TextColor(tint)
	})
	return b
}

// ctlChip is the tag a table's Type column carries: a soft pill.
func ctlChip(c *ui.Context, text string, tint ui.Color) ui.Element {
	b := ui.Box(c).Padding(1, 7).Radius(4).Shrink(0).Background(tint.Alpha(0.10))
	b.Children(func() {
		ui.Text(c, text).FontSize(typeCaption).TextColor(tint)
	})
	return b
}

// ctlMeter is a channel with a bar of accent in it, for a share of a
// whole.
func ctlMeter(c *ui.Context, value float64) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)

	track := ch.inset
	if t.Dark {
		// On warm black the inset slot would vanish; the road must read.
		track = ui.RGBA(255, 255, 255, 0.12)
	}
	e := ui.Box(c).Height(6).Shrink(0).Background(ui.Transparent).Border(0, ui.Transparent)
	return e.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, track, 3)
		v := float32(min(max(value, 0), 1))
		if v <= 0 {
			return
		}
		bar := ui.Rect{X: r.X, Y: r.Y, W: max(r.H, r.W*v), H: r.H}
		if t.Dark {
			// The night shift: progress carries its own light.
			p.Shadow(bar, 3, 0, 0, 9, 2, t.Accent.Alpha(0.45))
		}
		p.Fill(bar, t.Accent, 3)
	})
}

// led is a small dot of a kind's color, for beside a status.
func led(c *ui.Context, color ui.Color, size float32) ui.Element {
	e := ui.Box(c).Size(size, size).Shrink(0).Background(ui.Transparent)
	return e.Draw(func(p *ui.Painter, r ui.Rect) {
		if c.Theme().Dark {
			p.Shadow(r, size/2, 0, 0, 6, 2, color.Alpha(0.55))
		}
		p.Fill(r, color, size/2)
	})
}

// iconPlate is the rounded square a glyph stands on: the mark of a kind,
// or of a figure on the dashboard.
func iconPlate(c *ui.Context, glyph *ui.SVG, tint ui.Color, size float32) ui.Element {
	plate := ui.Box(c).Size(size, size).Radius(size*0.28).Center().Shrink(0).
		Background(ui.Transparent).Border(0, ui.Transparent)
	plate.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, tint, size*0.28)
	})
	plate.Children(func() {
		ui.Icon(c, glyph).FontSize(size * 0.52).TextColor(ui.Hex("#FFFFFF"))
	})
	return plate
}

// monogram is a round tag of a name's initials.
// connectionMark draws a connection's mark: the service's own logo on a
// quiet plate when the app knows the service, the drive plate with
// initials otherwise.
func connectionMark(c *ui.Context, profile store.Profile, size float32) ui.Element {
	svg := brandIcon(profile.Provider)
	if svg == nil {
		return drivePlate(c, profile.Name, size)
	}
	tint := brandTint(profile.Provider)

	plate := ui.Box(c).Size(size, size).Radius(size * 0.30).Shrink(0).Center().Clip()
	plate.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, tint.Alpha(0.14), size*0.30)
		p.Stroke(r, tint.Alpha(0.45), size*0.30, 1)
	})
	plate.Children(func() {
		ui.Icon(c, svg).FontSize(size * 0.60)
	})
	return plate
}

// drivePlate is the connection's mark: a small drive with a warm
// anodized face, a highlight along its top edge, and four screws. It is
// the one deliberately physical thing in the window — a bucket is a
// place things live.
func drivePlate(c *ui.Context, name string, size float32) ui.Element {
	e := ui.Box(c).Size(size, size).Radius(size * 0.30).Shrink(0).Center().Clip()
	rad := size * 0.30
	screw := max(2, size*0.075)
	off := size * 0.17
	e.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, ui.Hex("#17A574"), rad)
		p.Fill(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H * 0.46}, ui.RGBA(255, 255, 255, 0.28), rad)
		for _, pt := range [][2]float32{
			{off, off}, {r.W - off, off}, {off, r.H - off}, {r.W - off, r.H - off},
		} {
			p.Fill(ui.Rect{X: r.X + pt[0] - screw/2, Y: r.Y + pt[1] - screw/2, W: screw, H: screw},
				ui.RGBA(4, 40, 28, 0.55), screw/2)
		}
	})
	e.Children(func() {
		ui.Text(c, initialsOf(name)).FontSize(size * 0.38).FontWeight(weightSemi).TextColor(ui.Hex("#FFFFFF"))
	})
	return e
}

func monogram(c *ui.Context, name string, size float32) ui.Element {
	t := c.Theme()
	e := ui.Box(c).Size(size, size).Radius(size * 0.30).Shrink(0).Center().Clip()
	e = e.Background(t.Accent).Border(0, ui.Transparent)
	e.Children(func() {
		ui.Text(c, initialsOf(name)).FontSize(size * 0.40).FontWeight(weightSemi).TextColor(ui.Hex("#FFFFFF"))
	})
	return e
}

func initialsOf(name string) string {
	fields := strings.Fields(name)
	var b strings.Builder
	for i, f := range fields {
		if i == 2 {
			break
		}
		for _, r := range f {
			b.WriteRune(r)
			break
		}
	}
	if b.Len() == 0 && name != "" {
		return strings.ToUpper(name[:1])
	}
	return strings.ToUpper(b.String())
}

/* -------------------------------------------------------------------------
   Cards, dialogs
   ------------------------------------------------------------------------- */

// card is a white plate standing on the canvas: the dashboard's figures,
// the settings' groups, the transfers' dock.
func card(c *ui.Context) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)
	e := ui.Column(c).Background(ui.Transparent).Border(0, ui.Transparent).Radius(12)
	return e.Draw(func(p *ui.Painter, r ui.Rect) {
		paintPlate(p, r, 12, ch, ch.plate, ch.hairline)
	})
}

// ctlDialog is the panel of a modal: the one surface with a real shadow,
// because it genuinely floats.
func ctlDialog(c *ui.Context, open *bool, width float32, fn func()) {
	t := c.Theme()
	ch := chromeOf(t)
	_, winH := c.Size()

	ui.DialogBase(c, open, func(back, panel ui.Element) {
		back.Background(ch.scrim)
		panel.Width(width).MaxHeight(winH - 48).Padding(20).Gap(14).Radius(14)
		panel = panel.Background(ui.Transparent).Border(0, ui.Transparent)
		panel.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Shadow(r, 14, 0, 20, 60, 0, ch.floatShadow)
			p.Fill(r, ch.plate, 14)
			p.Stroke(r, ch.hairline, 14, 1)
		})
		fn()
	})
}

// statusBanner is the one-line result of trying something.
func statusBanner(c *ui.Context, wash ui.Color, glyph *ui.SVG, message string, textColor, glyphColor ui.Color) {
	row := ui.Row(c).
		FillWidth().
		Padding(8, 10).
		Radius(btnRadius).
		Gap(8).
		AlignItems(ui.Start).
		Background(wash).
		Border(0, ui.Transparent)
	row.Children(func() {
		ui.Icon(c, glyph).FontSize(14).TextColor(glyphColor).Shrink(0).Margin(0, 1)
		ui.Text(c, message).FontSize(typeCaption).Grow(1).TextColor(textColor)
	})
}
