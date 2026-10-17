package main

import (
	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
)

var typeFilters = []struct {
	Label string
	Kind  string
}{
	{"All types", "all"},
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
   ------------------------------------------------------------------------- */

func (a *App) topBar(c *ui.Context, bar ui.TitleBar) {
	t := c.Theme()
	ui.Row(c).
		Height(headerHeight(bar)).
		Padding(0, max(8, bar.Right-4), 0, 8).
		Gap(8).
		AlignItems(ui.Center).
		DragWindow().
		Children(func() {
			a.breadcrumbs(c)

			if a.section == sectionSettings {
				ui.Spacer(c).DragWindow()
				return
			}
			if !a.browsing() {
				ui.Spacer(c).DragWindow()
				if a.session.Connected {
					a.viewChooser(c)
				}
				return
			}

			a.fileActions(c)
			ui.Spacer(c).DragWindow()
			a.viewChooser(c)
			a.searchField(c)
		})
	_ = t
}

func (a *App) breadcrumbs(c *ui.Context) {
	t := c.Theme()
	if !a.browsing() {
		title := appName
		if a.section == sectionDashboard {
			title = "Dashboard"
		} else if a.section == sectionSettings {
			title = "Settings"
		}
		ui.Text(c, title).FontSize(14).Bold().Padding(0, 6).TextColor(t.Text)
		return
	}

	// Back / Up button
	up := ui.Button(c.Key("up"), "").Label("Parent folder").Tooltip("Go to parent folder (⌘↑)").
		Disabled(a.prefix == "" && !a.browsing())
	up.Children(func() { ui.Icon(c, iconUp).FontSize(14) })
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
	if ui.Breadcrumbs(c.Key("crumbs"), labels, &a.crumb).Label("Path").Changed() {
		if a.crumb >= 0 && a.crumb < len(prefixes) {
			a.section = sectionFiles
			a.openPrefix(prefixes[a.crumb])
		}
	}
}

func (a *App) fileActions(c *ui.Context) {
	if a.hasSelection() {
		a.selectionActions(c)
		return
	}

	// New folder action
	newFolder := ui.Button(c.Key("new-folder"), "").Label("New folder").Tooltip("Create folder (⌘⇧N)")
	newFolder.Children(func() { ui.Icon(c, iconNewFile).FontSize(14) })
	if newFolder.Clicked() {
		a.openNewFolder()
	}

	// Refresh action
	refresh := ui.Button(c.Key("refresh"), "").Label("Refresh").Tooltip("Reload listing (⌘R)")
	refresh.Children(func() { ui.Icon(c, iconRefresh).FontSize(14) })
	if refresh.Clicked() {
		a.refresh()
	}

	// More options popover
	more := ui.Button(c.Key("more"), "").Label("More options").Tooltip("More bucket actions")
	more.Children(func() { ui.Icon(c, iconMore).FontSize(14) })
	if more.Clicked() {
		a.menuOpen = !a.menuOpen
	}
	ui.Popover(c, more, &a.menuOpen, func() {
		ui.Column(c).Width(180).Padding(4).Gap(2).Children(func() {
			if menuItem(c, "New bucket…", "bucket") {
				a.menuOpen = false
				a.openNewBucket()
			}
			if menuItem(c, "Measure storage", "dashboard") {
				a.menuOpen = false
				a.section = sectionDashboard
				a.stat = nil
				a.loadStat()
			}
			divider(c)
			if menuItem(c, "Copy S3 path", "copy") {
				a.menuOpen = false
				a.copyText(a.location(), "S3 path copied")
			}
		})
	})

	// Primary Upload Button
	upload := ui.PrimaryButton(c.Key("upload"), "").Label("Upload")
	upload.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconUpload).FontSize(13)
			ui.Text(c, "Upload").Bold().FontSize(12.5)
		})
	})
	if upload.Clicked() {
		a.upload()
	}
}

func (a *App) selectionActions(c *ui.Context) {
	t := c.Theme()
	count := a.sel.Len()

	ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(4, 8).Radius(6).
		Background(t.Selection).Children(func() {
		ui.Text(c, plural(count, "item", "items")).FontSize(12).Bold().TextColor(t.Accent)
	})

	download := ui.Button(c.Key("download"), "Download").Tooltip("Download selected")
	download.Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconDownload).FontSize(13)
			ui.Text(c, "Download").FontSize(12)
		})
	})
	if download.Clicked() {
		a.download()
	}

	move := ui.Button(c.Key("move"), "Move").Tooltip("Move to folder")
	move.Children(func() {
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconMove).FontSize(13)
			ui.Text(c, "Move…").FontSize(12)
		})
	})
	if move.Clicked() {
		a.openMove()
	}

	del := ui.Button(c.Key("delete"), "").Tooltip("Delete selected")
	del.Children(func() { ui.Icon(c, iconTrash).FontSize(13).TextColor(t.Danger) })
	if del.Clicked() {
		a.deleteChosen()
	}

	if ui.Button(c.Key("clear"), "Done").Tooltip("Deselect all (Esc)").Clicked() {
		a.sel.Clear()
	}
}

func (a *App) viewChooser(c *ui.Context) {
	// Mode switcher (Grid / List)
	ui.ToggleGroup(c.Key("mode"), func() {
		gridOn := a.mode == viewGrid
		grid := ui.Toggle(c, &gridOn, "").Label("Grid view").Tooltip("Grid view")
		grid.Children(func() { ui.Icon(c, iconGrid).FontSize(14) })
		grid.OnClick(func() {
			gridOn = true
			a.setViewMode(viewGrid)
		})

		listOn := a.mode == viewList
		list := ui.Toggle(c, &listOn, "").Label("List view").Tooltip("List view")
		list.Children(func() { ui.Icon(c, iconList).FontSize(14) })
		list.OnClick(func() {
			listOn = true
			a.setViewMode(viewList)
		})
	}).Label("View as")

	// Filter dropdown
	filter := a.filterLabel()
	if ui.Select(c.Key("filter"), &filter, filterLabels()).Label("Type").Width(110).
		Tooltip("Filter by file type").Changed() {
		for _, f := range typeFilters {
			if f.Label == filter {
				a.filter = f.Kind
				break
			}
		}
	}
}

func (a *App) searchField(c *ui.Context) {
	field := ui.SearchField(c.Key("search"), &a.search).Label("Search objects").Width(180)
	switch {
	case field.Submitted():
		a.deepSearch()
	case field.Changed() && a.searchResults != nil:
		a.searchResults = nil
		a.sel.Clear()
	}
	if a.searchResults != nil {
		if ui.Button(c.Key("show-folder"), "Folder").Tooltip("Clear search").Clicked() {
			a.clearSearch()
		}
	}
}

/* -------------------------------------------------------------------------
   Content Shell
   ------------------------------------------------------------------------- */

func (a *App) content(c *ui.Context) {
	t := c.Theme()
	col := ui.Column(c).
		Grow(1).
		FillHeight().
		Background(t.Surface).
		Border(1, t.Border).
		Radius(t.Radius).
		Clip()

	if dropped := col.DroppedFiles(); len(dropped) > 0 {
		a.uploadPaths(dropped)
	}
	if col.FileDragOver() {
		col.Border(2, t.Accent)
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

func (a *App) errorBanner(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).
		Padding(8, 14).
		Gap(10).
		AlignItems(ui.Center).
		Background(t.Danger.Alpha(0.10)).
		BorderWidth(0, 0, 1, 0).
		BorderColor(t.Danger.Alpha(0.20)).
		Children(func() {
			ui.Icon(c, iconAlert).FontSize(14).TextColor(t.Danger)
			ui.Text(c, a.err).FontSize(12).Grow(1).TextColor(t.Danger)
			dismiss := ui.Button(c.Key("dismiss-error"), "").Label("Dismiss")
			dismiss.Children(func() { ui.Icon(c, iconClose).FontSize(13) })
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
	ui.Column(c).Fill().Center().Gap(16).Padding(32).Children(func() {
		// App mascot icon hero container
		ui.Box(c).Size(80, 80).Radius(20).Background(t.SurfaceHover).Border(1, t.Border).Center().Children(func() {
			ui.Icon(c, iconCloud).FontSize(36).TextColor(t.Accent)
		})

		ui.Column(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Connect your storage").FontSize(22).Bold().TextColor(t.Text)
			ui.Text(c, "Fast, native client for Cloudflare R2, Amazon S3, MinIO, and S3 storage.").
				FontSize(13).
				TextColor(t.TextMuted).
				TextAlign(ui.Center).
				MaxWidth(440)
		})

		ui.Row(c).Gap(10).Children(func() {
			connectBtn := ui.PrimaryButton(c, "Connect storage")
			if connectBtn.Clicked() {
				a.openConnection(nil)
			}

			if len(a.connections) > 0 {
				label := plural(len(a.connections), "Saved connection", "Saved connections")
				if ui.Button(c, label+"…").Clicked() {
					a.chipOpen = true
				}
			}
		})
	})
}

func emptyState(c *ui.Context, icon *ui.SVG, title, message string, actions func()) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(12).Padding(32).Children(func() {
		ui.Box(c).Size(56, 56).Radius(16).Background(t.SurfaceHover).Border(1, t.Border).Center().Children(func() {
			ui.Icon(c, icon).FontSize(24).TextColor(t.TextMuted)
		})
		ui.Column(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).FontSize(16).Bold().TextColor(t.Text)
			ui.Text(c, message).FontSize(12.5).TextColor(t.TextMuted).TextAlign(ui.Center).MaxWidth(380)
		})
		if actions != nil {
			ui.Row(c).Gap(8).Children(actions)
		}
	})
}

func skeletons(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Gap(8).Padding(16).Children(func() {
		for i := range 4 {
			ui.Box(c).Key(i).Height(44).Radius(8).Background(t.SurfaceHover).Opacity(0.6)
		}
	})
}

func entryIcon(e store.Entry) *ui.SVG { return kindIcon(kindOf(e.Key, e.Folder)) }
