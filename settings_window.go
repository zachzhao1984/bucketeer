package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// A settings controller owns its widget state, dialogs, services and window.
// The root controller owns shared preferences, connections and their lifetime.
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
	if a.win != nil {
		a.refreshConnections()
	}
	if a.win != nil && child.win == nil {
		child.win = mygo.NewWindow(mygo.WindowOptions{
			Title: "Bucketeer Settings", Width: 900, Height: 680, MinWidth: 820, MinHeight: 560,
			StateKey: "settings", BackgroundColor: "light-dark(#FBFAF6, #151312)",
			Content: ui.View(child.settingsView),
		})
		child.win.OnClosed(func() {
			child.cancel()
			if a.settingsApp == child {
				a.settingsApp = nil
			}
		})
	}
	if page == pageCLI {
		child.refreshCLIState()
	}
	if child.win != nil {
		child.win.Show()
		child.win.Focus()
		child.win.Invalidate()
	}
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
		if child.win != nil {
			child.win.Invalidate()
		}
	}
}

func (a *App) invalidatePreferenceWindows() {
	if a.win != nil {
		a.win.Invalidate()
	}
	if child := a.settingsApp; child != nil && child.win != nil {
		child.win.Invalidate()
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
