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

// fileHeader sums up what is shown, and warns when the listing stopped
// short of the folder's end.
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

	ui.Row(c).Padding(12, 16).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, summarizeListing(folders, files, bytes)).FontSize(12).TextColor(t.TextMuted)
		if a.searchResults != nil {
			ui.Badge(c, "Search results")
		}
		if source.Truncated {
			ui.Badge(c, "More objects than shown")
		}
		ui.Spacer(c)
		if a.loading {
			ui.Spinner(c).Label("Loading")
		}
	})
}

// filesEmpty draws the block the file area shows for an empty folder, an
// empty bucket, or a search that found nothing.
func (a *App) filesEmpty(c *ui.Context) {
	switch {
	case a.searchResults != nil:
		emptyState(c, iconSearch, "Nothing found",
			fmt.Sprintf("No object in %s matches “%s”.", a.bucket, a.search), func() {
				if ui.PrimaryButton(c, "Show folder").Clicked() {
					a.clearSearch()
				}
			})
	case a.filter != "all":
		emptyState(c, iconOther, "Nothing of that kind",
			"This folder holds nothing the chosen type covers.", func() {
				if ui.PrimaryButton(c, "Show everything").Clicked() {
					a.filter = "all"
				}
				if ui.Button(c, "Search the bucket").Clicked() {
					a.deepSearch()
				}
			})
	case a.prefix != "":
		emptyState(c, iconFolder, "This folder is empty",
			"Drop files here, or upload them to fill it.", func() {
				if ui.PrimaryButton(c, "Upload files").Clicked() {
					a.upload()
				}
				if ui.Button(c, "New folder").Clicked() {
					a.openNewFolder()
				}
			})
	default:
		emptyState(c, iconBucket, "This bucket is empty",
			"Nothing has been uploaded here yet.", func() {
				if ui.PrimaryButton(c, "Upload files").Clicked() {
					a.upload()
				}
			})
	}
}

/* -------------------------------------------------------------------------
   The grid of cards
   ------------------------------------------------------------------------- */

// gridView draws the folder as a grid of cards, each showing a thumbnail
// when there is one.
func (a *App) gridView(c *ui.Context, items []store.Entry) {
	a.grid.Key = func(i int) any { return items[i].Key }
	a.grid.Label = func(i int) string { return items[i].Name }
	a.grid.Selection = &a.sel
	a.grid.Selected = &a.cursor
	g := ui.GridView(c, &a.grid, len(items), 170, 176, func(i int) {
		a.card(c, items[i])
	}).Grow(1)
	if a.cursor >= 0 && a.cursor < len(items) && g.Submitted() {
		a.openEntry(items[a.cursor])
	}
}

// card draws one file in the grid.
func (a *App) card(c *ui.Context, e store.Entry) {
	t := c.Theme()
	chosen := a.sel.Has(e.Key)
	card := ui.Column(c).Key(e.Key).Padding(10).Gap(6).Radius(t.Radius).
		Border(1, t.Border).Cursor(ui.CursorPointer)
	switch {
	case chosen:
		card.Background(t.Selection).Border(1, t.Accent)
	case card.Hovered():
		card.Background(t.SurfaceHover)
	}
	card.ContextMenu(func(m *ui.Menu) { a.entryMenu(m, e) })
	card.Children(func() {
		ui.Box(c).FillWidth().Height(96).Radius(max(t.Radius-2, 2)).Clip().Center().Children(func() {
			a.thumbnail(c, e)
		})
		ui.Text(c, e.Name).FontSize(12.5).SingleLine().Ellipsis("…").Label(e.Name)
		ui.Text(c, cardDetail(e)).FontSize(11).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
	})
}

// thumbnail draws what stands for a file in a card: its picture when the
// app has one, its icon otherwise.
func (a *App) thumbnail(c *ui.Context, e store.Entry) {
	t := c.Theme()
	if bmp := a.thumbs[e.Key]; bmp != nil {
		ui.Image(c, bmp).Fit(ui.Contain).Grow(1)
		return
	}
	kind := kindOf(e.Key, e.Folder)
	if a.thumbLoading[e.Key] {
		ui.Spinner(c).Label("Loading " + e.Name)
		return
	}
	ui.Icon(c, entryIcon(e)).FontSize(34).TextColor(kindTint(t, kind))
}

// cardDetail describes an entry under its name: a folder says how it is
// marked, a file its size and when it changed.
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

// tableView draws the folder as a table, with a header that sorts it.
func (a *App) tableView(c *ui.Context, items []store.Entry) {
	a.fileList.Key = func(i int) any { return items[i].Key }
	a.fileList.Label = func(i int) string { return items[i].Name }
	a.fileList.Selection = &a.sel
	a.fileList.Selected = &a.cursor
	a.fileList.Sort = &a.tableSort
	a.fileList.RoundRows = false

	columns := []ui.TableColumn{
		{Title: "Name", ID: string(sortName), MinWidth: 160, Sortable: true},
		{Title: "Kind", ID: "kind", Width: 150},
		{Title: "Size", ID: string(sortSize), Width: 110, Align: ui.End, Sortable: true},
		{Title: "Modified", ID: string(sortModified), Width: 190, Sortable: true},
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

// cell draws one cell of the table.
func (a *App) cell(c *ui.Context, e store.Entry, col int) {
	t := c.Theme()
	switch col {
	case 0:
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, entryIcon(e)).FontSize(15).TextColor(kindTint(t, kindOf(e.Key, e.Folder)))
			ui.Text(c, e.Name).FontSize(12.5).SingleLine().Ellipsis("…")
		})
	case 1:
		ui.Text(c, kindLabel(kindOf(e.Key, e.Folder), e.Key)).FontSize(12).TextColor(t.TextMuted).SingleLine()
	case 2:
		ui.Text(c, sizeText(e)).FontSize(12).TextColor(t.TextMuted).SingleLine()
	case 3:
		ui.Text(c, modifiedText(e)).FontSize(12).TextColor(t.TextMuted).SingleLine()
	}
}

// sizeText is how a row's size reads: a folder has none of its own.
func sizeText(e store.Entry) string {
	if e.Folder {
		return "—"
	}
	return formatBytes(e.Size)
}

// modifiedText is how a row's date reads.
func modifiedText(e store.Entry) string {
	if e.LastModified.IsZero() {
		return "—"
	}
	return formatRelative(e.LastModified, now())
}

/* -------------------------------------------------------------------------
   Buckets
   ------------------------------------------------------------------------- */

// bucketsView lists the endpoint's buckets, which is what the file area
// shows before one is opened.
func (a *App) bucketsView(c *ui.Context) {
	t := c.Theme()
	switch {
	case a.loading && len(a.buckets) == 0:
		skeletons(c)
	case len(a.buckets) == 0:
		emptyState(c, iconBucket, "No buckets here",
			"The endpoint answered, but it has no buckets this key can see.", func() {
				if ui.PrimaryButton(c, "New bucket").Clicked() {
					a.openNewBucket()
				}
				if ui.Button(c, "Refresh").Clicked() {
					a.loadBuckets()
				}
			})
	default:
		ui.Row(c).Padding(12, 16).AlignItems(ui.Center).Children(func() {
			ui.Text(c, plural(len(a.buckets), "bucket", "buckets")).FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			if ui.PrimaryButton(c, "New bucket").Clicked() {
				a.openNewBucket()
			}
		})
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(6).Padding(0, 16, 16, 16).Children(func() {
				for _, b := range a.buckets {
					a.bucketRow(c, b)
				}
			})
		})
	}
}

// bucketRow draws one bucket, which opens on a double click and offers its
// own actions on a right click.
func (a *App) bucketRow(c *ui.Context, b store.Bucket) {
	t := c.Theme()
	row := ui.Row(c).Key(b.Name).Padding(12).Gap(12).AlignItems(ui.Center).
		Radius(t.Radius).Border(1, t.Border).Cursor(ui.CursorPointer)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.ContextMenu(func(m *ui.Menu) { a.bucketMenu(m, b) })
	row.Focusable()
	row.OnClick(func() { a.openBucket(b.Name) })
	row.Children(func() {
		ui.Box(c).Size(34, 34).Radius(9).Background(t.SurfaceHover).Center().Children(func() {
			ui.Icon(c, iconBucket).FontSize(18).TextColor(t.Accent)
		})
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, b.Name).FontSize(13).Bold().SingleLine().Ellipsis("…")
			ui.Text(c, createdText(b)).FontSize(11).TextColor(t.TextMuted).SingleLine()
		})
		ui.Icon(c, iconChevron).FontSize(14).TextColor(t.TextMuted)
	})
}

// createdText describes when a bucket was made, when the endpoint said.
func createdText(b store.Bucket) string {
	if b.Created.IsZero() {
		return "Bucket"
	}
	return "Created " + formatRelative(b.Created, now())
}

/* -------------------------------------------------------------------------
   Menus
   ------------------------------------------------------------------------- */

// entryMenu fills the menu a file offers: the actions that apply to it,
// and to the whole selection when it is part of one.
func (a *App) entryMenu(m *ui.Menu, e store.Entry) {
	// A right click on something outside the selection acts on it alone,
	// as in a file manager.
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

// bucketMenu fills the menu a bucket offers.
func (a *App) bucketMenu(m *ui.Menu, b store.Bucket) {
	if m.Item("Open").Chosen() {
		a.openBucket(b.Name)
	}
	if m.Item("Measure").Chosen() {
		a.bucket = b.Name
		a.prefix = ""
		a.section = sectionDashboard
		a.stat = nil
		a.loadStat()
	}
	m.Separator()
	if m.Item("Copy name").Chosen() {
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
