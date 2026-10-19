package main

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
)

// sidebarWidth is how wide the sidebar is, in DIPs.
const sidebarWidth = 220

// view builds the whole window: the sidebar and the bar along the top,
// both on the window's material, and the content plane under them.
func (a *App) view(c *ui.Context) {
	a.services = c.Services()
	t := a.theme(c.Theme().Dark)
	c.SetTheme(t)
	ch := chromeOf(t)

	if a.notice != "" {
		c.Toast(a.notice)
		a.notice = ""
	}

	c.OnShortcut(ui.Cmd, ui.KeyR, a.refresh)
	c.OnShortcut(ui.Cmd, ui.KeyO, a.upload)
	c.OnShortcut(ui.Cmd, ui.KeyT, func() { a.transfersOpen = !a.transfersOpen })

	ui.Row(c).Fill().Children(func() {
		a.sidebar(c)

		// The content plane: the bar along the top, the section it
		// shows, and the transfers under it. A single hairline down the
		// side is all that separates the two columns.
		plane := ui.Column(c).Grow(1).FillHeight().Background(ch.canvas)
		plane.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(ui.Rect{X: r.X, Y: r.Y, W: 1, H: r.H}, ch.hairline, 0)
		})
		plane.Children(func() {
			a.topBar(c)
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

// compactWindow reports whether the window is too narrow for the sidebar
// to show its name and the search field at full size, as at its least
// width.
func compactWindow(c *ui.Context) bool {
	w, _ := c.Size()
	return w < 980
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
	navTransfers = "section:transfers"
	navSettings  = "section:settings"
)

// sidebar draws the navigation and the connection it is looking through.
// It sits on the window's material, full height, with nothing under it
// but a hairline.
func (a *App) sidebar(c *ui.Context) {
	t := c.Theme()
	a.syncSidebarChoice()

	bar := c.TitleBar()
	pane := ui.Column(c).
		Width(sidebarWidth).
		FillHeight().
		Padding(0, 0, 8, 0).
		Background(ui.Transparent)
	pane = paintMaterial(c, pane)
	if compactWindow(c) {
		pane.Width(190)
	}

	pane.Children(func() {
		// The window's drag region, with the app's mark on it.
		ui.Row(c).
			Height(headerHeight(bar)).
			Padding(0, 10, 0, trafficInset(bar, 14)).
			AlignItems(ui.Center).
			DragWindow().
			Children(func() {
				a.brandMark(c)
				ui.Text(c, appName).
					FontSize(typeBody).
					FontWeight(weightSemi).
					TextColor(t.TextMuted)
			})

		ui.Column(c).Padding(6, 8, 0, 8).Gap(1).FocusGroup(ui.Vertical).Children(func() {
			navRow(c, navDashboard, "Dashboard", iconDashboard, a.sidebarID == navDashboard, nil, func() {
				a.setSection(sectionDashboard)
			})
			navRow(c, navFiles, "Files", iconFiles, a.sidebarID == navFiles, nil, func() {
				a.setSection(sectionFiles)
			})
			navRow(c, navTransfers, "Transfers", iconTransfer, false, func() {
				if n := a.activeTransfers(); n > 0 {
					ctlBadge(c, formatCount(n), t.Accent)
				}
			}, func() {
				a.transfersOpen = !a.transfersOpen
			})
			navRow(c, navSettings, "Settings", iconSettings, a.sidebarID == navSettings, nil, func() {
				a.setSection(sectionSettings)
			})
		})

		// The folders of the browsed place, as deep as the sidebar
		// goes: one level below where it is looking.
		ui.Column(c).Grow(1).Children(func() {
			if folders := a.sidebarFolders(); len(folders) > 0 {
				sidebarLabel(c, a.folderSectionTitle())
				ui.Column(c).Padding(0, 2, 0, 8).Gap(1).FocusGroup(ui.Vertical).Children(func() {
					for _, e := range folders {
						entry := e
						id := "folder:" + entry.Key
						navRow(c, id, entry.Name, iconFolder, a.sidebarID == id, nil, func() {
							a.setSection(sectionFiles)
							a.openPrefix(entry.Key)
						})
					}
				})
			}
		})

		a.connectionRow(c)
	})
}

// sidebarLabel heads a group in the sidebar.
func sidebarLabel(c *ui.Context, title string) {
	t := c.Theme()
	ui.Text(c, strings.ToUpper(title)).
		FontSize(typeCaption).
		FontWeight(weightSemi).
		LetterSpacing(0.4).
		TextColor(t.TextMuted).
		Padding(8, 10, 3, 10).
		SingleLine().
		Ellipsis("…")
}

// brandMark is a small square with the app's glyph on it.
func (a *App) brandMark(c *ui.Context) {
	t := c.Theme()
	ch := chromeOf(t)

	mark := ui.Box(c).Size(20, 20).Radius(5).Center().Shrink(0).
		Background(ui.Transparent).Border(0, ui.Transparent)
	mark.Draw(func(p *ui.Painter, r ui.Rect) {
		paintSurface(p, r, 5, t.Accent, ui.Transparent)
	})
	mark.Children(func() {
		ui.Icon(c, iconCloud).FontSize(12).TextColor(ch.accentOnFill)
	})
}

// syncSidebarChoice points the sidebar's own selection at where the app
// is looking.
func (a *App) syncSidebarChoice() {
	switch {
	case a.section == sectionSettings:
		a.sidebarID = navSettings
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
		a.setSection(sectionFiles)
		a.openPrefix(strings.TrimPrefix(id, "folder:"))
	}
}

/* -------------------------------------------------------------------------
   Connection row
   ------------------------------------------------------------------------- */

// connectionRow draws the connection the app is looking through, at the
// foot of the sidebar.
func (a *App) connectionRow(c *ui.Context) {
	t := c.Theme()
	ch := chromeOf(t)
	connected := a.session.Connected

	label, detail := "Not connected", "Choose a connection…"
	if p := a.session.Profile; p != nil {
		label = p.Name
		detail = a.session.Host
		if p.Bucket != "" {
			detail += " · " + p.Bucket
		}
	}

	ui.Row(c).Grow(1).Height(1).Padding(0, 10, 0, 10).AlignItems(ui.End).Children(func() {
		rule(c, 0, ch.hairline)
	})

	row := ui.Row(c).
		Key("connection").
		Margin(6, 6, 0, 6).
		Padding(5, 7).
		Gap(8).
		AlignItems(ui.Center).
		Radius(btnRadius).
		Focusable().
		Cursor(ui.CursorPointer).
		TextColor(t.Text).
		OnClick(func() { a.chipOpen = !a.chipOpen })

	hovered := row.Hovered()
	row = row.Background(ui.Transparent).Border(0, ui.Transparent).
		Draw(func(p *ui.Painter, r ui.Rect) {
			if hovered || a.chipOpen {
				p.Fill(r, ch.hover, btnRadius)
			}
		})

	row.Children(func() {
		if connected && a.session.Profile != nil {
			monogram(c, a.session.Profile.Name, 26)
		} else {
			face := ui.Box(c).Size(26, 26).Radius(13).Center().Shrink(0).
				Background(ui.Transparent).Border(1, t.Border)
			face.Children(func() {
				ui.Icon(c, iconCloud).FontSize(14).TextColor(t.TextMuted)
			})
		}

		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				dot := t.TextMuted
				if connected {
					dot = t.Success
				}
				led(c, dot, 6)
				ui.Text(c, label).FontSize(typeCaption).FontWeight(weightSemi).SingleLine().Ellipsis("…")
			})
			ui.Text(c, detail).FontSize(typeCaption).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
		})

		ui.Icon(c, iconMore).FontSize(13).TextColor(t.TextMuted).Shrink(0)
	})

	ui.Popover(c, row, &a.chipOpen, func() {
		a.connectionMenu(c)
	})
}

// connectionMenu fills the connection row's popover menu.
func (a *App) connectionMenu(c *ui.Context) {
	ui.Column(c).Width(264).Gap(1).Children(func() {
		if a.session.Connected {
			if menuItem(c, "Disconnect", "close") {
				a.chipOpen = false
				a.disconnect()
			}
			divider(c)
		}
		if len(a.connections) > 0 {
			menuHeading(c, "Connections")
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
		if menuItem(c, "Add Connection…", "plus") {
			a.chipOpen = false
			a.openConnection(nil)
		}
		if menuItem(c, "Manage Connections…", "settings") {
			a.chipOpen = false
			a.setSection(sectionSettings)
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
	ch := chromeOf(t)

	row := ui.Row(c).Key(label).Padding(5, 8).Gap(8).AlignItems(ui.Center).Radius(5).
		Focusable().Cursor(ui.CursorPointer).OnClick(func() {}).
		TextColor(t.Text)

	hovered := row.Hovered()
	row = row.Background(ui.Transparent).Border(0, ui.Transparent).
		Draw(func(p *ui.Painter, r ui.Rect) {
			if hovered {
				p.Fill(r, ch.hover, 5)
			}
		})

	clicked := row.Clicked()
	row.Children(func() {
		ui.Box(c).Size(14, 14).Shrink(0).Children(func() {
			if checked {
				ui.Icon(c, iconCheck).FontSize(12).TextColor(t.Accent)
			}
		})
		if len(iconName) > 0 && iconName[0] != "" {
			ui.Icon(c, iconFor(iconName[0])).FontSize(14).TextColor(t.TextMuted).Shrink(0)
		}
		if detail == "" {
			ui.Text(c, label).FontSize(typeBody).Grow(1).SingleLine()
		} else {
			ui.Column(c).Grow(1).Gap(1).Children(func() {
				ui.Text(c, label).FontSize(typeBody).SingleLine().Ellipsis("…")
				ui.Text(c, detail).FontSize(typeCaption).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
			})
		}
	})
	return clicked
}

// menuHeading labels a group inside a menu.
func menuHeading(c *ui.Context, title string) {
	t := c.Theme()
	ui.Text(c, strings.ToUpper(title)).
		FontSize(typeCaption).
		FontWeight(weightSemi).
		TextColor(t.TextMuted).
		Padding(6, 8, 2, 8)
}

func divider(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(3, 6).Children(func() {
		rule(c, 0, t.Border)
	})
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
