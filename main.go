// Bucketeer is a native MyGo client for S3-compatible storage.
package main

import (
	"github.com/egoist/mygo"
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

	service, err := newService()
	if err != nil {
		log.Fatal(err)
	}
	app := newApp(service)
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
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
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
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { app.close() })
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
