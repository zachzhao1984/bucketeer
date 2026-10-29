//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
void bktMenuBarStart(void);
void bktMenuBarStop(void);
void bktMenuBarTarget(const char *title);
*/
import "C"

import (
	"encoding/json"
	"unsafe"

	"github.com/egoist/mygo"
)

var menuBarApp *App // accessed only on the AppKit main thread

func startMenuBar(a *App) func() {
	menuBarApp = a
	C.bktMenuBarStart()
	refreshMenuBarTarget(a)
	// Closing the browser leaves uploads and the menu bar running.
	off := a.win.OnClose(func(e *mygo.CloseEvent) { e.PreventDefault(); a.win.Hide() })
	return func() { off(); C.bktMenuBarStop(); menuBarApp = nil }
}

func refreshMenuBarTarget(a *App) {
	target := a.svc.GetSettings().MenuBarUpload
	title := "Default bucket: not set"
	if target.Bucket != "" {
		title = "Default bucket: " + target.Bucket
	}
	str := C.CString(title)
	defer C.free(unsafe.Pointer(str))
	C.bktMenuBarTarget(str)
}

//export bktMenuBarEvent
func bktMenuBarEvent(action C.int, payload *C.char) {
	a := menuBarApp
	if a == nil {
		return
	}
	var paths []string
	if payload != nil {
		if err := json.Unmarshal([]byte(C.GoString(payload)), &paths); err != nil {
			return
		}
	}
	a.menuBarAction(int(action), paths)
	if action != 2 {
		refreshMenuBarTarget(a)
		a.win.Invalidate()
	}
}
