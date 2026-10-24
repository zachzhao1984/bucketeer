package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
)

// The grid: cards wide enough for a picture and a name. The picture is the
// object itself when it is an image, and otherwise the mark of its kind on
// a soft plate of the kind's color.
const (
	cardWidth  = 148.0
	cardHeight = 140.0
	cardPlate  = 88.0
)

// files draws the contents of the folder being browsed, or the buckets of
// the endpoint when no bucket is open.
func (a *App) files(c *ui.Context) {
	if !a.browsing() {
		a.bucketsView(c)
		return
	}
	if a.loading && len(a.source().Items) == 0 {
		skeletons(c)
		return
	}
	items := a.visible()
	if len(items) == 0 {
		a.filesEmpty(c)
		return
	}
	if a.mode == viewGrid {
		a.gridView(c, items)
		return
	}
	a.tableView(c, items)
}

func (a *App) filesEmpty(c *ui.Context) {
	switch {
	case a.searchResults != nil:
		emptyState(c, iconSearch, "No matches",
			fmt.Sprintf("No object matching “%s” was found in %s.", a.search, a.bucket), func() {
				if ctlPrimary(c, "Clear search").Clicked() {
					a.clearSearch()
				}
			})
	case a.filter != "all":
		emptyState(c, iconOther, "Nothing of this type",
			"This folder holds no objects matching the selected filter.", func() {
				if ctlPrimary(c, "Show all types").Clicked() {
					a.filter = "all"
				}
			})
	case a.prefix != "":
		emptyState(c, iconFolder, "This folder is empty",
			"Drop files here, or use Upload to add objects.", func() {
				if ctlPrimary(c, "Upload files").Clicked() {
					a.upload()
				}
				if ctlButton(c, "New folder").Clicked() {
					a.openNewFolder()
				}
			})
	default:
		emptyState(c, iconBucket, "This bucket is empty",
			"No objects have been uploaded to this bucket yet.", func() {
				if ctlPrimary(c, "Upload files").Clicked() {
					a.upload()
				}
			})
	}
}

/* -------------------------------------------------------------------------
   The grid of cards
   ------------------------------------------------------------------------- */

func (a *App) gridView(c *ui.Context, items []store.Entry) {
	a.grid.Key = func(i int) any { return items[i].Key }
	a.grid.Label = func(i int) string { return items[i].Name }
	a.grid.Selection = &a.sel
	a.grid.Selected = &a.cursor

	g := ui.GridView(c, &a.grid, len(items), cardWidth, cardHeight, func(i int) {
		a.card(c, items[i])
	}).Grow(1).Padding(18)

	if a.cursor >= 0 && a.cursor < len(items) && g.Submitted() {
		a.openEntry(items[a.cursor])
	}
}

// card is one thing in the bucket: its picture, its name, and what it is.
func (a *App) card(c *ui.Context, e store.Entry) {
	if e.Folder {
		a.folderCard(c, e)
		return
	}
	t := c.Theme()
	ch := chromeOf(t)
	chosen := a.sel.Has(e.Key)
	kind := kindOf(e.Key, e.Folder)

	upload, uploading := a.activeTransferFor(e.Key)

	card := ui.Column(c.Key(e.Key)).
		Fill().
		Padding(6).
		Gap(4).
		Radius(10).
		Cursor(ui.CursorPointer).
		TextColor(t.Text).
		Background(ui.Transparent).
		Border(0, ui.Transparent)
	card.Label(e.Name)

	hovered := card.Hovered()
	card.ContextMenu(func(m *ui.Menu) { a.entryMenu(m, e) })

	card.Draw(func(p *ui.Painter, r ui.Rect) {
		switch {
		case chosen:
			p.Fill(r, ch.selected.Alpha(0.10), 10)
			p.Stroke(r, ch.selected, 10, 1.5)
		case hovered:
			p.Fill(r, ch.hover, 10)
		}
		if uploading {
			// The object is in flight: an orange edge and its progress
			// along the card's bottom, so the grid shows the queue.
			p.Stroke(r, ch.accentFill, 10, 1.5)
			frac := float32(1)
			if upload.Size > 0 {
				frac = float32(upload.Done) / float32(upload.Size)
			}
			p.Fill(ui.Rect{X: r.X + 1, Y: r.Y + r.H - 4, W: (r.W - 2) * frac, H: 3}, ch.accentFill, 1.5)
		}
	})

	card.Children(func() {
		a.thumbnail(c, e, kind)

		ui.Text(c, e.Name).
			FontSize(typeBody).
			FontWeight(weightMedium).
			TextColor(t.Text).
			TextAlign(ui.Center).
			SingleLine().
			Ellipsis("…").
			FillWidth().
			Label(e.Name)
	})
}

// folderCard draws a folder in the grid: the file card's own anatomy —
// a tall inset plate with a large line glyph, the name below — so the
// two kinds share one shape and differ only in their mark. Chosen is the
// file card's language too: an emerald wash and a hairline, with the
// glyph turning emerald. The cell is flooded with the canvas color
// first, covering the chrome the grid paints for a chosen cell.
func (a *App) folderCard(c *ui.Context, e store.Entry) {
	t := c.Theme()
	ch := chromeOf(t)
	chosen := a.sel.Has(e.Key)

	card := ui.Column(c.Key(e.Key)).Fill().Padding(6).Gap(4).Radius(10).
		Cursor(ui.CursorPointer).TextColor(t.Text).Background(ui.Transparent).
		Border(0, ui.Transparent)
	card.Label(e.Name)
	hovered := card.Hovered()
	card.ContextMenu(func(m *ui.Menu) { a.entryMenu(m, e) })

	glyph := t.TextMuted
	if chosen {
		glyph = ch.accentOn
	}
	card.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, ch.canvas, 10)
		switch {
		case chosen:
			p.Fill(r, ch.selected.Alpha(0.12), 10)
		case hovered:
			p.Fill(r, ch.hover, 10)
		}
	})
	card.DrawOver(func(p *ui.Painter, r ui.Rect) {
		if chosen {
			p.Stroke(r, ch.selected, 10, 1.5)
		}
	})
	card.Children(func() {
		zone := ui.Box(c).Fill().Center().Radius(8)
		zone.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(r, ch.inset, 8)
		})
		zone.Children(func() {
			ui.Icon(c, iconFolder).FontSize(46).TextColor(glyph)
			if chosen || hovered {
				mark := ui.Box(c).Absolute().Top(8).Left(8)
				mark.Children(func() { a.cellCheck(c, e) })
			}
		})
		ui.Text(c, e.Name).FontSize(typeBody).FontWeight(weightMedium).
			TextColor(t.Text).TextAlign(ui.Center).SingleLine().Ellipsis("…").FillWidth()
	})
}

// thumbnail is the picture on a card: the object itself when the app has
// decoded it, and otherwise the mark of its kind, drawn large enough to
// tell two kinds apart at a glance.
func (a *App) thumbnail(c *ui.Context, e store.Entry, kind fileKind) {
	t := c.Theme()
	ch := chromeOf(t)

	plate := ui.Box(c).
		FillWidth().
		Height(cardPlate).
		Radius(8).
		Clip().
		Center().
		Background(ui.Transparent).
		Border(0, ui.Transparent)

	if bmp := a.thumbs[e.Key]; bmp != nil {
		plate.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(r, ch.inset, 8)
		})
		plate.Children(func() {
			ui.Image(c, bmp).Fit(ui.Cover).Fill()
		})
		return
	}

	tint := kindTint(t, kind)
	plate.Children(func() {
		switch {
		case a.thumbLoading[e.Key]:
			ui.Spinner(c).Label("Loading " + e.Name)
		case e.Folder:
			ui.Icon(c, iconFolderFilled).FontSize(60).TextColor(tint)
		default:
			ui.Icon(c, entryIcon(e)).FontSize(52).TextColor(tint)
		}
	})
}

/* -------------------------------------------------------------------------
   The list
   ------------------------------------------------------------------------- */

// listColumns are the columns of the table: the name, what it is, its
// size, and when it changed.
var listColumns = []ui.TableColumn{
	{Title: "Name", ID: string(sortName), MinWidth: 200, Sortable: true},
	{Title: "Type", ID: "kind", Width: 132},
	{Title: "Size", ID: string(sortSize), Width: 96, Align: ui.End, Sortable: true},
	{Title: "Modified", ID: string(sortModified), Width: 140, Align: ui.End, Sortable: true},
	{Title: "", ID: "actions", Width: 40, Fixed: true},
}

func (a *App) tableView(c *ui.Context, items []store.Entry) {
	a.fileList.Key = func(i int) any { return items[i].Key }
	a.fileList.Label = func(i int) string { return items[i].Name }
	a.fileList.Selection = &a.sel
	a.fileList.Selected = &a.cursor
	a.fileList.Sort = &a.tableSort

	table := ui.Table(c, &a.fileList, listColumns, len(items), func(row, col int) {
		if row < 0 || row >= len(items) {
			return
		}
		a.cell(c, items[row], col)
	}).Grow(1).Padding(0, 18)

	if a.cursor >= 0 && a.cursor < len(items) && table.Submitted() {
		a.openEntry(items[a.cursor])
	}
}

func (a *App) cell(c *ui.Context, e store.Entry, col int) {
	t := c.Theme()
	kind := kindOf(e.Key, e.Folder)
	switch col {
	case 0:
		ui.Row(c).Gap(9).AlignItems(ui.Center).Padding(0, 2).Children(func() {
			a.cellCheck(c, e)
			a.cellMark(c, e, kind, 24)
			ui.Text(c, e.Name).FontSize(typeBody).TextColor(t.Text).Grow(1).SingleLine().Ellipsis("…")
		})
	case 1:
		ui.Row(c).Padding(0, 2).Children(func() {
			ctlChip(c, typeChip(e), t.TextMuted)
		})
	case 2:
		ui.Text(c, sizeText(e)).
			FontSize(typeBody).
			TextColor(t.TextMuted).
			SingleLine().
			Padding(0, 2)
	case 3:
		ui.Text(c, modifiedText(e)).
			FontSize(typeBody).
			TextColor(t.TextMuted).
			SingleLine().
			Padding(0, 2)
	case 4:
		a.rowActions(c, e)
	}
}

// rowActions is the menu a row carries at its end: the same actions the
// context menu holds, reachable without a right click.
func (a *App) rowActions(c *ui.Context, e store.Entry) {
	t := c.Theme()
	ch := chromeOf(t)

	open := a.rowMenuKey == e.Key && a.rowMenuOpen
	btn := ui.ButtonBase(c.Key("actions:"+e.Key)).Size(28, 28).Shrink(0).
		Radius(6).Center().Cursor(ui.CursorPointer).Label("Actions for " + e.Name)
	hovered := btn.Hovered()
	btn.Children(func() {
		ui.Icon(c, iconMore).FontSize(15).TextColor(t.TextMuted)
	})
	if btn.Clicked() {
		a.rowMenuKey = e.Key
		a.rowMenuOpen = !open
	}

	btn.Background(ui.Transparent).Border(0, ui.Transparent).
		Draw(func(p *ui.Painter, r ui.Rect) {
			if hovered || open {
				p.Fill(r, ch.hover, 6)
			}
		})

	ui.Popover(c, btn, &a.rowMenuOpen, func() {
		ctlMenuPanel(c, 196).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				a.rowMenuItems(c, e, &a.rowMenuOpen)
			})
		})
	})
}

// cellCheck is the row's checkbox: drawn only when the row is hovered or
// chosen, so a quiet list stays quiet.
func (a *App) cellCheck(c *ui.Context, e store.Entry) {
	chosen := a.sel.Has(e.Key)
	box := ctlCheckGlyph(c, e.Key, chosen, "Select "+e.Name)
	if box.Clicked() {
		if chosen {
			a.sel.Remove(e.Key)
		} else {
			a.sel.Add(e.Key)
		}
	}
}

// cellMark is the small plate beside a name in the list.
func (a *App) cellMark(c *ui.Context, e store.Entry, kind fileKind, size float32) {
	t := c.Theme()
	plate := ui.Box(c).Size(size, size).Radius(size*0.26).Center().Shrink(0).
		Background(ui.Transparent).Border(0, ui.Transparent)
	plate.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, kindWash(t, kind), size*0.26)
	})
	plate.Children(func() {
		ui.Icon(c, entryIcon(e)).FontSize(size * 0.64).TextColor(kindTint(t, kind))
	})
}

// typeChip is what the Type column says: a folder or a file, as the
// reference client names them. The mark beside the name carries the kind.
func typeChip(e store.Entry) string {
	if e.Folder {
		return "Folder"
	}
	return "File"
}

func sizeText(e store.Entry) string {
	if e.Folder {
		return "—"
	}
	return formatBytes(e.Size)
}

func modifiedText(e store.Entry) string {
	if e.LastModified.IsZero() {
		return "—"
	}
	return formatRelative(e.LastModified, now())
}

/* -------------------------------------------------------------------------
   Buckets
   ------------------------------------------------------------------------- */

func (a *App) bucketsView(c *ui.Context) {
	t := c.Theme()

	switch {
	case a.loading && len(a.buckets) == 0:
		skeletons(c)

	case len(a.buckets) == 0:
		emptyState(c, iconBucket, "No buckets available",
			"The connection answered, but no buckets are accessible with these credentials.", func() {
				if ctlPrimary(c, "New bucket").Clicked() {
					a.openNewBucket()
				}
				if ctlButton(c, "Refresh").Clicked() {
					a.loadBuckets()
				}
			})

	default:
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Padding(18).Gap(2).Children(func() {
				for _, b := range a.buckets {
					a.bucketRow(c, b)
				}
			})
		})
	}
	_ = t
}

func (a *App) bucketRow(c *ui.Context, b store.Bucket) {
	t := c.Theme()
	ch := chromeOf(t)

	row := ui.Row(c.Key(b.Name)).
		FillWidth().
		Height(52).
		Padding(0, 10).
		Gap(11).
		AlignItems(ui.Center).
		Radius(9).
		Cursor(ui.CursorPointer).
		TextColor(t.Text).
		Background(ui.Transparent).
		Border(0, ui.Transparent).
		Focusable().
		OnClick(func() { a.openBucket(b.Name) })
	row.Label(b.Name)

	hovered := row.Hovered()
	row.ContextMenu(func(m *ui.Menu) { a.bucketMenu(m, b) })
	row.Draw(func(p *ui.Painter, r ui.Rect) {
		if hovered {
			p.Fill(r, ch.hover, 9)
		}
	})

	row.Children(func() {
		iconPlate(c, iconBucket, kindTint(t, kindFolder), 32)

		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, b.Name).FontSize(typeBody).FontWeight(weightMedium).SingleLine().Ellipsis("…")
			ui.Text(c, createdText(b)).FontSize(typeCaption).TextColor(t.TextMuted).SingleLine()
		})

		ui.Icon(c, iconChevron).FontSize(13).TextColor(t.TextMuted).Shrink(0)
	})
}

func createdText(b store.Bucket) string {
	if b.Created.IsZero() {
		return "S3 bucket"
	}
	return "Created " + formatRelative(b.Created, now())
}

/* -------------------------------------------------------------------------
   Context menus
   ------------------------------------------------------------------------- */

func (a *App) entryMenu(m *ui.Menu, e store.Entry) {
	if !a.sel.Has(e.Key) {
		a.chooseOnly(e.Key)
	}
	if !e.Folder {
		if m.Item("Preview").Shortcut(ui.Cmd, ui.KeyEnter).Chosen() {
			a.showDetail(e.Key)
		}
		if m.Item("Copy link").Chosen() {
			a.copyLink(e.Key)
		}
		if m.Item("Share…").Chosen() {
			a.openShare(e.Key)
		}
		m.Separator()
	}
	if m.Item("Download").Chosen() {
		a.download()
	}
	if m.Item("Move…").Chosen() {
		a.openMove()
	}
	if len(a.chosen()) == 1 {
		if m.Item("Rename").Shortcut(0, ui.KeyF2).Chosen() {
			a.openRename(e)
		}
	}
	m.Separator()
	if !e.Folder {
		if m.Item("Open in browser").Chosen() {
			a.openInBrowser(e.Key)
		}
		if m.Item("Save as…").Chosen() {
			a.saveAs(e.Key)
		}
		if m.Item("Copy key").Chosen() {
			a.copyText(e.Key, "Key copied")
		}
		m.Separator()
	}
	if m.Item("Delete").Shortcut(ui.Cmd, ui.KeyBackspace).Chosen() {
		a.deleteChosen()
	}
}

func (a *App) bucketMenu(m *ui.Menu, b store.Bucket) {
	if m.Item("Open").Chosen() {
		a.openBucket(b.Name)
	}
	if m.Item("Measure storage").Chosen() {
		a.bucket = b.Name
		a.prefix = ""
		a.setSection(sectionDashboard)
		a.stat = nil
		a.loadStat()
	}
	m.Separator()
	if m.Item("Copy bucket name").Chosen() {
		a.copyText(b.Name, "Name copied")
	}
	m.Separator()
	if m.Item("Delete bucket…").Chosen() {
		name := b.Name
		a.ask("Delete “"+name+"”?", "The bucket must be empty. This cannot be undone.", "Delete", func() {
			a.removeBucket(name)
		})
	}
}

// rowMenuItems is the list of things a row's Actions menu holds: the
// order the reference client uses, so the menu is learned once.
func (a *App) rowMenuItems(c *ui.Context, e store.Entry, open *bool) {
	if !e.Folder {
		if menuItem(c, "Preview", "eye", false, "") {
			*open = false
			a.showDetail(e.Key)
		}
		if menuItem(c, "Copy link", "link", false, "") {
			*open = false
			a.copyLink(e.Key)
		}
		if menuItem(c, "Share…", "share", false, "") {
			*open = false
			a.openShare(e.Key)
		}
		menuSeparator(c)
	}
	if menuItem(c, "Download", "download", false, "") {
		*open = false
		a.chooseOnly(e.Key)
		a.download()
	}
	if menuItem(c, "Move…", "move", false, "") {
		*open = false
		a.chooseOnly(e.Key)
		a.openMove()
	}
	if menuItem(c, "Rename", "rename", false, "") {
		*open = false
		a.chooseOnly(e.Key)
		a.openRename(e)
	}
	menuSeparator(c)
	if menuItem(c, "Delete", "trash", false, "") {
		*open = false
		a.chooseOnly(e.Key)
		a.deleteChosen()
	}
}
