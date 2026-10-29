package main

import "github.com/egoist/mygo/ui"

// A settings controller owns its widget state and nested dialogs.
// Both controllers render in the main window; preferences remain shared.
func (a *App) newSettingsController() *App {
	child := newApp(a.svc)
	child.settingsOwner = a
	child.prefs = a.prefs
	child.connections = a.connections
	child.session = a.session
	return child
}

func (a *App) openSettingsPage(page settingsPage) {
	if a.settingsOwner != nil {
		a.settingsOwner.openSettingsPage(page)
		return
	}
	child := a.settingsApp
	if child == nil {
		child = a.newSettingsController()
		a.settingsApp = child
	}
	child.settingsPage = page
	child.win = a.win
	a.settingsOpen = true
	if a.win != nil {
		a.refreshConnections()
		a.win.Show()
		a.win.Focus()
		a.win.Invalidate()
	}
	if page == pageCLI {
		child.refreshCLIState()
	}
}

func (a *App) settingsModal(c *ui.Context) {
	if !a.settingsOpen || a.settingsApp == nil {
		return
	}
	t := c.Theme()
	ch := chromeOf(t)
	width, height := c.Size()
	open := a.settingsOpen
	ui.DialogBase(c.Key("settings-modal"), &open, func(back, panel ui.Element) {
		back.Background(ch.scrim)
		panel.Width(min(900, width-48)).Height(min(680, height-48)).Radius(14).Clip().
			Background(ch.canvas).Border(1, ch.hairline)
		panel.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Shadow(r, 14, 0, 20, 60, 0, ch.floatShadow)
		})
		ui.Row(c).FillWidth().Height(44).Padding(0, 16).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Settings").FontSize(typeBody).FontWeight(weightMedium).Grow(1)
			close := ctlIconButtonBare(c, "Close Settings")
			close.Children(func() { ui.Icon(c, iconClose).FontSize(14).TextColor(t.TextMuted) })
			close.OnClick(func() { a.settingsOpen = false })
		})
		ui.Box(c).FillWidth().Height(1).Background(ch.hairline)
		a.settingsApp.settingsView(c)
	})
	a.settingsOpen = open
}

func (a *App) settingsView(c *ui.Context) {
	a.services = c.Services()
	dark := a.dark(c)
	a.appearanceDark = dark
	c.SetTheme(a.theme(dark))
	if a.notice != "" {
		c.Toast(a.notice)
		a.notice = ""
	}
	ui.Column(c).Fill().Background(chromeOf(c.Theme()).canvas).Children(func() {
		if a.err != "" {
			a.errorBanner(c)
		}
		a.settings(c)
	})
	a.dialogs(c)
}

func (a *App) syncSettingsConnection() {
	if child := a.settingsApp; child != nil {
		child.connections = a.connections
		child.session = a.session
	}
}

func (a *App) invalidatePreferences() {
	if a.win != nil {
		a.win.Invalidate()
	}
}

func (a *App) syncMenuBar() {
	if a.win == nil {
		return
	}
	if a.prefs.ShowMenuBarIcon {
		if a.menuBarStop == nil {
			a.menuBarStop = startMenuBar(a)
		}
	} else if a.menuBarStop != nil {
		a.menuBarStop()
		a.menuBarStop = nil
	}
}
