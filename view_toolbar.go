package main

import (
	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
)

var typeFilters = []struct {
	Label string
	Kind  string
}{
	{"All Types", "all"},
	{"Folders", "folders"},
	{"Images", "image"},
	{"Video", "video"},
	{"Audio", "audio"},
	{"Documents", "document"},
	{"Archives", "archive"},
	{"Other", "other"},
}

func filterLabels() []string {
	out := make([]string, 0, len(typeFilters))
	for _, f := range typeFilters {
		out = append(out, f.Label)
	}
	return out
}

func (a *App) filterLabel() string {
	for _, f := range typeFilters {
		if f.Kind == a.filter {
			return f.Label
		}
	}
	return typeFilters[0].Label
}

/* -------------------------------------------------------------------------
   Toolbar

   Three groups, and no more: where you are, how the files are shown,
   and what you can do to them. The one primary action sits at the very
   end of the trailing edge.
   ------------------------------------------------------------------------- */

// topBar is the strip of controls along the top of the window. It shares
// the window's material with the sidebar, and a single hairline is all
// that tells it apart from the content below.
func (a *App) topBar(c *ui.Context) {
	t := c.Theme()
	ch := chromeOf(t)
	bar := c.TitleBar()

	strip := ui.Row(c).
		FillWidth().
		Height(headerHeight(bar)).
		Padding(0, max(bar.Right, 14), 0, 8).
		Gap(8).
		AlignItems(ui.Center).
		Background(ui.Transparent).
		Border(0, ui.Transparent).
		DragWindow()
	strip = paintMaterial(c, strip)
	strip.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - 1, W: r.W, H: 1}, ch.hairlineEdge, 0)
	})

	strip.Children(func() {
		a.toolbarLeading(c)

		ui.Spacer(c).Grow(1)

		if a.browsing() {
			a.viewChooser(c)
			a.searchField(c)
			ui.Box(c).Width(1).Height(18).Margin(0, 2).Background(ch.hairline)
		}
		a.toolbarActions(c)
	})
}

// toolbarLeading says where you are: the way back up, and the path.
func (a *App) toolbarLeading(c *ui.Context) {
	t := c.Theme()

	if !a.browsing() {
		ui.Text(c, a.sectionTitle()).
			FontSize(typeBody).
			FontWeight(weightSemi).
			TextColor(t.Text).
			Padding(0, 6)
		return
	}

	up := ctlIconButtonBare(c, "Parent Folder").Tooltip("Go to parent folder (⌘↑)")
	up.Children(func() { ui.Icon(c, iconUp).FontSize(14).TextColor(t.Text) })
	if up.Clicked() {
		a.goUp()
	}

	crumbs := a.crumbs()
	labels := make([]string, 0, len(crumbs))
	prefixes := make([]string, 0, len(crumbs))
	for _, crumb := range crumbs {
		labels = append(labels, crumb.Label)
		prefixes = append(prefixes, crumb.Prefix)
	}
	a.crumb = len(labels) - 1

	path := ui.Breadcrumbs(c.Key("crumbs"), labels, &a.crumb).
		Label("Path").
		MinWidth(120).
		FontSize(typeBody)
	if path.Changed() {
		if a.crumb >= 0 && a.crumb < len(prefixes) {
			a.setSection(sectionFiles)
			a.openPrefix(prefixes[a.crumb])
		}
	}
}

// toolbarActions are the things that can be done to what is being shown.
func (a *App) toolbarActions(c *ui.Context) {
	t := c.Theme()

	if a.hasSelection() {
		a.selectionActions(c)
		return
	}

	if a.section == sectionSettings || !a.session.Connected {
		return
	}

	// As the window narrows, the toolbar sheds the items that have
	// somewhere else to live: New Folder is in the More menu, and the
	// type filter gives its width back. Nothing reaches the edge.
	if !compactWindow(c) {
		newFolder := ctlIconButtonBare(c, "New Folder").Tooltip("Create folder (⌘⇧N)")
		newFolder.Children(func() { ui.Icon(c, iconNewFile).FontSize(15).TextColor(t.Text) })
		if newFolder.Clicked() {
			a.openNewFolder()
		}
	}

	refresh := ctlIconButtonBare(c, "Refresh").Tooltip("Reload listing (⌘R)")
	refresh.Children(func() { ui.Icon(c, iconRefresh).FontSize(15).TextColor(t.Text) })
	if refresh.Clicked() {
		a.refresh()
	}

	more := ctlIconButtonBare(c, "More Options").Tooltip("More bucket actions")
	more.Children(func() { ui.Icon(c, iconMore).FontSize(15).TextColor(t.Text) })
	if more.Clicked() {
		a.menuOpen = !a.menuOpen
	}
	ui.Popover(c, more, &a.menuOpen, func() {
		a.bucketMenuPopover(c)
	})

	upload := ctlPrimary(c, "").Tooltip("Upload files (⌘O)")
	upload.Children(func() {
		ui.Icon(c, iconUpload).FontSize(13)
		ui.Text(c, "Upload").FontSize(typeBody)
	})
	if upload.Clicked() {
		a.upload()
	}
}

// bucketMenuPopover holds the bucket-wide actions that do not deserve a
// place of their own in the toolbar.
func (a *App) bucketMenuPopover(c *ui.Context) {
	ui.Column(c).Width(210).Children(func() {
		if menuItem(c, "New Folder…", "newfile") {
			a.menuOpen = false
			a.openNewFolder()
		}
		if menuItem(c, "New Bucket…", "bucket") {
			a.menuOpen = false
			a.openNewBucket()
		}
		divider(c)
		if menuItem(c, "Measure Storage", "dashboard") {
			a.menuOpen = false
			a.setSection(sectionDashboard)
			a.stat = nil
			a.loadStat()
		}
		if menuItem(c, "Copy S3 Path", "copy") {
			a.menuOpen = false
			a.copyText(a.location(), "S3 path copied")
		}
	})
}

// selectionActions act on what is chosen, and replace the usual actions
// while something is.
func (a *App) selectionActions(c *ui.Context) {
	t := c.Theme()
	count := a.sel.Len()

	tally := ui.Row(c).Gap(5).AlignItems(ui.Center).Padding(4, 9).Radius(btnRadius).
		Shrink(0).Background(chromeOf(t).selected)
	tally.Children(func() {
		ui.Text(c, plural(count, "item", "items")).
			FontSize(typeCaption).
			FontWeight(weightSemi).
			TextColor(t.Text).
			SingleLine()
	})

	download := ctlButton(c, "Download")
	download.Children(func() {
		ui.Icon(c, iconDownload).FontSize(13).TextColor(t.Text)
		ui.Text(c, "Download").FontSize(typeBody)
	})
	if download.Clicked() {
		a.download()
	}

	move := ctlButton(c, "Move…")
	move.Children(func() {
		ui.Icon(c, iconMove).FontSize(13).TextColor(t.Text)
		ui.Text(c, "Move…").FontSize(typeBody)
	})
	if move.Clicked() {
		a.openMove()
	}

	del := ctlIconButtonBare(c, "Delete").Tooltip("Delete selected")
	del.Children(func() { ui.Icon(c, iconTrash).FontSize(14).TextColor(t.Danger) })
	if del.Clicked() {
		a.deleteChosen()
	}

	done := ctlIconButtonBare(c, "Done").Tooltip("Deselect all (esc)")
	done.Children(func() { ui.Icon(c, iconClose).FontSize(13).TextColor(t.Text) })
	if done.Clicked() {
		a.sel.Clear()
	}
}

// viewChooser holds the grid/list switch and the type filter.
func (a *App) viewChooser(c *ui.Context) {
	ch := chromeOf(c.Theme())
	t := c.Theme()

	track := ui.ToggleGroup(c.Key("mode"), func() {
		gridOn := a.mode == viewGrid
		grid := ctlSegment(c, &gridOn, "Grid View")
		grid.Children(func() {
			ui.Icon(c, iconGrid).FontSize(14).TextColor(segmentTint(gridOn, t)).Shrink(0)
		})
		grid.OnClick(func() {
			gridOn = true
			a.setViewMode(viewGrid)
		})

		listOn := a.mode == viewList
		list := ctlSegment(c, &listOn, "List View")
		list.Children(func() {
			ui.Icon(c, iconList).FontSize(14).TextColor(segmentTint(listOn, t)).Shrink(0)
		})
		list.OnClick(func() {
			listOn = true
			a.setViewMode(viewList)
		})
	}).Label("View As")
	track = track.Background(ui.Transparent).Border(0, ui.Transparent).
		Padding(2).Radius(btnRadius + 2).Gap(0).
		Draw(func(p *ui.Painter, r ui.Rect) {
			paintSurface(p, r, btnRadius+2, ch.inset, ui.Transparent)
		})

	filterWidth := float32(116)
	if compactWindow(c) {
		filterWidth = 96
	}
	filter := a.filterLabel()
	sel := ctlSelect(c, "filter", &filter, filterLabels(), filterWidth)
	sel.Label("Type").Tooltip("Filter by file type")
	if sel.Changed() {
		for _, f := range typeFilters {
			if f.Label == filter {
				a.filter = f.Kind
				break
			}
		}
	}
}

// segmentTint keeps a segment's glyph in step with its own label color.
func segmentTint(on bool, t *ui.Theme) ui.Color {
	if on {
		return t.Text
	}
	return t.TextMuted
}

func (a *App) searchField(c *ui.Context) {
	t := c.Theme()

	width := float32(176)
	if compactWindow(c) {
		width = 128
	}

	field := ui.SearchField(c.Key("search"), &a.search).Label("Search objects").Width(width)
	field = ctlField(c, field)
	switch {
	case field.Submitted():
		a.deepSearch()
	case field.Changed() && a.searchResults != nil:
		a.searchResults = nil
		a.sel.Clear()
	}

	if a.searchResults != nil {
		clear := ctlIconButtonBare(c, "Clear Search").Tooltip("Clear search")
		clear.Children(func() { ui.Icon(c, iconClose).FontSize(12).TextColor(t.Text) })
		if clear.Clicked() {
			a.clearSearch()
		}
	}
}

/* -------------------------------------------------------------------------
   Content
   ------------------------------------------------------------------------- */

// sectionTitle is what the toolbar says when there is no path to show.
// It always agrees with what the sidebar marks as chosen.
func (a *App) sectionTitle() string {
	switch {
	case a.section == sectionSettings:
		return "Settings"
	case a.section == sectionDashboard && a.session.Connected:
		return "Dashboard"
	case a.session.Connected:
		return "Buckets"
	default:
		return "Files"
	}
}

func (a *App) content(c *ui.Context) {
	t := c.Theme()

	col := ui.Column(c).Grow(1).FillHeight().Background(ui.Transparent)

	if dropped := col.DroppedFiles(); len(dropped) > 0 {
		a.uploadPaths(dropped)
	}
	if col.FileDragOver() {
		col.DrawOver(func(p *ui.Painter, r ui.Rect) {
			inner := ui.Rect{X: r.X + 2, Y: r.Y + 2, W: r.W - 4, H: r.H - 4}
			p.Fill(inner, t.Accent.Alpha(0.06), 8)
			p.Stroke(inner, t.Accent, 8, 2)
		})
	}

	col.Children(func() {
		if a.err != "" {
			a.errorBanner(c)
		}
		switch {
		case a.section == sectionSettings:
			a.settings(c)
		case !a.session.Connected:
			a.welcome(c)
		case a.section == sectionDashboard:
			a.dashboard(c)
		default:
			a.files(c)
		}
	})
}

// errorBanner says what went wrong, in a band above the content.
func (a *App) errorBanner(c *ui.Context) {
	t := c.Theme()

	banner := ui.Row(c).
		FillWidth().
		Padding(7, 14).
		Gap(9).
		AlignItems(ui.Center).
		Background(ui.Transparent).
		Border(0, ui.Transparent)
	banner.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, t.Danger.Alpha(0.09), 0)
		p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - 1, W: r.W, H: 1}, t.Danger.Alpha(0.28), 0)
	})
	banner.Children(func() {
		ui.Icon(c, iconAlert).FontSize(14).TextColor(t.Danger).Shrink(0)
		ui.Text(c, a.err).FontSize(typeBody).Grow(1).TextColor(t.Text)
		dismiss := ctlIconButtonBare(c, "Dismiss")
		dismiss.Children(func() { ui.Icon(c, iconClose).FontSize(12).TextColor(t.Text) })
		if dismiss.Clicked() {
			a.err = ""
		}
	})
}

func (a *App) location() string {
	if !a.browsing() {
		return ""
	}
	if a.prefix == "" {
		return a.bucket
	}
	return a.bucket + "/" + a.prefix
}

/* -------------------------------------------------------------------------
   Welcome Screen
   ------------------------------------------------------------------------- */

func (a *App) welcome(c *ui.Context) {
	t := c.Theme()
	ch := chromeOf(t)

	ui.Column(c).Fill().Center().Gap(16).Padding(40).Children(func() {
		mark := ui.Box(c).Size(76, 76).Radius(18).Center().Shrink(0).
			Background(ui.Transparent).Border(0, ui.Transparent)
		mark.Draw(func(p *ui.Painter, r ui.Rect) {
			paintSurface(p, r, 18, t.Accent, ui.Transparent)
		})
		mark.Children(func() {
			ui.Icon(c, iconCloud).FontSize(36).TextColor(ch.accentOnFill)
		})

		ui.Column(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Connect Your Storage").
				FontSize(typeTitle).
				FontWeight(weightSemi).
				TextColor(t.Text)
			ui.Text(c, "A native client for Cloudflare R2, Amazon S3, MinIO, and any other S3-compatible endpoint.").
				FontSize(typeBody).
				TextColor(t.TextMuted).
				TextAlign(ui.Center).
				MaxWidth(380)
		})

		ui.Row(c).Gap(8).Children(func() {
			connect := ctlPrimary(c, "Connect Storage")
			if connect.Clicked() {
				a.openConnection(nil)
			}

			if len(a.connections) > 0 {
				label := plural(len(a.connections), "Saved Connection", "Saved Connections")
				if ctlButton(c, label+"…").Clicked() {
					a.chipOpen = true
				}
			}
		})
	})
}

// emptyState fills a content plane that has nothing to show, and says
// what to do about it.
func emptyState(c *ui.Context, icon *ui.SVG, title, message string, actions func()) {
	t := c.Theme()

	ui.Column(c).Fill().Center().Gap(12).Padding(40).Children(func() {
		mark := ui.Box(c).Size(52, 52).Radius(13).Center().Shrink(0).
			Background(ui.Transparent).Border(0, ui.Transparent)
		mark.Draw(func(p *ui.Painter, r ui.Rect) {
			paintSurface(p, r, 13, chromeOf(t).inset, ui.Transparent)
		})
		mark.Children(func() {
			ui.Icon(c, icon).FontSize(24).TextColor(t.TextMuted)
		})

		ui.Column(c).Gap(5).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).FontSize(typeHeadline).FontWeight(weightSemi).TextColor(t.Text)
			ui.Text(c, message).
				FontSize(typeBody).
				TextColor(t.TextMuted).
				TextAlign(ui.Center).
				MaxWidth(340)
		})

		if actions != nil {
			ui.Row(c).Gap(8).Children(actions)
		}
	})
}

// skeletons stands in for content that has not arrived yet.
func skeletons(c *ui.Context) {
	t := c.Theme()
	ch := chromeOf(t)

	ui.Column(c).FillWidth().Padding(16).Gap(6).Children(func() {
		for i := range 6 {
			ui.Box(c).Key(i).Height(30).Radius(btnRadius).Background(ui.Transparent).
				Draw(func(p *ui.Painter, r ui.Rect) {
					paintSurface(p, r, btnRadius, ch.inset, ui.Transparent)
				})
		}
	})
}

func entryIcon(e store.Entry) *ui.SVG { return kindIcon(kindOf(e.Key, e.Folder)) }
