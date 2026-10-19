package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
)

// The grid of cards. A card is wide enough for a name and tall enough for
// a picture; nothing else.
const (
	cardWidth   = 150.0
	cardHeight  = 152.0
	plateHeight = 96.0
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
	a.fileHeader(c, items)
	if a.mode == viewGrid {
		a.gridView(c, items)
		return
	}
	a.tableView(c, items)
}

// fileHeader is the quiet band above the files: how many there are, and
// anything the load has to say about them.
func (a *App) fileHeader(c *ui.Context, items []store.Entry) {
	t := c.Theme()
	ch := chromeOf(t)

	folders, files := 0, 0
	var bytes int64
	for _, e := range items {
		if e.Folder {
			folders++
			continue
		}
		files++
		bytes += e.Size
	}

	head := ui.Row(c).
		FillWidth().
		Height(30).
		Padding(0, 16).
		Gap(8).
		AlignItems(ui.Center).
		Background(ui.Transparent).
		Border(0, ui.Transparent)
	head.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, ch.inset, 0)
		p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - 1, W: r.W, H: 1}, ch.hairline, 0)
	})

	head.Children(func() {
		ui.Text(c, summarizeListing(folders, files, bytes)).
			FontSize(typeCaption).
			TextColor(t.TextMuted)

		if a.searchResults != nil {
			ctlBadge(c, "Deep Search", t.Accent)
		}
		if a.source().Truncated {
			ctlBadge(c, "10,000 Item Limit", t.Warning)
		}

		ui.Spacer(c)

		if a.loading {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c).Label("Loading")
				ui.Text(c, "Syncing…").FontSize(typeCaption).TextColor(t.TextMuted)
			})
		}
	})
}

func (a *App) filesEmpty(c *ui.Context) {
	switch {
	case a.searchResults != nil:
		emptyState(c, iconSearch, "No Matches",
			fmt.Sprintf("No object matching “%s” was found in %s.", a.search, a.bucket), func() {
				if ctlPrimary(c, "Clear Search").Clicked() {
					a.clearSearch()
				}
			})
	case a.filter != "all":
		emptyState(c, iconOther, "Nothing of This Type",
			"This folder holds no objects matching the selected filter.", func() {
				if ctlPrimary(c, "Show All Types").Clicked() {
					a.filter = "all"
				}
			})
	case a.prefix != "":
		emptyState(c, iconFolder, "Folder Is Empty",
			"Drop files here, or use Upload to add objects.", func() {
				if ctlPrimary(c, "Upload Files").Clicked() {
					a.upload()
				}
				if ctlButton(c, "New Folder").Clicked() {
					a.openNewFolder()
				}
			})
	default:
		emptyState(c, iconBucket, "Bucket Is Empty",
			"No objects have been uploaded to this bucket yet.", func() {
				if ctlPrimary(c, "Upload Files").Clicked() {
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
	}).Grow(1).Padding(16)

	if a.cursor >= 0 && a.cursor < len(items) && g.Submitted() {
		a.openEntry(items[a.cursor])
	}
}

// card is one thing in the bucket: its picture, its name, and enough
// about it to tell two of them apart.
func (a *App) card(c *ui.Context, e store.Entry) {
	t := c.Theme()
	ch := chromeOf(t)
	chosen := a.sel.Has(e.Key)
	kind := kindOf(e.Key, e.Folder)

	card := ui.Column(c).
		Key(e.Key).
		Fill().
		Padding(8).
		Gap(6).
		Radius(btnRadius).
		Cursor(ui.CursorPointer).
		TextColor(ch.selectedOn).
		Background(ui.Transparent).
		Border(0, ui.Transparent)

	hovered := card.Hovered()
	card.ContextMenu(func(m *ui.Menu) { a.entryMenu(m, e) })

	card.Draw(func(p *ui.Painter, r ui.Rect) {
		switch {
		case chosen:
			paintSurface(p, r, btnRadius, ch.selected, t.Accent)
		case hovered:
			paintSurface(p, r, btnRadius, ch.hover, ch.hairline)
		default:
			paintSurface(p, r, btnRadius, ui.Transparent, ch.hairlineFaint)
		}
	})

	card.Children(func() {
		a.thumbnail(c, e, kind)

		ui.Column(c).Gap(1).Padding(0, 2).Children(func() {
			ui.Text(c, e.Name).
				FontSize(typeBody).
				FontWeight(weightMedium).
				SingleLine().
				Ellipsis("…").
				Label(e.Name)
			ui.Text(c, cardDetail(e)).
				FontSize(typeCaption).
				TextColor(t.TextMuted).
				SingleLine().
				Ellipsis("…")
		})
	})
}

// thumbnail is the picture on a card: the object itself when there is
// one, and otherwise a plain plate carrying the kind's glyph. The plate
// is the same neutral for every kind — the glyph, not a field of color,
// is what says what this is.
func (a *App) thumbnail(c *ui.Context, e store.Entry, kind fileKind) {
	t := c.Theme()
	ch := chromeOf(t)

	plate := ui.Box(c).
		FillWidth().
		Height(plateHeight).
		Radius(5).
		Clip().
		Center().
		Background(ch.inset).
		Border(0, ui.Transparent)

	if bmp := a.thumbs[e.Key]; bmp != nil {
		plate.Children(func() {
			ui.Image(c, bmp).Fit(ui.Cover).Fill()
		})
		return
	}

	plate.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, ch.inset, 5)
		if e.Folder {
			// Folders read as a place rather than a document, so their
			// plate is tinted a shade away from the files'.
			p.Fill(r, kindWash(t, kindFolder), 5)
		}
	})
	plate.Children(func() {
		switch {
		case a.thumbLoading[e.Key]:
			ui.Spinner(c).Label("Loading " + e.Name)
		default:
			size := float32(40)
			if e.Folder {
				size = 44
			}
			ui.Icon(c, entryIcon(e)).FontSize(size).TextColor(kindTint(t, kind))
		}
	})
}

func cardDetail(e store.Entry) string {
	if e.Folder {
		return "Folder"
	}
	size := formatBytes(e.Size)
	if e.LastModified.IsZero() {
		return size
	}
	return size + " · " + formatRelative(e.LastModified, now())
}

/* -------------------------------------------------------------------------
   The table
   ------------------------------------------------------------------------- */

func (a *App) tableView(c *ui.Context, items []store.Entry) {
	a.fileList.Key = func(i int) any { return items[i].Key }
	a.fileList.Label = func(i int) string { return items[i].Name }
	a.fileList.Selection = &a.sel
	a.fileList.Selected = &a.cursor
	a.fileList.Sort = &a.tableSort

	columns := []ui.TableColumn{
		{Title: "Name", ID: string(sortName), MinWidth: 200, Sortable: true},
		{Title: "Kind", ID: "kind", Width: 130},
		{Title: "Size", ID: string(sortSize), Width: 92, Align: ui.End, Sortable: true},
		{Title: "Modified", ID: string(sortModified), Width: 150, Sortable: true},
	}
	table := ui.Table(c, &a.fileList, columns, len(items), func(row, col int) {
		if row < 0 || row >= len(items) {
			return
		}
		a.cell(c, items[row], col)
	}).Grow(1)

	if a.cursor >= 0 && a.cursor < len(items) && table.Submitted() {
		a.openEntry(items[a.cursor])
	}
}

func (a *App) cell(c *ui.Context, e store.Entry, col int) {
	t := c.Theme()
	kind := kindOf(e.Key, e.Folder)
	switch col {
	case 0:
		ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(2, 4).Children(func() {
			ui.Icon(c, entryIcon(e)).FontSize(15).TextColor(kindTint(t, kind)).Shrink(0)
			ui.Text(c, e.Name).FontSize(typeBody).Grow(1).SingleLine().Ellipsis("…")
		})
	case 1:
		ui.Text(c, kindLabel(kind, e.Key)).
			FontSize(typeCaption).
			TextColor(t.TextMuted).
			SingleLine().
			Padding(0, 4)
	case 2:
		ui.Text(c, sizeText(e)).
			FontSize(typeCaption).
			TextColor(t.TextMuted).
			SingleLine().
			Padding(0, 4)
	case 3:
		ui.Text(c, modifiedText(e)).
			FontSize(typeCaption).
			TextColor(t.TextMuted).
			SingleLine().
			Padding(0, 4)
	}
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
	ch := chromeOf(t)

	switch {
	case a.loading && len(a.buckets) == 0:
		skeletons(c)

	case len(a.buckets) == 0:
		emptyState(c, iconBucket, "No Buckets Available",
			"The connection answered successfully, but no buckets are accessible with these credentials.", func() {
				if ctlPrimary(c, "New Bucket").Clicked() {
					a.openNewBucket()
				}
				if ctlButton(c, "Refresh").Clicked() {
					a.loadBuckets()
				}
			})

	default:
		head := ui.Row(c).
			FillWidth().
			Height(30).
			Padding(0, 16).
			AlignItems(ui.Center).
			Background(ui.Transparent).
			Border(0, ui.Transparent)
		head.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(r, ch.inset, 0)
			p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - 1, W: r.W, H: 1}, ch.hairline, 0)
		})
		head.Children(func() {
			ui.Text(c, plural(len(a.buckets), "bucket", "buckets")).
				FontSize(typeCaption).
				TextColor(t.TextMuted)
		})

		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Padding(12, 16).Gap(4).Children(func() {
				for _, b := range a.buckets {
					a.bucketRow(c, b)
				}
			})
		})
	}
}

func (a *App) bucketRow(c *ui.Context, b store.Bucket) {
	t := c.Theme()
	ch := chromeOf(t)

	row := ui.Row(c).
		Key(b.Name).
		Padding(8, 12).
		Gap(11).
		AlignItems(ui.Center).
		Radius(btnRadius).
		Cursor(ui.CursorPointer).
		TextColor(t.Text).
		Background(ui.Transparent).
		Border(0, ui.Transparent).
		Focusable().
		OnClick(func() { a.openBucket(b.Name) })

	hovered := row.Hovered()
	row.ContextMenu(func(m *ui.Menu) { a.bucketMenu(m, b) })
	row.Draw(func(p *ui.Painter, r ui.Rect) {
		switch {
		case hovered:
			p.Fill(r, ch.hover, btnRadius)
		default:
			paintSurface(p, r, btnRadius, ui.Transparent, ch.hairlineFaint)
		}
	})

	row.Children(func() {
		mark := ui.Box(c).Size(34, 34).Radius(8).Center().Shrink(0).
			Background(ui.Transparent).Border(0, ui.Transparent)
		mark.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(r, kindWash(t, kindFolder), 8)
		})
		mark.Children(func() {
			ui.Icon(c, iconBucket).FontSize(19).TextColor(kindTint(t, kindFolder))
		})

		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, b.Name).FontSize(typeBody).FontWeight(weightMedium).SingleLine().Ellipsis("…")
			ui.Text(c, createdText(b)).FontSize(typeCaption).TextColor(t.TextMuted).SingleLine()
		})

		ui.Icon(c, iconChevron).FontSize(12).TextColor(t.TextMuted).Shrink(0)
	})
}

func createdText(b store.Bucket) string {
	if b.Created.IsZero() {
		return "S3 Bucket"
	}
	return "Created " + formatRelative(b.Created, now())
}

/* -------------------------------------------------------------------------
   Menus
   ------------------------------------------------------------------------- */

func (a *App) entryMenu(m *ui.Menu, e store.Entry) {
	if !a.sel.Has(e.Key) {
		a.chooseOnly(e.Key)
	}
	if !e.Folder {
		if m.Item("Open").Shortcut(ui.Cmd, ui.KeyEnter).Chosen() {
			a.showDetail(e.Key)
		}
		if m.Item("Open in Browser").Chosen() {
			a.openInBrowser(e.Key)
		}
		if m.Item("Save As…").Chosen() {
			a.saveAs(e.Key)
		}
		m.Separator()
		if m.Item("Share…").Chosen() {
			a.openShare(e.Key)
		}
		if m.Item("Copy Link").Chosen() {
			a.copyLink(e.Key)
		}
		if m.Item("Copy Key").Chosen() {
			a.copyText(e.Key, "Key copied")
		}
		m.Separator()
	}
	if m.Item("Download").Chosen() {
		a.download()
	}
	if len(a.chosen()) == 1 {
		if m.Item("Rename…").Shortcut(0, ui.KeyF2).Chosen() {
			a.openRename(e)
		}
	}
	if m.Item("Move to Folder…").Chosen() {
		a.openMove()
	}
	m.Separator()
	if m.Item("Delete…").Chosen() {
		a.deleteChosen()
	}
}

func (a *App) bucketMenu(m *ui.Menu, b store.Bucket) {
	if m.Item("Open").Chosen() {
		a.openBucket(b.Name)
	}
	if m.Item("Measure Storage").Chosen() {
		a.bucket = b.Name
		a.prefix = ""
		a.setSection(sectionDashboard)
		a.stat = nil
		a.loadStat()
	}
	m.Separator()
	if m.Item("Copy Bucket Name").Chosen() {
		a.copyText(b.Name, "Name copied")
	}
	m.Separator()
	if m.Item("Delete Bucket…").Chosen() {
		name := b.Name
		a.ask("Delete “"+name+"”?", "The bucket must be empty. This cannot be undone.", "Delete", func() {
			a.removeBucket(name)
		})
	}
}
