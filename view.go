package main

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
)

// sidebarWidth is how wide the sidebar is, in DIPs.
const sidebarWidth = 230

// view builds the whole window: the sidebar, the bar along the top, the
// part the section shows, and the transfers panel under it.
func (a *App) view(c *ui.Context) {
	a.services = c.Services()
	t := a.theme(c.Theme().Dark)
	c.SetTheme(t)

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

// headerHeight is how tall the band along the top of the window is.
func headerHeight(bar ui.TitleBar) float32 { return max(bar.Height, 52) }

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

const (
	navDashboard = "section:dashboard"
	navFiles     = "section:files"
	navTransfers = "section:transfers"
	navSettings  = "section:settings"
)

// sidebar draws the navigation, folders, and connection status.
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
		Padding(0, 0, 10, 0).
		Children(func() {
			// Title bar drag region with subtle brand mark
			ui.Row(c).
				Height(headerHeight(bar)).
				Padding(0, 14, 0, max(bar.Left, 18)).
				AlignItems(ui.Center).
				DragWindow().
				Children(func() {
					ui.Text(c, "BUCKETEER").
						FontSize(11).
						Bold().
						LetterSpacing(1.2).
						TextColor(t.TextMuted)
				})

			// Primary navigation items
			sidebar := ui.Sidebar(c, &a.sidebarID, func() {
				ui.SidebarItem(c, navDashboard, iconDashboard, "Dashboard")
				ui.SidebarItem(c, navFiles, iconFiles, "All files")
				ui.SidebarItem(c, navTransfers, iconTransfer, "Transfers").Children(func() {
					if n := a.activeTransfers(); n > 0 {
						ui.Badge(c, formatCount(n))
					}
				})
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

			a.connectionChip(c)
		})
}

// syncSidebarChoice points the sidebar's own selection at where the app is looking.
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

func (a *App) folderSectionTitle() string {
	if a.prefix == "" {
		return a.bucket
	}
	return a.folderName()
}

// sidebarChose acts on the chosen sidebar item.
func (a *App) sidebarChose() {
	id := a.sidebarID
	switch {
	case id == navDashboard:
		a.setSection(sectionDashboard)
	case id == navFiles:
		a.setSection(sectionFiles)
	case id == navTransfers:
		a.transfersOpen = !a.transfersOpen
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

const sidebarHorizontalInset float32 = 8

// connectionChip draws the connection status card at the bottom of the sidebar.
func (a *App) connectionChip(c *ui.Context) {
	t := c.Theme()
	connected := a.session.Connected
	label, detail := "Not connected", "Connect storage…"
	if p := a.session.Profile; p != nil {
		label = p.Name
		detail = a.session.Host
		if p.Bucket != "" {
			detail += " · " + p.Bucket
		}
	}

	chip := ui.Row(c).
		Margin(0, sidebarHorizontalInset).
		Padding(8, 10).
		Gap(10).
		AlignItems(ui.Center).
		Radius(8).
		Border(1, t.Border).
		Background(t.Surface).
		Focusable().
		Cursor(ui.CursorPointer).
		OnClick(func() { a.chipOpen = !a.chipOpen })
	if chip.Hovered() || a.chipOpen {
		chip.Background(t.SurfaceHover)
	}

	chip.Children(func() {
		// Status avatar with online indicator
		if connected && a.session.Profile != nil {
			ui.Avatar(c, a.session.Profile.Name, nil).FontSize(10.5)
		} else {
			ui.Box(c).Size(26, 26).Radius(13).Background(t.SurfaceHover).Center().Children(func() {
				ui.Icon(c, iconCloud).FontSize(14).TextColor(t.TextMuted)
			})
		}

		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				statusDot := t.TextMuted.Alpha(0.5)
				if connected {
					statusDot = t.Success
				}
				ui.Box(c).Size(6, 6).Radius(3).Background(statusDot)
				ui.Text(c, label).FontSize(12).Bold().SingleLine().Ellipsis("…")
			})
			ui.Text(c, detail).FontSize(10.5).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
		})

		ui.Icon(c, iconMore).FontSize(13).TextColor(t.TextMuted)
	})

	ui.Popover(c, chip, &a.chipOpen, func() {
		a.connectionMenu(c)
	})
}

// connectionMenu fills the connection chip's popover menu.
func (a *App) connectionMenu(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(260).Gap(2).Padding(4).Children(func() {
		if a.session.Connected {
			if menuItem(c, "Disconnect", "close") {
				a.chipOpen = false
				a.disconnect()
			}
			divider(c)
		}
		if len(a.connections) > 0 {
			ui.Text(c, "Connections").FontSize(11).Bold().TextColor(t.TextMuted).Padding(6, 8, 2, 8)
			for _, p := range a.connections {
				profile := p
				chosen := a.session.Profile != nil && a.session.Profile.ID == profile.ID
				host := profile.Host()
				if profile.Bucket != "" {
					host += " · " + profile.Bucket
				}
				if menuItemChecked(c, profile.Name, host, chosen) {
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
   Menu helpers
   ------------------------------------------------------------------------- */

func menuItem(c *ui.Context, label, iconName string) bool {
	return menuItemChecked(c, label, "", false, iconName)
}

func menuItemChecked(c *ui.Context, label, detail string, checked bool, iconName ...string) bool {
	t := c.Theme()
	row := ui.Row(c).Key(label).Padding(6, 8).Gap(8).AlignItems(ui.Center).Radius(6).
		Focusable().Cursor(ui.CursorPointer).OnClick(func() {})
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	clicked := row.Clicked()
	row.Children(func() {
		ui.Box(c).Size(16, 16).Center().Children(func() {
			if checked {
				ui.Icon(c, iconCheck).FontSize(13).TextColor(t.Accent)
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

func divider(c *ui.Context) {
	ui.Row(c).PaddingY(4).PaddingX(4).Children(func() { ui.Divider(c) })
}

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
