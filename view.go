package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
)

// sidebarWidth is how wide the sidebar is, in DIPs.
const sidebarWidth = 232

// view builds the whole window: the sidebar on the window's own material,
// and the content plane beside it with its bar along the top.
func (a *App) view(c *ui.Context) {
	a.services = c.Services()
	t := a.theme(a.dark(c))
	c.SetTheme(t)
	ch := chromeOf(t)

	if a.notice != "" {
		c.Toast(a.notice)
		a.notice = ""
	}

	a.shortcuts(c)

	ui.Row(c).Fill().Children(func() {
		a.sidebar(c)

		// The content plane: the bar along the top, then the section it
		// shows. One hairline down the side is all that separates the
		// two columns.
		plane := ui.Column(c).Grow(1).FillHeight().Background(ch.canvas)
		plane.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(ui.Rect{X: r.X, Y: r.Y, W: 1, H: r.H}, ch.hairline, 0)
		})
		plane.Children(func() {
			a.topBar(c)
			a.content(c)
		})
	})

	// The transfers dock floats over the bottom right corner of the
	// content, as the reference client's does.
	if a.transfersOpen {
		a.transfersDock(c)
	}

	a.dialogs(c)
}

// shortcuts declares the window's commands.
func (a *App) shortcuts(c *ui.Context) {
	c.OnShortcut(ui.Cmd, ui.KeyR, a.refresh)
	c.OnShortcut(ui.Cmd, ui.KeyO, a.upload)
	c.OnShortcut(ui.Cmd, ui.KeyT, func() { a.transfersOpen = !a.transfersOpen })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyN, a.openNewFolder)
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyU, a.upload)
	c.OnShortcut(ui.Cmd, ui.KeyA, a.selectAll)
	c.OnShortcut(0, ui.KeyEscape, func() {
		switch {
		case a.hasSelection():
			a.sel.Clear()
		case a.search != "":
			a.clearSearch()
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyUp, a.goUp)
	c.OnShortcut(0, ui.KeyEnter, func() {
		if e, ok := a.one(); ok {
			a.openEntry(e)
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyBackspace, a.deleteChosen)
}

// headerHeight is how tall the band along the top of the window is.
func headerHeight(bar ui.TitleBar) float32 { return max(bar.Height, 56) }

// compactWindow reports whether the window is too narrow to show every
// label in the toolbar.
func compactWindow(c *ui.Context) bool {
	w, _ := c.Size()
	return w < 1010
}

// dark reports the appearance to draw in: the system's, unless the
// preferences asked for one of their own.
func (a *App) dark(c *ui.Context) bool {
	if a.prefs.DarkMode {
		return true
	}
	return c.Theme().Dark
}

func (a *App) theme(dark bool) *ui.Theme {
	if dark {
		if a.darkTheme == nil {
			a.darkTheme = darkTheme()
		}
		return a.darkTheme
	}
	if a.lightTheme == nil {
		a.lightTheme = lightTheme()
	}
	return a.lightTheme
}

// trafficInset leaves room for the window's own controls.
func trafficInset(bar ui.TitleBar, fallback float32) float32 {
	return max(bar.Left, fallback)
}

/* -------------------------------------------------------------------------
   Sidebar
   ------------------------------------------------------------------------- */

const (
	navDashboard = "section:dashboard"
	navFiles     = "section:files"
	navRecent    = "section:recent"
	navSettings  = "section:settings"
)

// sidebar draws the navigation: the three places to look, the folders of
// the place being browsed, and the connection at the foot.
func (a *App) sidebar(c *ui.Context) {
	bar := c.TitleBar()
	a.syncSidebarChoice()

	width := float32(sidebarWidth)
	if compactWindow(c) {
		width = 208
	}
	pane := ui.Column(c).Width(width).FillHeight().Padding(0, 0, 0, 0).Background(ui.Transparent)
	pane = paintMaterial(c, pane)

	pane.Children(func() {
		// The window's drag region, clear of the traffic lights.
		ui.Row(c).Height(headerHeight(bar)).Padding(0, 10, 0, trafficInset(bar, 14)).DragWindow()

		ui.Column(c).Padding(0, 10).Gap(2).FocusGroup(ui.Vertical).Children(func() {
			navRow(c, navDashboard, "Dashboard", iconDashboard, a.sidebarID == navDashboard, nil, func() {
				a.setSection(sectionDashboard)
			})
			navRow(c, navFiles, "All files", iconFiles, a.sidebarID == navFiles, nil, func() {
				a.setSection(sectionFiles)
			})
			navRow(c, navRecent, "Recent files", iconRecent, a.sidebarID == navRecent, nil, func() {
				a.setSection(sectionRecent)
			})
		})

		// The folders of the place being browsed, one level deep, as the
		// reference client lists them.
		ui.Column(c).Grow(1).Padding(0, 10).Children(func() {
			folders := a.sidebarFolders()
			if len(folders) == 0 {
				return
			}
			sidebarLabel(c, "Folders")
			ui.Column(c).Gap(2).FocusGroup(ui.Vertical).Children(func() {
				for _, e := range folders {
					entry := e
					id := "folder:" + entry.Key
					navRow(c, id, entry.Name, iconFolder, a.sidebarID == id, nil, func() {
						a.setSection(sectionFiles)
						a.openPrefix(entry.Key)
					})
				}
			})
		})

		ui.Column(c).Padding(0, 10, 0, 10).Gap(2).Children(func() {
			navRow(c, navSettings, "Settings", iconSettings, a.sidebarID == navSettings, nil, func() {
				a.setSection(sectionSettings)
			})
		})

		ui.Column(c).Padding(0, 10, 0, 10).Children(func() {
			a.connectionRow(c)
		})
	})
}

// syncSidebarChoice points the sidebar's own selection at where the app
// is looking.
func (a *App) syncSidebarChoice() {
	switch {
	case a.section == sectionSettings:
		a.sidebarID = navSettings
	case a.section == sectionRecent:
		a.sidebarID = navRecent
	case a.section == sectionDashboard && a.session.Connected:
		a.sidebarID = navDashboard
	case a.browsing() && a.prefix != "":
		a.sidebarID = "folder:" + a.prefix
	default:
		a.sidebarID = navFiles
	}
}

// sidebarFolders returns the folders of the browsed location.
func (a *App) sidebarFolders() []store.Entry {
	if !a.browsing() {
		return nil
	}
	var out []store.Entry
	for _, e := range a.listing.Items {
		if e.Folder {
			out = append(out, e)
		}
	}
	return out
}

/* -------------------------------------------------------------------------
   Connection row
   ------------------------------------------------------------------------- */

// connectionRow draws the connection the app is looking through, at the
// foot of the sidebar: a plate with the account's mark and its bucket.
func (a *App) connectionRow(c *ui.Context) {
	t := c.Theme()
	ch := chromeOf(t)
	connected := a.session.Connected

	label, detail := "No connection", "Choose a storage"
	if p := a.session.Profile; p != nil {
		label = p.Name
		detail = p.Host()
		if p.Bucket != "" {
			detail = p.Bucket + " · " + shortHost(p.Host())
		}
	}

	row := ui.Row(c).
		Key("connection").
		FillWidth().
		Height(46).
		Padding(0, 8).
		Gap(9).
		AlignItems(ui.Center).
		Radius(9).
		Focusable().
		Cursor(ui.CursorPointer).
		TextColor(t.Text).
		OnClick(func() { a.chipOpen = !a.chipOpen })
	row.Label("Current connection")

	hovered := row.Hovered()
	row = row.Background(ui.Transparent).Border(0, ui.Transparent).
		Draw(func(p *ui.Painter, r ui.Rect) {
			if hovered || a.chipOpen {
				p.Fill(r, ch.hover, 9)
			}
		})

	row.Children(func() {
		if connected && a.session.Profile != nil {
			monogram(c, a.session.Profile.Name, 28)
		} else {
			face := ui.Box(c).Size(28, 28).Radius(8).Center().Shrink(0).
				Background(ui.Transparent).Border(1, t.Border)
			face.Children(func() {
				ui.Icon(c, iconCloud).FontSize(14).TextColor(t.TextMuted)
			})
		}

		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, label).FontSize(typeBody).FontWeight(weightMedium).SingleLine().Ellipsis("…")
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				if connected {
					led(c, t.Success, 5)
				}
				ui.Text(c, detail).FontSize(typeCaption).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
			})
		})

		ui.Icon(c, iconChevronUp).FontSize(12).TextColor(t.TextMuted).Shrink(0)
	})

	ui.Popover(c, row, &a.chipOpen, func() {
		a.connectionMenu(c)
	})
}

// shortHost trims a host to what fits beside a bucket name.
func shortHost(host string) string {
	if len(host) <= 22 {
		return host
	}
	return host[:20] + "…"
}

// connectionMenu fills the connection row's popover: the saved
// connections, then the ways to change them.
func (a *App) connectionMenu(c *ui.Context) {
	ctlMenuPanel(c, 288).Children(func() {
		ui.Column(c).Gap(1).Children(func() {
			if a.session.Connected {
				if menuItem(c, "Disconnect", "close", false, "") {
					a.chipOpen = false
					a.disconnect()
				}
				menuSeparator(c)
			}
			if len(a.connections) > 0 {
				menuHeading(c, "Connections")
				for _, p := range a.connections {
					profile := p
					chosen := a.session.Profile != nil && a.session.Profile.ID == profile.ID
					detail := profile.Host()
					if profile.Bucket != "" {
						detail += " · " + profile.Bucket
					}
					if menuItem(c, profile.Name, "", chosen, detail) {
						a.chipOpen = false
						a.connect(profile.ID)
					}
				}
				menuSeparator(c)
			}
			if menuItem(c, "Add connection…", "plus", false, "") {
				a.chipOpen = false
				a.openConnection(nil)
			}
			if menuItem(c, "Manage connections…", "settings", false, "") {
				a.chipOpen = false
				a.setSection(sectionSettings)
			}
		})
	})
}

/* -------------------------------------------------------------------------
   Top bar
   ------------------------------------------------------------------------- */

// topBar is the strip along the top of the content: where you are on the
// left, and the actions on the right, with the files' own controls on a
// second line.
func (a *App) topBar(c *ui.Context) {
	t := c.Theme()
	ch := chromeOf(t)
	bar := c.TitleBar()

	strip := ui.Column(c).FillWidth().Background(ui.Transparent).Border(0, ui.Transparent)
	strip.Children(func() {
		// The title line: the path, and the actions that apply to it.
		head := ui.Row(c).
			FillWidth().
			Height(headerHeight(bar)).
			Padding(0, max(bar.Right, 18), 0, 18).
			Gap(8).
			AlignItems(ui.Center).
			Background(ui.Transparent).
			Border(0, ui.Transparent).
			DragWindow()
		head.Children(func() {
			a.toolbarLeading(c)
			ui.Spacer(c)
			a.toolbarActions(c)
		})

		// The files' own line: how many, and how they are shown.
		if a.browsing() && (a.section == sectionFiles || a.section == sectionRecent) {
			a.filesToolbar(c)
		}
	})

	strip.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - 1, W: r.W, H: 1}, ch.hairline, 0)
	})
}

// toolbarLeading says where you are: the breadcrumb of the folder, or the
// name of the section.
func (a *App) toolbarLeading(c *ui.Context) {
	t := c.Theme()

	if !a.browsing() {
		ui.Text(c, a.sectionTitle()).
			FontSize(typeHeadline).
			FontWeight(weightMedium).
			TextColor(t.Text).
			Padding(0, 2)
		return
	}

	// A section that is not a folder is named after the bucket it
	// measures, as "magpie › Dashboard".
	if a.section != sectionFiles {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			bucket := ui.ButtonBase(c).Cursor(ui.CursorPointer).Shrink(0).
				Padding(2, 2).Radius(5).TextColor(t.Text).OnClick(func() {
				a.setSection(sectionFiles)
				a.openPrefix("")
			})
			bucket.Children(func() {
				ui.Text(c, a.bucket).FontSize(typeBody).TextColor(t.Text).SingleLine()
			})
			ui.Icon(c, iconChevron).FontSize(12).TextColor(t.TextMuted).Shrink(0)
			if a.section == sectionDashboard {
				ui.Icon(c, iconDashboard).FontSize(15).TextColor(t.Text).Shrink(0)
			} else {
				ui.Icon(c, iconRecent).FontSize(15).TextColor(t.Text).Shrink(0)
			}
			ui.Text(c, a.sectionTitle()).FontSize(typeBody).FontWeight(weightMedium).TextColor(t.Text).SingleLine()
		})
		return
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

// toolbarActions are the things that can be done where you are.
func (a *App) toolbarActions(c *ui.Context) {
	t := c.Theme()

	if a.hasSelection() {
		a.selectionActions(c)
		return
	}

	if a.section == sectionSettings || !a.session.Connected {
		return
	}

	if a.section == sectionRecent {
		if a.hasRecent() {
			clear := ctlButton(c, "")
			clear.Label("Clear history")
			clear.Children(func() {
				ui.Icon(c, iconTrash).FontSize(14).TextColor(t.TextMuted).Shrink(0)
				ui.Text(c, "Clear history").FontSize(typeBody).TextColor(t.TextMuted)
			})
			if clear.Clicked() {
				a.clearRecent()
			}
		}
		return
	}

	if a.section == sectionDashboard {
		recalc := ctlButton(c, "")
		recalc.Label("Recalculate")
		recalc.Children(func() {
			ui.Icon(c, iconRefresh).FontSize(14).TextColor(t.Text).Shrink(0)
			ui.Text(c, "Recalculate").FontSize(typeBody)
		})
		if recalc.Clicked() {
			a.stat = nil
			a.loadStat()
		}
		return
	}

	// New folder, then the one primary action, with its own menu for the
	// other ways to add things.
	newFolder := ctlButton(c, "")
	newFolder.Label("New folder")
	newFolder.Children(func() {
		ui.Icon(c, iconNewFolder).FontSize(15).TextColor(t.Text).Shrink(0)
		if !compactWindow(c) {
			ui.Text(c, "New folder").FontSize(typeBody)
		}
	})
	if compactWindow(c) {
		newFolder.Width(btnHeight).Padding(0, 0)
	}
	if newFolder.Clicked() {
		a.openNewFolder()
	}

	split := ui.Row(c).Height(btnHeight).Shrink(0).Radius(btnRadius).Clip()
	split.Draw(func(p *ui.Painter, r ui.Rect) {
		paintSurface(p, r, btnRadius, chromeOf(t).accentFill, ui.Transparent)
	})
	split.Children(func() {
		upload := ctlButtonKind(c, "", kindAccent)
		upload.Label("Upload")
		upload.Height(btnHeight).Radius(0)
		upload.Children(func() {
			ui.Icon(c, iconUpload).FontSize(14).TextColor(chromeOf(t).accentOnFill).Shrink(0)
			ui.Text(c, "Upload").FontSize(typeBody)
		})
		if upload.Clicked() {
			a.upload()
		}

		ui.Box(c).Width(1).Height(18).Shrink(0).Background(ui.RGBA(255, 255, 255, 0.22))

		options := ui.ButtonBase(c).Size(28, btnHeight).Shrink(0).Center().
			Cursor(ui.CursorPointer).Label("Upload options")
		if options.Clicked() {
			a.uploadMenuOpen = !a.uploadMenuOpen
		}
		options.Children(func() {
			ui.Icon(c, iconChevronDown).FontSize(12).TextColor(chromeOf(t).accentOnFill)
		})
		ui.Popover(c, options, &a.uploadMenuOpen, func() {
			ctlMenuPanel(c, 240).Children(func() {
				ui.Column(c).Gap(1).Children(func() {
					if menuItem(c, "Upload files…", "upload", false, "") {
						a.uploadMenuOpen = false
						a.upload()
					}
					if menuItem(c, "Upload a folder…", "newfolder", false, "") {
						a.uploadMenuOpen = false
						a.uploadFolder()
					}
					menuSeparator(c)
					if menuItem(c, "New folder…", "newfolder", false, "") {
						a.uploadMenuOpen = false
						a.openNewFolder()
					}
					if menuItem(c, "New bucket…", "bucket", false, "") {
						a.uploadMenuOpen = false
						a.openNewBucket()
					}
					menuSeparator(c)
					if menuItem(c, "Copy S3 path", "copy", false, "") {
						a.uploadMenuOpen = false
						a.copyText(a.location(), "S3 path copied")
					}
				})
			})
		})
	})
}

// filesToolbar is the second line of the bar: the count on the left, and
// the controls over the listing on the right.
func (a *App) filesToolbar(c *ui.Context) {
	t := c.Theme()
	items := a.visible()
	recent := a.section == sectionRecent

	row := ui.Row(c).
		FillWidth().
		Height(44).
		Padding(0, 18).
		Gap(8).
		AlignItems(ui.Center).
		Background(ui.Transparent).
		Border(0, ui.Transparent)
	row.Children(func() {
		// Select all, as a checkbox at the head of the listing: the
		// table's own header belongs to the table.
		if len(items) > 0 {
			all := a.allChosen(items)
			box := ctlCheckGlyph(c, "select-all", all, "Select all")
			if box.Clicked() {
				if all {
					a.sel.Clear()
				} else {
					a.selectAll()
				}
			}
		}

		summary := a.summary(items)
		if recent {
			summary = a.summaryRecent()
		}
		if compactWindow(c) && a.searchResults == nil {
			summary = a.shortSummary(items)
		}
		ui.Text(c, summary).
			FontSize(typeBody).
			TextColor(t.TextMuted).
			SingleLine()

		if a.loading {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c).Label("Loading")
			})
		}

		ui.Spacer(c)

		a.typeFilter(c)
		if recent {
			a.recentSort(c)
		} else {
			a.sortMenu(c)
		}

		width := float32(176)
		if compactWindow(c) {
			width = 128
		}
		field := ctlSearchField(c, "filter", &a.search, width, "Filter")
		if field.Changed() {
			a.sel.Clear()
			a.cursor = -1
		}

		if !recent {
			a.modeSwitch(c)
		}
		a.moreMenu(c)
	})
}

// recentSort is the Recent files page's order, which is the order things
// happened in and cannot be changed: the control says so rather than
// offering choices that would not be honored.
func (a *App) recentSort(c *ui.Context) {
	width := float32(136)
	if compactWindow(c) {
		width = 118
	}
	btn := ctlMenuButton(c, "Order", "Recently used", false, width)
	btn.Disabled(true)
}

// summaryRecent is the count on the Recent files page.
func (a *App) summaryRecent() string {
	items := a.visibleRecent()
	var bytes int64
	for _, r := range items {
		bytes += r.Size
	}
	return fmt.Sprintf("%s · %s", plural(len(items), "item", "items"), formatBytes(bytes))
}

// shortSummary is the count for a narrow window, where the full line of
// figures would be cut off: "1 folder", "3 items".
func (a *App) shortSummary(items []store.Entry) string {
	if a.loading && len(items) == 0 {
		return "Loading…"
	}
	folders, files := 0, 0
	for _, e := range items {
		if e.Folder {
			folders++
			continue
		}
		files++
	}
	var parts []string
	if folders > 0 {
		parts = append(parts, plural(folders, "folder", "folders"))
	}
	if files > 0 {
		parts = append(parts, plural(files, "item", "items"))
	}
	if len(parts) == 0 {
		return "Empty"
	}
	return strings.Join(parts, " · ")
}

// summary describes what is being shown: "1 folder", "3 items · 1.6 GB".
func (a *App) summary(items []store.Entry) string {
	if a.searchResults != nil {
		return plural(len(items), "result", "results") + " · " + a.bucket
	}
	if a.loading && len(items) == 0 {
		return "Loading…"
	}
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
	var parts []string
	if folders > 0 {
		parts = append(parts, plural(folders, "folder", "folders"))
	}
	if files > 0 {
		parts = append(parts, plural(files, "file", "files"))
	}
	if bytes > 0 {
		parts = append(parts, formatBytes(bytes))
	}
	if len(parts) == 0 {
		return "Empty"
	}
	return strings.Join(parts, " · ")
}

// typeFilter is the All types menu.
func (a *App) typeFilter(c *ui.Context) {
	width := float32(116)
	if compactWindow(c) {
		width = 104
	}
	btn := ctlMenuButton(c, "Type", a.filterLabel(), a.typeOpen, width)
	if btn.Clicked() {
		a.typeOpen = !a.typeOpen
	}
	ui.Popover(c, btn, &a.typeOpen, func() {
		ctlMenuPanel(c, 188).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				for _, f := range typeFilters {
					filter := f
					if menuItem(c, filter.Label, "", a.filter == filter.Kind, "") {
						a.typeOpen = false
						a.filter = filter.Kind
						a.sel.Clear()
					}
				}
			})
		})
	})
}

// sortMenu is the sort control: the field, and its direction.
func (a *App) sortMenu(c *ui.Context) {
	width := float32(122)
	if compactWindow(c) {
		width = 110
	}
	label := a.sortLabel()
	if a.sortDesc {
		label += " ↓"
	} else {
		label += " ↑"
	}
	btn := ctlMenuButton(c, "Sort", label, a.sortOpen, width)
	if btn.Clicked() {
		a.sortOpen = !a.sortOpen
	}
	ui.Popover(c, btn, &a.sortOpen, func() {
		ctlMenuPanel(c, 196).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				for _, option := range sortOptions {
					opt := option
					chosen := a.sort == opt.Field
					detail := ""
					if chosen {
						if a.sortDesc {
							detail = "Descending"
						} else {
							detail = "Ascending"
						}
					}
					if menuItem(c, opt.Label, "", chosen, detail) {
						if chosen {
							a.sortDesc = !a.sortDesc
						} else {
							a.sort = opt.Field
							a.sortDesc = false
						}
						a.sortOpen = false
					}
				}
			})
		})
	})
}

// modeSwitch is the grid/list pair, drawn as one channel.
func (a *App) modeSwitch(c *ui.Context) {
	t := c.Theme()
	ch := chromeOf(t)

	track := ui.Row(c).Height(btnHeight).Shrink(0).Padding(2).Radius(btnRadius).
		Gap(0).Background(ui.Transparent).Border(0, ui.Transparent)
	track.Draw(func(p *ui.Painter, r ui.Rect) {
		paintSurface(p, r, btnRadius, ch.inset, ui.Transparent)
	})
	track.Children(func() {
		gridOn := a.mode == viewGrid
		grid := ctlSegment(c, &gridOn, "Grid view")
		grid.Width(30).Height(28)
		grid.Children(func() {
			ui.Icon(c, iconGrid).FontSize(14).TextColor(segmentTint(gridOn, t)).Shrink(0)
		})
		grid.OnClick(func() {
			gridOn = true
			a.setViewMode(viewGrid)
		})

		listOn := a.mode == viewList
		list := ctlSegment(c, &listOn, "List view")
		list.Width(30).Height(28)
		list.Children(func() {
			ui.Icon(c, iconList).FontSize(14).TextColor(segmentTint(listOn, t)).Shrink(0)
		})
		list.OnClick(func() {
			listOn = true
			a.setViewMode(viewList)
		})
	})
}

// segmentTint keeps a segment's glyph in step with its own label color.
func segmentTint(on bool, t *ui.Theme) ui.Color {
	if on {
		return t.Text
	}
	return t.TextMuted
}

// moreMenu holds the bucket-wide actions that have no place of their own.
func (a *App) moreMenu(c *ui.Context) {
	t := c.Theme()

	btn := ctlIconButtonBare(c, "More options")
	btn.Children(func() {
		ui.Icon(c, iconSparkle).FontSize(15).TextColor(t.TextMuted)
	})
	if btn.Clicked() {
		a.menuOpen = !a.menuOpen
	}
	ui.Popover(c, btn, &a.menuOpen, func() {
		ctlMenuPanel(c, 226).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				if menuItem(c, "New folder…", "newfolder", false, "") {
					a.menuOpen = false
					a.openNewFolder()
				}
				if menuItem(c, "New bucket…", "bucket", false, "") {
					a.menuOpen = false
					a.openNewBucket()
				}
				menuSeparator(c)
				if menuItem(c, "Measure storage", "dashboard", false, "") {
					a.menuOpen = false
					a.setSection(sectionDashboard)
					a.stat = nil
					a.loadStat()
				}
				if menuItem(c, "Copy S3 path", "copy", false, "") {
					a.menuOpen = false
					a.copyText(a.location(), "S3 path copied")
				}
				menuSeparator(c)
				if menuItem(c, "Refresh", "refresh", false, "⌘R") {
					a.menuOpen = false
					a.refresh()
				}
			})
		})
	})

	refresh := ctlIconButtonBare(c, "Refresh")
	refresh.Children(func() {
		ui.Icon(c, iconRefresh).FontSize(15).TextColor(t.TextMuted)
	})
	if refresh.Clicked() {
		a.refresh()
	}
}

// selectionActions act on what is chosen, and replace the usual actions
// while something is.
func (a *App) selectionActions(c *ui.Context) {
	t := c.Theme()
	count := a.sel.Len()

	tally := ui.Row(c).Gap(5).AlignItems(ui.Center).Padding(0, 9).Height(btnHeight).
		Radius(btnRadius).Shrink(0).Background(chromeOf(t).inset)
	tally.Children(func() {
		ui.Text(c, plural(count, "item", "items")).
			FontSize(typeBody).
			FontWeight(weightMedium).
			TextColor(t.Text).
			SingleLine()
	})

	download := ctlButton(c, "")
	download.Label("Download")
	download.Children(func() {
		ui.Icon(c, iconDownload).FontSize(14).TextColor(t.Text).Shrink(0)
		if !compactWindow(c) {
			ui.Text(c, "Download").FontSize(typeBody)
		}
	})
	if download.Clicked() {
		a.download()
	}

	move := ctlButton(c, "")
	move.Label("Move")
	move.Children(func() {
		ui.Icon(c, iconMove).FontSize(14).TextColor(t.Text).Shrink(0)
		if !compactWindow(c) {
			ui.Text(c, "Move…").FontSize(typeBody)
		}
	})
	if move.Clicked() {
		a.openMove()
	}

	del := ctlButton(c, "")
	del.Label("Delete")
	del.Children(func() {
		ui.Icon(c, iconTrash).FontSize(14).TextColor(t.Danger).Shrink(0)
	})
	if del.Clicked() {
		a.deleteChosen()
	}

	done := ctlIconButtonBare(c, "Deselect")
	done.Children(func() {
		ui.Icon(c, iconClose).FontSize(14).TextColor(t.TextMuted)
	})
	if done.Clicked() {
		a.sel.Clear()
	}
}

/* -------------------------------------------------------------------------
   Content
   ------------------------------------------------------------------------- */

// sectionTitle is what the toolbar says when there is no path to show.
func (a *App) sectionTitle() string {
	switch {
	case a.section == sectionSettings:
		return "Settings"
	case a.section == sectionRecent:
		return "Recent files"
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
			inner := ui.Rect{X: r.X + 8, Y: r.Y + 8, W: r.W - 16, H: r.H - 16}
			p.Fill(inner, t.Accent.Alpha(0.05), 12)
			p.Stroke(inner, t.Accent, 12, 2)
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
		case a.section == sectionRecent:
			a.recentFiles(c)
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
		Padding(8, 18).
		Gap(9).
		AlignItems(ui.Center).
		Background(ui.Transparent).
		Border(0, ui.Transparent)
	banner.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, t.Danger.Alpha(0.08), 0)
		p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - 1, W: r.W, H: 1}, t.Danger.Alpha(0.24), 0)
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
   Empty states
   ------------------------------------------------------------------------- */

// emptyState fills a content plane that has nothing to show, and says
// what to do about it.
func emptyState(c *ui.Context, glyph *ui.SVG, title, message string, actions func()) {
	t := c.Theme()
	ch := chromeOf(t)

	ui.Column(c).Fill().Center().Gap(14).Padding(40).Children(func() {
		mark := ui.Box(c).Size(56, 56).Radius(16).Center().Shrink(0).
			Background(ui.Transparent).Border(0, ui.Transparent)
		mark.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(r, ch.inset, 16)
		})
		mark.Children(func() {
			ui.Icon(c, glyph).FontSize(26).TextColor(t.TextMuted)
		})

		ui.Column(c).Gap(5).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).FontSize(typeHeadline).FontWeight(weightMedium).TextColor(t.Text)
			ui.Text(c, message).
				FontSize(typeBody).
				TextColor(t.TextMuted).
				TextAlign(ui.Center).
				MaxWidth(360)
		})

		if actions != nil {
			ui.Row(c).Gap(8).Children(actions)
		}
	})
}

// skeletons stands in for content that has not arrived yet.
func skeletons(c *ui.Context) {
	ch := chromeOf(c.Theme())

	ui.Column(c).FillWidth().Padding(18).Gap(8).Children(func() {
		for i := range 7 {
			ui.Box(c).Key(i).Height(34).Radius(btnRadius).Background(ui.Transparent).
				Draw(func(p *ui.Painter, r ui.Rect) {
					p.Fill(r, ch.inset, btnRadius)
				})
		}
	})
}

func entryIcon(e store.Entry) *ui.SVG { return kindIcon(kindOf(e.Key, e.Folder)) }

/* -------------------------------------------------------------------------
   Welcome screen
   ------------------------------------------------------------------------- */

func (a *App) welcome(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Fill().Center().Gap(18).Padding(40).Children(func() {
		ui.Column(c).Gap(8).AlignItems(ui.Center).Children(func() {
			mark := ui.Box(c).Size(64, 64).Radius(18).Center().Shrink(0).
				Background(ui.Transparent).Border(0, ui.Transparent)
			mark.Draw(func(p *ui.Painter, r ui.Rect) {
				p.Fill(r, t.Accent, 18)
			})
			mark.Children(func() {
				ui.Icon(c, iconCloud).FontSize(32).TextColor(ui.Hex("#FFFFFF"))
			})

			ui.Text(c, "Add a connection").
				FontSize(typeTitle).
				FontWeight(weightSemi).
				TextColor(t.Text).
				Margin(0, 0, 4, 0)
			ui.Text(c, "Connect a bucket with an access key, or sign in to Cloudflare R2.").
				FontSize(typeBody).
				TextColor(t.TextMuted).
				TextAlign(ui.Center).
				MaxWidth(400)
		})

		ui.Column(c).Width(320).Gap(8).Children(func() {
			connect := ctlPrimary(c, "Connect a bucket")
			connect.Width(320)
			if connect.Clicked() {
				a.openConnection(nil)
			}

			if len(a.connections) > 0 {
				label := plural(len(a.connections), "Saved connection", "Saved connections")
				recent := ctlButton(c, label+"…")
				recent.Width(320)
				if recent.Clicked() {
					a.chipOpen = true
				}
			}
		})

		ui.Column(c).Width(420).Gap(10).Margin(10, 0, 0, 0).Children(func() {
			for _, point := range []struct {
				glyph *ui.SVG
				tint  ui.Color
				title string
				body  string
			}{
				{iconFiles, ui.Hex("#3B7DF6"), "Browse like a drive", "Switch between grid and list views, even for large buckets."},
				{iconDashboard, ui.Hex("#8B5CF6"), "See your storage", "A living dashboard of what's in your bucket, by type and size."},
				{iconShare, ui.Hex("#16A34A"), "Share in a click", "Presigned links with an expiry, or your own public domain."},
			} {
				p := point
				ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
					iconPlate(c, p.glyph, p.tint, 34)
					ui.Column(c).Grow(1).Gap(1).Children(func() {
						ui.Text(c, p.title).FontSize(typeBody).FontWeight(weightMedium).TextColor(t.Text)
						ui.Text(c, p.body).FontSize(typeCaption).TextColor(t.TextMuted)
					})
				})
			}
		})
	})
}
