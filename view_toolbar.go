package main

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
)

// typeFilters are the kinds the file area can be narrowed to. The label is
// what the chooser shows; the kind is what it matches, "folders" for the
// prefixes and "all" for everything.
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

// filterLabels are the labels of typeFilters, in order.
func filterLabels() []string {
	out := make([]string, 0, len(typeFilters))
	for _, f := range typeFilters {
		out = append(out, f.Label)
	}
	return out
}

// filterLabel returns the label of the kind the file area is showing.
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

// topBar draws the bar along the top of the main column: where the app is,
// the actions that apply there, and the search and view controls. Its
// empty space drags the window, and it keeps clear of the window controls
// where the system puts them on this side.
func (a *App) topBar(c *ui.Context, bar ui.TitleBar) {
	ui.Row(c).
		Height(headerHeight(bar)).
		Padding(0, max(6, bar.Right-8), 0, 6).
		Gap(6).
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
				if a.svc.Connected() {
					a.viewChooser(c)
				}
				return
			}

			a.fileActions(c)
			ui.Spacer(c).DragWindow()
			a.viewChooser(c)
			a.searchField(c)
		})
}

// breadcrumbs shows the path to the folder being browsed, and the way back
// up. Choosing a part of the path opens it.
func (a *App) breadcrumbs(c *ui.Context) {
	if !a.browsing() {
		ui.Text(c, appName).FontSize(13).Bold().PaddingX(6)
		return
	}
	up := ui.Button(c.Key("up"), "").Label("Up one level")
	up.Children(func() { ui.Icon(c, iconUp).FontSize(15) })
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

// fileActions draws what can be done with the files: the selection's
// actions when anything is chosen, the folder's otherwise.
func (a *App) fileActions(c *ui.Context) {
	if a.hasSelection() {
		a.selectionActions(c)
		return
	}

	refresh := ui.Button(c.Key("refresh"), "").Label("Refresh (⌘R)").Tooltip("Reload this folder")
	refresh.Children(func() { ui.Icon(c, iconRefresh).FontSize(15) })
	if refresh.Clicked() {
		a.refresh()
	}

	more := ui.Button(c.Key("more"), "").Label("More actions").Tooltip("New folder, new bucket, and more")
	more.Children(func() { ui.Icon(c, iconMore).FontSize(15) })
	if more.Clicked() {
		a.menuOpen = !a.menuOpen
	}
	ui.Popover(c, more, &a.menuOpen, func() {
		if menuItem(c, "New folder…", "newfile") {
			a.menuOpen = false
			a.openNewFolder()
		}
		if menuItem(c, "New bucket…", "bucket") {
			a.menuOpen = false
			a.openNewBucket()
		}
		divider(c)
		if menuItem(c, "Measure bucket", "dashboard") {
			a.menuOpen = false
			a.section = sectionDashboard
			a.stat = nil
			a.loadStat()
		}
		if menuItem(c, "Copy path", "copy") {
			a.menuOpen = false
			a.copyText(a.location(), "Path copied")
		}
	})

	upload := ui.PrimaryButton(c.Key("upload"), "Upload")
	upload.Children(func() { ui.Icon(c, iconUpload).FontSize(15) })
	if upload.Clicked() {
		a.upload()
	}
}

// selectionActions draws what can be done with the chosen entries, which
// takes the place of the folder's actions while anything is chosen.
func (a *App) selectionActions(c *ui.Context) {
	ui.Text(c, plural(a.sel.Len(), "item", "items")).FontSize(12).TextColor(c.Theme().TextMuted).PaddingX(4)
	if ui.Button(c.Key("clear"), "Done").Tooltip("Clear the selection").Clicked() {
		a.sel.Clear()
	}

	download := ui.Button(c.Key("download"), "Download").Tooltip("Save the chosen items")
	download.Children(func() { ui.Icon(c, iconDownload).FontSize(15) })
	if download.Clicked() {
		a.download()
	}

	more := ui.Button(c.Key("sel-more"), "").Label("More actions for the selection")
	more.Children(func() { ui.Icon(c, iconMore).FontSize(15) })
	if more.Clicked() {
		a.selMenuOpen = !a.selMenuOpen
	}
	ui.Popover(c, more, &a.selMenuOpen, func() {
		if menuItem(c, "Download…", "download") {
			a.selMenuOpen = false
			a.download()
		}
		if menuItem(c, "Move to folder…", "move") {
			a.selMenuOpen = false
			a.openMove()
		}
		if menuItem(c, "Copy keys", "copy") {
			a.selMenuOpen = false
			a.copyText(strings.Join(a.chosenKeys(), "\n"), "Keys copied")
		}
		divider(c)
		if menuItem(c, "Delete…", "trash") {
			a.selMenuOpen = false
			a.deleteChosen()
		}
	})
}

// viewChooser draws the switch between the grid of cards and the table,
// and the chooser that narrows the listing to one kind of file.
func (a *App) viewChooser(c *ui.Context) {
	// The toggles are rebuilt from the app's mode every pass; OnClick runs
	// after the bound value has taken the click, so reading them there is
	// what tells which one the user pressed.
	ui.ToggleGroup(c.Key("mode"), func() {
		gridOn := a.mode == viewGrid
		grid := ui.Toggle(c, &gridOn, "").Label("Grid view").Tooltip("Grid view")
		grid.Children(func() { ui.Icon(c, iconGrid).FontSize(15) })
		grid.OnClick(func() {
			gridOn = true
			a.setViewMode(viewGrid)
		})

		listOn := a.mode == viewList
		list := ui.Toggle(c, &listOn, "").Label("List view").Tooltip("List view")
		list.Children(func() { ui.Icon(c, iconList).FontSize(15) })
		list.OnClick(func() {
			listOn = true
			a.setViewMode(viewList)
		})
	}).Label("View as")

	filter := a.filterLabel()
	if ui.Select(c.Key("filter"), &filter, filterLabels()).Label("Type").Width(126).
		Tooltip("Show only one kind of file").Changed() {
		for _, f := range typeFilters {
			if f.Label == filter {
				a.filter = f.Kind
				break
			}
		}
	}
}

// searchField draws the filter over the folder, and the deep search of the
// whole bucket: typing narrows what is loaded, Enter searches everything,
// and Escape shows the folder again.
func (a *App) searchField(c *ui.Context) {
	field := ui.SearchField(c.Key("search"), &a.search).Label("Search this bucket").Width(190)
	switch {
	case field.Submitted():
		a.deepSearch()
	case field.Changed() && a.searchResults != nil:
		// A new query leaves the results of the old one behind.
		a.searchResults = nil
		a.sel.Clear()
	}
	if a.searchResults != nil {
		if ui.Button(c.Key("show-folder"), "Show folder").Tooltip("Leave the search results").Clicked() {
			a.clearSearch()
		}
	}
}

/* -------------------------------------------------------------------------
   Content
   ------------------------------------------------------------------------- */

// content draws the part of the app the section chose, on the surface the
// window's panels sit on.
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

// errorBanner shows the last error over the content, with a way to dismiss
// it.
func (a *App) errorBanner(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(8, 12).Gap(8).AlignItems(ui.Center).Background(t.Danger.Alpha(0.12)).Children(func() {
		ui.Icon(c, iconAlert).FontSize(15).TextColor(t.Danger)
		ui.Text(c, a.err).FontSize(12).Grow(1)
		dismiss := ui.Button(c.Key("dismiss-error"), "").Label("Dismiss")
		dismiss.Children(func() { ui.Icon(c, iconClose).FontSize(14) })
		if dismiss.Clicked() {
			a.err = ""
		}
	})
}

// location is the path the app is looking at, as one string, for the
// clipboard.
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
   Welcome
   ------------------------------------------------------------------------- */

// welcome invites the user to connect something, when nothing is.
func (a *App) welcome(c *ui.Context) {
	emptyState(c, iconCloud, "Connect your storage",
		"Bucketeer browses any S3-compatible bucket: Cloudflare R2, Amazon S3, Backblaze B2, MinIO, Wasabi, or an endpoint of your own.",
		func() {
			if ui.PrimaryButton(c, "Connect storage").Clicked() {
				a.openConnection(nil)
			}
			if len(a.connections) > 0 {
				label := plural(len(a.connections), "saved connection", "saved connections")
				if ui.Button(c, label+"…").Clicked() {
					a.chipOpen = true
				}
			}
		})
}

// emptyState draws the block a part of the interface shows when there is
// nothing in it.
func emptyState(c *ui.Context, icon *ui.SVG, title, message string, actions func()) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(10).Padding(32).Children(func() {
		ui.Box(c).Size(56, 56).Radius(16).Background(t.SurfaceHover).Center().Children(func() {
			ui.Icon(c, icon).FontSize(26).TextColor(t.TextMuted)
		})
		ui.Text(c, title).FontSize(17).Bold()
		ui.Text(c, message).FontSize(12.5).TextColor(t.TextMuted).TextAlign(ui.Center).MaxWidth(400)
		if actions != nil {
			ui.Row(c).Gap(8).Children(actions)
		}
	})
}

// skeletons draws placeholder rows while a listing loads.
func skeletons(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Gap(8).Padding(16).Children(func() {
		for i := range 3 {
			ui.Box(c).Key(i).Height(52).Radius(t.Radius).Background(t.SurfaceHover).Opacity(0.6)
		}
	})
}

// entryIcon returns the icon that stands for an entry.
func entryIcon(e store.Entry) *ui.SVG { return kindIcon(kindOf(e.Key, e.Folder)) }
