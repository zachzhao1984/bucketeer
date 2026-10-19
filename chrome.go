package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

/* -------------------------------------------------------------------------
   Painting surfaces

   A surface in this app is a fill and, where it needs to be told apart
   from what is next to it, a hairline. Only the things that float above
   the window — menus, popovers, dialogs — get a shadow.
   ------------------------------------------------------------------------- */

// paintSurface fills a plane and draws the line that separates it from
// whatever it stands on.
func paintSurface(p *ui.Painter, r ui.Rect, radius float32, fill, line ui.Color) {
	p.Fill(r, fill, radius)
	if line.A > 0 {
		p.Stroke(r, line, radius, 1)
	}
}

// paintFloating paints what sits above the window: a shadow to lift it
// off the page, the fill, and a hairline for its edge.
func paintFloating(p *ui.Painter, r ui.Rect, radius float32, ch chrome, fill ui.Color) {
	p.Shadow(r, radius, 0, 6, 24, 0, ch.floatShadow)
	p.Fill(r, fill, radius)
	p.Stroke(r, ch.hairline, radius, 1)
}

// paintMaterial fills the sidebar and the top bar, unless the window is
// already showing a material of its own — then we leave the space empty
// and let the system paint it, so the bar stays translucent over
// whatever is behind the window.
func paintMaterial(c *ui.Context, e ui.Element) ui.Element {
	if c.Vibrancy() {
		return e.Background(ui.Transparent)
	}
	return e.Background(chromeOf(c.Theme()).material)
}

// rule draws a single hairline across a row, inset from its edges.
func rule(c *ui.Context, inset float32, color ui.Color) ui.Element {
	return ui.Box(c).Grow(1).Height(1).MarginX(inset).Shrink(0).Background(color)
}

/* -------------------------------------------------------------------------
   Buttons
   ------------------------------------------------------------------------- */

const (
	btnRadius = 6.0
	btnPadY   = 5.0
	btnPadX   = 11.0
)

// ctlButton is a standard button: the face of the page, a hairline, and
// a quiet wash when the pointer is over it.
func ctlButton(c *ui.Context, label string) ui.Element {
	return ctlButtonKind(c, label, kindStandard)
}

// ctlIconButton is a square button for a glyph, named for assistive
// technology and for the tests.
func ctlIconButton(c *ui.Context, name string) ui.Element {
	b := ctlButtonKind(c, "", kindStandard)
	b = b.Size(26, 26).Padding(0, 0).Radius(btnRadius)
	return b.Label(name)
}

// ctlIconButtonBare is the same button without the standard fill, for
// the icons that stand directly on the material.
func ctlIconButtonBare(c *ui.Context, name string) ui.Element {
	b := ctlButtonKind(c, "", kindBare)
	b = b.Size(26, 26).Padding(0, 0).Radius(btnRadius)
	return b.Label(name)
}

// ctlPrimary is the one action a screen wants done.
func ctlPrimary(c *ui.Context, label string) ui.Element {
	return ctlButtonKind(c, label, kindAccent)
}

// ctlDanger is for what cannot be undone.
func ctlDanger(c *ui.Context, label string) ui.Element {
	return ctlButtonKind(c, label, kindDanger)
}

type buttonKind int

const (
	kindStandard buttonKind = iota
	kindBare
	kindAccent
	kindDanger
)

func ctlButtonKind(c *ui.Context, label string, kind buttonKind) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)

	b := ui.ButtonBase(c).Cursor(ui.CursorPointer).AlignItems(ui.Center).Gap(5).
		Padding(btnPadY, btnPadX).Radius(btnRadius).Shrink(0).
		TextColor(t.Text)
	if kind == kindAccent || kind == kindDanger {
		b = b.TextColor(ch.accentOnFill).FontWeight(weightSemi)
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
			default:
				fill := t.Surface
				if hovered {
					fill = ch.hover
				}
				if pressed {
					fill = ch.pressed
				}
				paintSurface(p, r, btnRadius, fill, ch.hairline)
			}
		})
}

// ctlLink is a name that does something, until the pointer is over it.
func ctlLink(c *ui.Context, label string) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)

	e := ui.ButtonBase(c).Cursor(ui.CursorPointer).AlignItems(ui.Center).Justify(ui.Start).
		Padding(3, 5).Radius(5).Shrink(0).TextColor(t.Accent)
	hovered := e.Hovered()
	e.Children(func() { ui.Text(c, label).FontSize(typeBody).SingleLine().Ellipsis("…") })
	return e.Background(ui.Transparent).Border(0, ui.Transparent).
		Draw(func(p *ui.Painter, r ui.Rect) {
			if hovered {
				p.Fill(r, ch.hover, 5)
			}
		})
}

/* -------------------------------------------------------------------------
   Fields
   ------------------------------------------------------------------------- */

// ctlField sinks a text field or a search field into the page: a flat
// well with a hairline around it.
func ctlField(c *ui.Context, e ui.Element) ui.Element {
	ch := chromeOf(c.Theme())
	return e.Background(ch.field).Border(1, ch.hairline).Radius(btnRadius)
}

// ctlSelect styles a drop-down as a button, and its list as the menu it
// drops out of.
func ctlSelect(c *ui.Context, key any, selected *string, options []string, width float32) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)

	parts := ui.SelectBase(c.Key(key), selected)
	trigger := parts.Trigger
	trigger.Width(width).Padding(5, 9).Radius(btnRadius).Shrink(0).
		Justify(ui.SpaceBetween).Gap(8).
		Background(ui.Transparent).Border(0, ui.Transparent).TextColor(t.Text)
	trigger.Children(func() {
		ui.Text(c, *selected).SingleLine().FontSize(typeBody)
		ui.Icon(c, iconChevronDown).FontSize(11).TextColor(t.TextMuted).Shrink(0)
	})
	hovered, pressed := trigger.Hovered(), trigger.Pressed()
	trigger.Draw(func(p *ui.Painter, r ui.Rect) {
		fill := t.Surface
		if hovered {
			fill = ch.hover
		}
		if pressed {
			fill = ch.pressed
		}
		paintSurface(p, r, btnRadius, fill, ch.hairline)
	})

	parts.Popup(func(panel ui.Element) {
		panel.Padding(4)
		panel = panel.Background(ui.Transparent).Border(0, ui.Transparent)
		panel.Draw(func(p *ui.Painter, r ui.Rect) {
			paintFloating(p, r, 8, ch, t.Surface)
		})
		for _, opt := range options {
			option := opt
			item := parts.Item(option).Padding(5, 8).Radius(5)
			highlighted := item.Highlighted()
			switch {
			case highlighted:
				item.Background(ch.selected).TextColor(ch.selectedOn)
			case option == *selected:
				item.Background(t.Surface).TextColor(t.Text)
			}
			item.Children(func() {
				ui.Text(c, option).FontSize(typeBody).SingleLine()
			})
		}
	})
	return trigger
}

/* -------------------------------------------------------------------------
   Navigation
   ------------------------------------------------------------------------- */

// navRow is a row of the sidebar: text that stays quiet until it is the
// place the app is looking, or the pointer is over it.
func navRow(c *ui.Context, id, label string, icon *ui.SVG, chosen bool, extra func(), onChoose func()) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)

	row := ui.ButtonBase(c).Key(id).Cursor(ui.CursorPointer).AlignItems(ui.Center).Gap(8).
		Padding(4, 8).Radius(btnRadius).Shrink(0).Height(28)
	if onChoose != nil {
		row.OnClick(onChoose)
	}
	hovered := row.Hovered()
	if chosen {
		row.TextColor(ch.selectedOn).FontWeight(weightSemi)
	} else {
		row.TextColor(t.Text)
	}
	row.Children(func() {
		if icon != nil {
			color := t.TextMuted
			if chosen {
				color = t.Accent
			}
			ui.Icon(c, icon).FontSize(15).TextColor(color).Shrink(0)
		}
		ui.Text(c, label).FontSize(typeBody).Grow(1).SingleLine()
		if extra != nil {
			extra()
		}
	})

	return row.Background(ui.Transparent).Border(0, ui.Transparent).
		Draw(func(p *ui.Painter, r ui.Rect) {
			switch {
			case chosen:
				paintSurface(p, r, btnRadius, ch.selected, ui.Transparent)
			case hovered:
				p.Fill(r, ch.hover, btnRadius)
			}
		})
}

/* -------------------------------------------------------------------------
   Readouts
   ------------------------------------------------------------------------- */

// ctlCheck is a check box: a small well that a check fills with accent.
func ctlCheck(c *ui.Context, key any, on *bool, label string) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)

	row := ui.CheckboxBase(c.Key(key), on).FocusRing(false).Gap(8).AlignItems(ui.Center).Shrink(0)
	checked := *on
	hovered := row.Hovered()

	row.Children(func() {
		box := ui.Box(c).Size(16, 16).Radius(4).Shrink(0).Center().Background(ui.Transparent)
		box.Draw(func(p *ui.Painter, r ui.Rect) {
			switch {
			case checked:
				paintSurface(p, r, 4, t.Accent, ui.Transparent)
			case hovered:
				paintSurface(p, r, 4, ch.hover, t.Border)
			default:
				paintSurface(p, r, 4, t.Surface, t.Border)
			}
			if row.FocusVisible() {
				p.FocusRing(r, [4]float32{4, 4, 4, 4})
			}
		})
		box.Children(func() {
			if checked {
				ui.Icon(c, iconCheck).FontSize(11).TextColor(ch.accentOnFill)
			}
		})
		if label != "" {
			ui.Text(c, label).FontSize(typeBody)
		}
	})
	return row
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
		track := ch.field
		if pos > 0.01 {
			track = blend(t.Accent, pos)
		}
		p.Fill(r, track, r.H/2)
		p.Stroke(r, ch.hairline, r.H/2, 1)

		inset := float32(2)
		d := r.H - 2*inset
		knob := ui.Rect{X: r.X + inset + pos*(r.W-d-2*inset), Y: r.Y + inset, W: d, H: d}
		p.Shadow(knob, d/2, 0, 1, 2, 0, ch.floatShadow)
		p.Fill(knob, ui.Hex("#FFFFFF"), d/2)
	})
	return sw
}

// ctlSegment is one key of a pair inside a channel: the chosen one sits
// on the channel as a plate of its own, the other stays flush.
func ctlSegment(c *ui.Context, on *bool, label string) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)

	seg := ui.ToggleBase(c, on).Center().Padding(5, 9).Radius(5).Shrink(0)
	seg.Label(label)
	chosen := *on
	hovered := seg.Hovered()
	if chosen {
		seg.TextColor(t.Text).FontWeight(weightSemi)
	} else {
		seg.TextColor(t.TextMuted)
	}

	return seg.Background(ui.Transparent).Border(0, ui.Transparent).
		Draw(func(p *ui.Painter, r ui.Rect) {
			switch {
			case chosen:
				paintSurface(p, r, 5, t.Surface, ui.Transparent)
			case hovered:
				p.Fill(r, ch.hover, 5)
			}
		})
}

// ctlBadge is a quiet tag: a count or a class name set beside a label.
func ctlBadge(c *ui.Context, text string, tint ui.Color) ui.Element {
	t := c.Theme()
	b := ui.Box(c).Padding(1, 6).Radius(4).Shrink(0).Background(chromeOf(t).inset)
	b.Children(func() {
		ui.Text(c, text).FontSize(typeCaption).FontWeight(weightSemi).TextColor(tint)
	})
	return b
}

// ctlMeter is a channel with a bar of accent in it, for a share of a
// whole.
func ctlMeter(c *ui.Context, value float64) ui.Element {
	t := c.Theme()
	ch := chromeOf(t)

	e := ui.Box(c).Height(6).Shrink(0).Background(ui.Transparent).Border(0, ui.Transparent)
	return e.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, ch.hover, 3)
		v := float32(min(max(value, 0), 1))
		if v <= 0 {
			return
		}
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: max(r.H, r.W*v), H: r.H}, t.Accent, 3)
	})
}

// led is a small dot of a kind's color, for beside a status.
func led(c *ui.Context, color ui.Color, size float32) ui.Element {
	e := ui.Box(c).Size(size, size).Shrink(0).Background(ui.Transparent)
	return e.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, color, size/2)
	})
}

// monogram is a round tag of a name's initials.
func monogram(c *ui.Context, name string, size float32) ui.Element {
	t := c.Theme()
	e := ui.Box(c).Size(size, size).Radius(size / 2).Shrink(0).Center().Clip()
	e = e.Background(chromeOf(t).canvas).Border(1, t.Border)
	e.Children(func() {
		ui.Text(c, initialsOf(name)).FontSize(size * 0.38).FontWeight(weightSemi).TextColor(t.TextMuted)
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
   Panels and dialogs
   ------------------------------------------------------------------------- */

// panelHeading labels a section of a page: the name, and the room
// around it.
func panelHeading(c *ui.Context, title string) {
	t := c.Theme()
	ui.Row(c).Gap(9).AlignItems(ui.Center).Children(func() {
		ui.Text(c, title).FontSize(typeBody).FontWeight(weightSemi).TextColor(t.Text)
	})
}

// ctlDialog is the panel of a modal: the one surface in the app with a
// real shadow, because it genuinely floats.
func ctlDialog(c *ui.Context, open *bool, width float32, fn func()) {
	t := c.Theme()
	ch := chromeOf(t)
	_, winH := c.Size()

	ui.DialogBase(c, open, func(back, panel ui.Element) {
		back.Background(ch.scrim)
		panel.Width(width).MaxHeight(winH - 48).Padding(20).Gap(14).Radius(12)
		panel = panel.Background(ui.Transparent).Border(0, ui.Transparent)
		panel.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Shadow(r, 12, 0, 18, 48, 0, ch.floatShadow)
			p.Fill(r, t.Surface, 12)
			p.Stroke(r, ch.hairline, 12, 1)
		})
		fn()
	})
}
