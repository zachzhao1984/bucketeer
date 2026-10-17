// Bucketeer is a native MyGo client for S3-compatible storage.
package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"log"
)

func main() {
	service, err := newService()
	if err != nil {
		log.Fatal(err)
	}
	app := newApp(service)
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title: appName, Width: 1180, Height: 760, MinWidth: 900, MinHeight: 560,
			TitleBarStyle: mygo.TitleBarHiddenInset, TitleBarHeight: 52,
			StateKey: "main", Content: ui.View(app.view),
		})
		app.attach(win)
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Label: "File", Submenu: []*mygo.MenuItem{
				{Label: "Upload Files…", Accelerator: "CmdOrCtrl+O", Click: func(*mygo.MenuItem, *mygo.Window) { win.Update(app.upload) }},
				{Label: "Refresh", Accelerator: "CmdOrCtrl+R", Click: func(*mygo.MenuItem, *mygo.Window) { win.Update(app.refresh) }},
				{Label: "Transfers", Accelerator: "CmdOrCtrl+T", Click: func(*mygo.MenuItem, *mygo.Window) { win.Update(func() { app.transfersOpen = !app.transfersOpen }) }},
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
