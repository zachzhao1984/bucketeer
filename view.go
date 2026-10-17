package main

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
)

// The views. Each is a function of the app's state, built again every
// frame; they never do work themselves, they only read state and report
// input back through the app's actions.

// sidebarWidth is how wide the sidebar is, in DIPs.
const sidebarWidth = 232

// view builds the whole window: the sidebar, the bar along the top, the
// part the section shows, and the transfers panel under it.
func (a *App) view(c *ui.Context) {
	a.services = c.Services()
	c.SetTheme(a.theme(c.Theme().Dark))

	// A message queued by an action is shown once and forgotten, so a
	// result replaces the one before it.
	if a.notice != "" {
		c.Toast(a.notice)
		a.notice = ""
	}

	c.OnShortcut(ui.Cmd, ui.KeyR, a.refresh)
	c.OnShortcut(ui.Cmd, ui.KeyO, a.upload)
	c.OnShortcut(ui.Cmd, ui.KeyT, func() { a.transfersOpen = !a.transfersOpen })

	bar := c.TitleBar()
	ui.Row(c).Fill().Gap(8).Padding(8).Children(func() {
		a.sidebar(c, bar)
		ui.Column(c).Grow(1).FillHeight().Gap(8).Children(func() {
			a.topBar(c, bar)
			a.content(c)
			if a.transfersOpen {
				a.transfersPanel(c)
			}
		})
	})

	a.dialogs(c)
}

// headerHeight is how tall the band along the top of the window is: the
// room the window controls take, and enough for a row of buttons. The
// controls are hidden in full screen, where MyGo reports no room, so the
// band keeps the height that holds the toolbar.
func headerHeight(bar ui.TitleBar) float32 { return max(bar.Height, 52) }

// theme returns the theme for the system's appearance. The two are built
// once and kept, so choosing one every frame costs nothing.
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

/* -------------------------------------------------------------------------
   Sidebar
   ------------------------------------------------------------------------- */

// The IDs the sidebar chooses between. A section or a folder is one item,
// so the sidebar's own choice follows where the app is looking.
const (
	navDashboard = "section:dashboard"
	navFiles     = "section:files"
	navSettings  = "section:settings"
	navTransfers = "section:transfers"
)

// sidebar draws the navigation, the folders of the current location, and
// the connection the app is using. The chip is last in a column that
// grows, so it stays at the bottom whatever the folders do.
func (a *App) sidebar(c *ui.Context, bar ui.TitleBar) {
	t := c.Theme()
	a.syncSidebarChoice()

	ui.Column(c).
		Width(sidebarWidth).
		FillHeight().
		Background(t.Surface).
		Border(1, t.Border).
		Radius(t.Radius).
		Clip().
		Padding(0, 0, 8, 0).
		Children(func() {
			// The band under the window controls. Dragging it moves the
			// window, which is what a title bar does.
			ui.Row(c).Height(headerHeight(bar)).DragWindow()

			sidebar := ui.Sidebar(c, &a.sidebarID, func() {
				ui.SidebarItem(c, navDashboard, iconDashboard, "Dashboard")
				ui.SidebarItem(c, navFiles, iconFiles, "All files")
				ui.SidebarItem(c, navSettings, iconSettings, "Settings")

				if folders := a.sidebarFolders(); len(folders) > 0 {
					ui.SidebarSection(c, a.folderSectionTitle(), &a.foldersOpen, func() {
						for _, e := range folders {
							ui.SidebarItem(c, "folder:"+e.Key, iconFolder, e.Name)
						}
					})
				}
			}).Grow(1)
			if sidebar.Changed() {
				a.sidebarChose()
			}

			ui.Row(c).PaddingX(12).PaddingY(6).Children(func() {
				ui.Divider(c)
			})

			// Transfers, with a count of what is still going.
			transfers := ui.Sidebar(c.Key("transfers"), &a.transfersChoice, func() {
				ui.SidebarItem(c, navTransfers, iconTransfer, "Transfers").Children(func() {
					if n := a.activeTransfers(); n > 0 {
						ui.Badge(c, formatCount(n))
					}
				})
			}).Width(sidebarWidth - 16)
			if transfers.Changed() && a.transfersChoice == navTransfers {
				a.transfersOpen = !a.transfersOpen
				a.transfersChoice = ""
			}

			a.connectionChip(c)
		})
}

// syncSidebarChoice points the sidebar's own selection at where the app is
// looking, so a folder opened from anywhere shows as chosen.
func (a *App) syncSidebarChoice() {
	switch {
	case a.section == sectionDashboard:
		a.sidebarID = navDashboard
	case a.section == sectionSettings:
		a.sidebarID = navSettings
	case a.browsing() && a.prefix != "":
		a.sidebarID = "folder:" + a.prefix
	default:
		a.sidebarID = navFiles
	}
}

// sidebarFolders returns the folders of the place being browsed: the
// bucket's folders when the root is open, the folders of the current
// folder otherwise.
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

// folderSectionTitle heads the sidebar's folder list: the folder whose
// contents are listed, or the bucket when at its root.
func (a *App) folderSectionTitle() string {
	if a.prefix == "" {
		return a.bucket
	}
	return a.folderName()
}

// sidebarChose acts on the item the user clicked in the sidebar.
func (a *App) sidebarChose() {
	id := a.sidebarID
	switch {
	case id == navDashboard:
		a.setSection(sectionDashboard)
	case id == navFiles:
		a.setSection(sectionFiles)
	case id == navSettings:
		a.setSection(sectionSettings)
	case strings.HasPrefix(id, "folder:"):
		a.section = sectionFiles
		a.openPrefix(strings.TrimPrefix(id, "folder:"))
	}
}

/* -------------------------------------------------------------------------
   Connection chip
   ------------------------------------------------------------------------- */

// connectionChip draws the chip at the bottom of the sidebar, which shows
// what the app is connected to and opens the list of connections.
func (a *App) connectionChip(c *ui.Context) {
	t := c.Theme()
	label, detail := "Not connected", "Add a connection"
	var icon *ui.SVG = iconCloud
	if p := a.session.Profile; p != nil {
		label = p.Name
		detail = a.session.Host
		if p.Bucket != "" {
			detail += " · " + p.Bucket
		}
	}

	chip := ui.Row(c).
		Margin(0, sidebarHorizontalInset).
		Padding(8).
		Gap(10).
		AlignItems(ui.Center).
		Radius(t.Radius).
		Focusable().
		Cursor(ui.CursorPointer).
		OnClick(func() { a.chipOpen = !a.chipOpen })
	if chip.Hovered() || a.chipOpen {
		chip.Background(t.SurfaceHover)
	}
	chip.Children(func() {
		if p := a.session.Profile; p != nil {
			ui.Avatar(c, p.Name, nil).FontSize(11)
		} else {
			ui.Box(c).Size(28, 28).Radius(14).Background(t.SurfaceHover).Center().Children(func() {
				ui.Icon(c, icon).FontSize(15).TextColor(t.TextMuted)
			})
		}
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, label).FontSize(12.5).Bold().SingleLine().Ellipsis("…")
			ui.Text(c, detail).FontSize(11).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
		})
		ui.Icon(c, iconMore).FontSize(14).TextColor(t.TextMuted)
	})

	ui.Popover(c, chip, &a.chipOpen, func() {
		a.connectionMenu(c)
	})
}

// sidebarHorizontalInset is the room the chip keeps from the sidebar's
// edges, so it lines up with the items above it.
const sidebarHorizontalInset float32 = 8

// connectionMenu fills the chip's popover: what is connected, the saved
// connections, and the ways to change them.
func (a *App) connectionMenu(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(260).Gap(2).Children(func() {
		if a.svc.Connected() {
			if menuItem(c, "Disconnect", "close") {
				a.chipOpen = false
				a.disconnect()
			}
			divider(c)
		}
		if len(a.connections) > 0 {
			ui.Text(c, "Connections").FontSize(11).Bold().TextColor(t.TextMuted).PaddingX(8).PaddingY(4)
			for _, p := range a.connections {
				profile := p
				chosen := a.session.Profile != nil && a.session.Profile.ID == profile.ID
				if menuItemChecked(c, profile.Name, profile.Host(), chosen) {
					a.chipOpen = false
					a.connect(profile.ID)
				}
			}
			divider(c)
		}
		if menuItem(c, "Add a connection…", "plus") {
			a.chipOpen = false
			a.openConnection(nil)
		}
		if menuItem(c, "Manage connections…", "settings") {
			a.chipOpen = false
			a.section = sectionSettings
		}
	})
}

/* -------------------------------------------------------------------------
   Small shared pieces
   ------------------------------------------------------------------------- */

// menuItem draws one row of a popover menu, and reports a click. It takes
// an icon name from the app's set.
func menuItem(c *ui.Context, label, iconName string) bool {
	return menuItemChecked(c, label, "", false, iconName)
}

// menuItemChecked draws a menu row with room for a check, an icon, a
// label and a detail line, and reports a click.
func menuItemChecked(c *ui.Context, label, detail string, checked bool, iconName ...string) bool {
	t := c.Theme()
	row := ui.Row(c).Key(label).Padding(6, 8).Gap(8).AlignItems(ui.Center).Radius(t.Radius).
		Focusable().Cursor(ui.CursorPointer).OnClick(func() {})
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	clicked := row.Clicked()
	row.Children(func() {
		ui.Box(c).Size(16, 16).Children(func() {
			if checked {
				ui.Icon(c, iconCheck).FontSize(14).TextColor(t.Accent)
			}
		})
		if len(iconName) > 0 && iconName[0] != "" {
			ui.Icon(c, iconFor(iconName[0])).FontSize(14).TextColor(t.TextMuted)
		}
		if detail == "" {
			ui.Text(c, label).FontSize(12.5).Grow(1).SingleLine()
		} else {
			ui.Column(c).Grow(1).Gap(1).Children(func() {
				ui.Text(c, label).FontSize(12.5).SingleLine().Ellipsis("…")
				ui.Text(c, detail).FontSize(11).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
			})
		}
	})
	return clicked
}

// divider draws a separating line inside a popover.
func divider(c *ui.Context) {
	ui.Row(c).PaddingY(4).PaddingX(4).Children(func() { ui.Divider(c) })
}

// iconFor returns an icon of the app's set by name, for the places that
// take a name rather than a parsed SVG.
func iconFor(name string) *ui.SVG {
	switch name {
	case "dashboard":
		return iconDashboard
	case "files":
		return iconFiles
	case "settings":
		return iconSettings
	case "bucket":
		return iconBucket
	case "plus":
		return iconPlus
	case "close":
		return iconClose
	case "check":
		return iconCheck
	case "folder":
		return iconFolder
	case "upload":
		return iconUpload
	case "download":
		return iconDownload
	case "trash":
		return iconTrash
	case "rename":
		return iconRename
	case "move":
		return iconMove
	case "share":
		return iconShare
	case "link":
		return iconLink
	case "copy":
		return iconCopy
	case "eye":
		return iconEye
	case "external":
		return iconExternal
	case "refresh":
		return iconRefresh
	case "search":
		return iconSearch
	case "grid":
		return iconGrid
	case "list":
		return iconList
	case "up":
		return iconUp
	case "more":
		return iconMore
	case "info":
		return iconInfo
	case "alert":
		return iconAlert
	case "transfer":
		return iconTransfer
	case "cloud":
		return iconCloud
	case "newfile":
		return iconNewFile
	default:
		return iconOther
	}
}
