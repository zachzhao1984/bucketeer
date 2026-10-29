// Bucketeer is a native MyGo client for S3-compatible storage.
package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
	"log"
	"os"
)

func main() {
	// The app is also its own command line: with arguments it acts on the
	// saved connections and exits, without arguments it opens the window.
	if len(os.Args) > 1 {
		os.Exit(runCLI(os.Args[1:]))
	}

	// The updater's window is native UI, like the app's own: it checks in
	// the background, shows release notes and installs with a progress bar.
	mygo.Use(native.Plugin)

	service, err := newService()
	if err != nil {
		log.Fatal(err)
	}
	app := newApp(service)
	var stopMenuBar func()
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title: appName, Width: 1180, Height: 760, MinWidth: 900, MinHeight: 560,
			TitleBarStyle: mygo.TitleBarHiddenInset, TitleBarHeight: 52,
			BackgroundColor: "light-dark(#e7e7eb, #121214)",
			// The sidebar and the bar along the top share the window's
			// own material; the content plane is painted over it.
			Vibrancy: mygo.VibrancySidebar,
			StateKey: "main", Content: ui.View(app.view),
		})
		app.attach(win)
		stopMenuBar = startMenuBar(app)
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleAbout},
				updater.MenuItem(),
				mygo.Separator(),
				{Label: "Settings…", Accelerator: "CmdOrCtrl+,", Click: func(*mygo.MenuItem, *mygo.Window) {
					win.Update(app.openSettings)
				}},
				mygo.Separator(),
				{Role: mygo.RoleServices},
				mygo.Separator(),
				{Role: mygo.RoleHide},
				{Role: mygo.RoleHideOthers},
				{Role: mygo.RoleUnhide},
				mygo.Separator(),
				{Role: mygo.RoleQuit},
			}},
			{Label: "File", Submenu: []*mygo.MenuItem{
				{Label: "Upload Files…", Accelerator: "CmdOrCtrl+O", Click: func(*mygo.MenuItem, *mygo.Window) { win.Update(app.upload) }},
				{Label: "Refresh", Accelerator: "CmdOrCtrl+R", Click: func(*mygo.MenuItem, *mygo.Window) { win.Update(app.refresh) }},
				{Label: "Transfers", Accelerator: "CmdOrCtrl+T", Click: func(*mygo.MenuItem, *mygo.Window) { win.Update(func() { app.transfersOpen = !app.transfersOpen }) }},
				mygo.Separator(),
				{Label: "Install Command Line…", Click: func(*mygo.MenuItem, *mygo.Window) {
					win.Update(func() {
						app.setSection(sectionSettings)
						app.settingsPage = pageCLI
						app.refreshCLIState()
					})
				}},
				mygo.Separator(), {Role: mygo.RoleClose},
			}},
			{Role: mygo.RoleEditMenu}, {Role: mygo.RoleWindowMenu},
		}))
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		app.close()
		if stopMenuBar != nil {
			stopMenuBar()
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
