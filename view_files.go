package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
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

func (a *App) fileHeader(c *ui.Context, items []store.Entry) {
	t := c.Theme()
	source := a.source()
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

	ui.Row(c).
		Padding(10, 16).
		Gap(10).
		AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).
		BorderColor(t.Border).
		Children(func() {
			ui.Text(c, summarizeListing(folders, files, bytes)).FontSize(12).TextColor(t.TextMuted)
			if a.searchResults != nil {
				ui.Badge(c, "Deep search")
			}
			if source.Truncated {
				ui.Badge(c, "Limit reached (10k items)")
			}
			ui.Spacer(c)
			if a.loading {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Spinner(c).Label("Loading")
					ui.Text(c, "Syncing…").FontSize(11).TextColor(t.TextMuted)
				})
			}
		})
}

func (a *App) filesEmpty(c *ui.Context) {
	switch {
	case a.searchResults != nil:
		emptyState(c, iconSearch, "No matches found",
			fmt.Sprintf("No object matching “%s” was found in %s.", a.search, a.bucket), func() {
				if ui.PrimaryButton(c, "Clear search").Clicked() {
					a.clearSearch()
				}
			})
	case a.filter != "all":
		emptyState(c, iconOther, "No files of this type",
			"This folder contains no objects matching the selected filter.", func() {
				if ui.PrimaryButton(c, "Show all types").Clicked() {
					a.filter = "all"
				}
			})
	case a.prefix != "":
		emptyState(c, iconFolder, "Folder is empty",
			"Drag and drop files here, or click Upload to add objects.", func() {
				if ui.PrimaryButton(c, "Upload files").Clicked() {
					a.upload()
				}
				if ui.Button(c, "New folder").Clicked() {
					a.openNewFolder()
				}
			})
	default:
		emptyState(c, iconBucket, "Bucket is empty",
			"No objects have been uploaded to this bucket yet.", func() {
				if ui.PrimaryButton(c, "Upload files").Clicked() {
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

	g := ui.GridView(c, &a.grid, len(items), 156, 168, func(i int) {
		a.card(c, items[i])
	}).Grow(1)
	if a.cursor >= 0 && a.cursor < len(items) && g.Submitted() {
		a.openEntry(items[a.cursor])
	}
}

func (a *App) card(c *ui.Context, e store.Entry) {
	t := c.Theme()
	chosen := a.sel.Has(e.Key)
	kind := kindOf(e.Key, e.Folder)

	card := ui.Column(c).
		Key(e.Key).
		Padding(8).
		Gap(6).
		Radius(t.Radius).
		Border(1, t.Border).
		Background(t.Surface).
		Cursor(ui.CursorPointer)

	switch {
	case chosen:
		card.Background(t.Selection).Border(1.5, t.Accent)
	case card.Hovered():
		card.Background(t.SurfaceHover)
	}

	card.ContextMenu(func(m *ui.Menu) { a.entryMenu(m, e) })

	card.Children(func() {
		// Thumbnail container with soft tinted background wash
		ui.Box(c).
			FillWidth().
			Height(96).
			Radius(6).
			Background(kindSoftBg(t, kind)).
			Clip().
			Center().
			Children(func() {
				a.thumbnail(c, e, kind)
			})

		// File Name and Detail
		ui.Column(c).Gap(2).Padding(2, 4, 2, 4).Children(func() {
			nameColor := t.Text
			if chosen {
				nameColor = t.Accent
			}
			ui.Text(c, e.Name).
				FontSize(12).
				Bold().
				TextColor(nameColor).
				SingleLine().
				Ellipsis("…").
				Label(e.Name)

			ui.Text(c, cardDetail(e)).
				FontSize(10.5).
				TextColor(t.TextMuted).
				SingleLine().
				Ellipsis("…")
		})
	})
}

func (a *App) thumbnail(c *ui.Context, e store.Entry, kind fileKind) {
	t := c.Theme()
	if bmp := a.thumbs[e.Key]; bmp != nil {
		ui.Image(c, bmp).Fit(ui.Cover).Fill().Radius(6)
		return
	}
	if a.thumbLoading[e.Key] {
		ui.Spinner(c).Label("Loading " + e.Name)
		return
	}

	iconSize := float32(42)
	if e.Folder {
		iconSize = 46
	}
	ui.Icon(c, entryIcon(e)).FontSize(iconSize).TextColor(kindTint(t, kind))
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
	a.fileList.RoundRows = true

	columns := []ui.TableColumn{
		{Title: "Name", ID: string(sortName), MinWidth: 200, Sortable: true},
		{Title: "Kind", ID: "kind", Width: 140},
		{Title: "Size", ID: string(sortSize), Width: 110, Align: ui.End, Sortable: true},
		{Title: "Modified", ID: string(sortModified), Width: 180, Sortable: true},
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
		ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(4, 0).Children(func() {
			ui.Icon(c, entryIcon(e)).FontSize(16).TextColor(kindTint(t, kind))
			ui.Text(c, e.Name).FontSize(12.5).SingleLine().Ellipsis("…")
		})
	case 1:
		ui.Text(c, kindLabel(kind, e.Key)).FontSize(12).TextColor(t.TextMuted).SingleLine()
	case 2:
		ui.Text(c, sizeText(e)).FontSize(12).TextColor(t.TextMuted).SingleLine()
	case 3:
		ui.Text(c, modifiedText(e)).FontSize(12).TextColor(t.TextMuted).SingleLine()
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
	switch {
	case a.loading && len(a.buckets) == 0:
		skeletons(c)
	case len(a.buckets) == 0:
		emptyState(c, iconBucket, "No buckets available",
			"The connection answered successfully, but no buckets are accessible with these credentials.", func() {
				if ui.PrimaryButton(c, "New bucket").Clicked() {
					a.openNewBucket()
				}
				if ui.Button(c, "Refresh").Clicked() {
					a.loadBuckets()
				}
			})
	default:
		ui.Row(c).
			Padding(12, 18).
			AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).
			BorderColor(t.Border).
			Children(func() {
				ui.Text(c, plural(len(a.buckets), "bucket", "buckets")).FontSize(12).Bold().TextColor(t.TextMuted)
				ui.Spacer(c)
				if ui.PrimaryButton(c, "New bucket").Clicked() {
					a.openNewBucket()
				}
			})

		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(8).Padding(16).Children(func() {
				for _, b := range a.buckets {
					a.bucketRow(c, b)
				}
			})
		})
	}
}

func (a *App) bucketRow(c *ui.Context, b store.Bucket) {
	t := c.Theme()
	row := ui.Row(c).
		Key(b.Name).
		Padding(12, 16).
		Gap(14).
		AlignItems(ui.Center).
		Radius(8).
		Border(1, t.Border).
		Background(t.Surface).
		Cursor(ui.CursorPointer)

	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}

	row.ContextMenu(func(m *ui.Menu) { a.bucketMenu(m, b) })
	row.Focusable()
	row.OnClick(func() { a.openBucket(b.Name) })

	row.Children(func() {
		ui.Box(c).Size(36, 36).Radius(8).Background(kindSoftBg(t, kindFolder)).Center().Children(func() {
			ui.Icon(c, iconBucket).FontSize(20).TextColor(t.Accent)
		})
		ui.Column(c).Grow(1).Gap(3).Children(func() {
			ui.Text(c, b.Name).FontSize(13.5).Bold().SingleLine().Ellipsis("…")
			ui.Text(c, createdText(b)).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
		ui.Icon(c, iconChevron).FontSize(13).TextColor(t.TextMuted)
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
		if m.Item("Open in browser").Chosen() {
			a.openInBrowser(e.Key)
		}
		if m.Item("Save as…").Chosen() {
			a.saveAs(e.Key)
		}
		m.Separator()
		if m.Item("Share…").Chosen() {
			a.openShare(e.Key)
		}
		if m.Item("Copy link").Chosen() {
			a.copyLink(e.Key)
		}
		if m.Item("Copy key").Chosen() {
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
	if m.Item("Move to folder…").Chosen() {
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
	if m.Item("Measure storage").Chosen() {
		a.bucket = b.Name
		a.prefix = ""
		a.section = sectionDashboard
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
		a.ask("Delete the bucket “"+name+"”?", "The bucket must be empty. This cannot be undone.", "Delete", func() {
			a.removeBucket(name)
		})
	}
}
