//go:build darwin

package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"structs"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

//go:embed assets/menubar-template.png
var menuBarIconPNG []byte

// MyGo deliberately builds with CGO_ENABLED=0 by default, including dev.
// Use the same Objective-C runtime bridge as MyGo so this code is included
// in both development and packaged apps without special environment flags.
type menuBarRect struct {
	_                   structs.HostLayout
	X, Y, Width, Height float64
}
type menuBarPoint struct {
	_    structs.HostLayout
	X, Y float64
}

type nativeMenuBar struct {
	app                      *App
	item, view, menu, target objc.ID
}

// AppKit calls all these methods on the main thread.
var activeMenuBar *nativeMenuBar

func mbSend(receiver objc.ID, selector string, args ...any) objc.ID {
	return receiver.Send(objc.RegisterName(selector), args...)
}
func mbClass(name string) objc.ID  { return objc.ID(objc.GetClass(name)) }
func mbString(text string) objc.ID { return mbSend(mbClass("NSString"), "stringWithUTF8String:", text) }
func mbGoString(text objc.ID) string {
	if text == 0 {
		return ""
	}
	return objc.Send[string](text, objc.RegisterName("UTF8String"))
}
func mbArray(items ...objc.ID) objc.ID {
	array := mbSend(mbClass("NSMutableArray"), "array")
	for _, item := range items {
		mbSend(array, "addObject:", item)
	}
	return array
}
func mbTypes() objc.ID {
	return mbArray(mbString("public.file-url"), mbString("public.png"), mbString("public.tiff"))
}
func mbMethod(name string, fn any) objc.MethodDef {
	return objc.MethodDef{Cmd: objc.RegisterName(name), Fn: fn}
}

func registerMenuBarDropView() (objc.Class, error) {
	if class := objc.GetClass("BKTDropView"); class != 0 {
		return class, nil
	}
	showMenu := func(self objc.ID, event objc.ID) {
		m := activeMenuBar
		if m == nil {
			return
		}
		m.refreshTarget()
		mbSend(mbClass("NSMenu"), "popUpContextMenu:withEvent:forView:", m.menu, event, self)
	}
	dragOperation := func(_ objc.ID, _ objc.SEL, sender objc.ID) uintptr {
		pb := mbSend(sender, "draggingPasteboard")
		if mbSend(pb, "availableTypeFromArray:", mbTypes()) != 0 {
			return 1
		} // NSDragOperationCopy
		return 0
	}
	return objc.RegisterClass("BKTDropView", objc.GetClass("NSView"), nil, nil, []objc.MethodDef{
		mbMethod("mouseDown:", func(self objc.ID, _ objc.SEL, event objc.ID) { showMenu(self, event) }),
		mbMethod("rightMouseDown:", func(self objc.ID, _ objc.SEL, event objc.ID) { showMenu(self, event) }),
		mbMethod("accessibilityPerformPress", func(_ objc.ID, _ objc.SEL) bool {
			if m := activeMenuBar; m != nil {
				m.refreshTarget()
				mbSend(m.menu, "popUpMenuPositioningItem:atLocation:inView:", objc.ID(0), menuBarPoint{}, m.view)
				return true
			}
			return false
		}),
		mbMethod("menuAction:", func(_ objc.ID, _ objc.SEL, item objc.ID) {
			if m := activeMenuBar; m != nil {
				action := int(mbSend(item, "tag"))
				m.app.menuBarAction(action, nil)
				m.refreshTarget()
				m.app.win.Invalidate()
			}
		}),
		mbMethod("draggingEntered:", dragOperation),
		mbMethod("draggingUpdated:", dragOperation),
		mbMethod("prepareForDragOperation:", func(self objc.ID, sel objc.SEL, sender objc.ID) bool { return dragOperation(self, sel, sender) != 0 }),
		mbMethod("performDragOperation:", func(_ objc.ID, _ objc.SEL, sender objc.ID) bool {
			m := activeMenuBar
			if m == nil {
				return false
			}
			paths, err := m.droppedPaths(mbSend(sender, "draggingPasteboard"))
			if err != nil {
				m.app.fail(err)
				m.app.win.Show()
				m.app.win.Invalidate()
				return false
			}
			if len(paths) == 0 {
				return false
			}
			m.app.menuBarAction(3, paths)
			m.app.win.Invalidate()
			return true
		}),
	})
}

func startMenuBar(a *App) func() {
	class, err := registerMenuBarDropView()
	if err != nil {
		a.fail(fmt.Errorf("create menu bar drop target: %w", err))
		return func() {}
	}
	m := &nativeMenuBar{app: a}
	bar := mbSend(mbClass("NSStatusBar"), "systemStatusBar")
	m.item = mbSend(bar, "statusItemWithLength:", float64(28))
	mbSend(m.item, "retain")
	mbSend(m.item, "setAutosaveName:", mbString("bucketeer.upload"))
	button := mbSend(m.item, "button")
	// Let NSStatusBarButton paint and tint the template, rather than drawing a
	// template image manually in a custom view (which misses menu-bar tinting).
	data := mbSend(mbClass("NSData"), "dataWithBytes:length:", unsafe.Pointer(&menuBarIconPNG[0]), len(menuBarIconPNG))
	image := mbSend(mbSend(mbClass("NSImage"), "alloc"), "initWithData:", data)
	mbSend(image, "setSize:", menuBarPoint{X: 18, Y: 18})
	mbSend(image, "setTemplate:", true)
	mbSend(button, "setImage:", image)
	mbSend(image, "release")
	rect := objc.Send[menuBarRect](button, objc.RegisterName("bounds"))
	m.view = mbSend(mbSend(objc.ID(class), "alloc"), "initWithFrame:", rect)
	mbSend(m.view, "setAutoresizingMask:", uintptr(2|16)) // flexible width and height
	mbSend(m.view, "setToolTip:", mbString("Drop files to upload to your default bucket"))
	mbSend(m.view, "setAccessibilityElement:", true)
	mbSend(m.view, "setAccessibilityRole:", mbString("AXButton"))
	mbSend(m.view, "setAccessibilityLabel:", mbString("Bucketeer uploads"))
	mbSend(m.view, "registerForDraggedTypes:", mbTypes())
	m.menu = mbSend(mbSend(mbClass("NSMenu"), "alloc"), "initWithTitle:", mbString("Bucketeer"))
	m.target = mbSend(mbSend(mbClass("NSMenuItem"), "alloc"), "initWithTitle:action:keyEquivalent:", mbString("Default bucket: not set"), objc.SEL(0), mbString(""))
	mbSend(m.menu, "addItem:", m.target)
	mbSend(m.target, "release") // the menu owns the item
	for action, title := range []string{"Open Bucketeer", "Use Current Bucket as Default", "Quit Bucketeer"} {
		item := mbSend(mbSend(mbClass("NSMenuItem"), "alloc"), "initWithTitle:action:keyEquivalent:", mbString(title), objc.RegisterName("menuAction:"), mbString(""))
		mbSend(item, "setTarget:", m.view)
		mbSend(item, "setTag:", action)
		mbSend(m.menu, "addItem:", item)
		mbSend(item, "release")
	}
	activeMenuBar = m
	mbSend(button, "addSubview:", m.view)
	m.refreshTarget()
	return func() {
		activeMenuBar = nil
		mbSend(bar, "removeStatusItem:", m.item)
		mbSend(m.item, "release")
		mbSend(m.view, "release")
		mbSend(m.menu, "release")
	}
}

func (m *nativeMenuBar) refreshTarget() {
	target := m.app.prefs.MenuBarUpload
	title := "Default bucket: not set"
	if target.Bucket != "" {
		title = "Default bucket: " + target.Bucket
	}
	mbSend(m.target, "setTitle:", mbString(title))
}

func (m *nativeMenuBar) droppedPaths(pb objc.ID) ([]string, error) {
	urls := mbSend(pb, "readObjectsForClasses:options:", mbArray(mbClass("NSURL")), objc.ID(0))
	var paths []string
	for i, n := uintptr(0), uintptr(mbSend(urls, "count")); i < n; i++ {
		url := mbSend(urls, "objectAtIndex:", i)
		if objc.Send[bool](url, objc.RegisterName("isFileURL")) {
			paths = append(paths, mbGoString(mbSend(url, "path")))
		}
	}
	if len(paths) > 0 {
		return paths, nil
	}
	png := mbSend(pb, "dataForType:", mbString("public.png"))
	if png == 0 {
		tiff := mbSend(pb, "dataForType:", mbString("public.tiff"))
		if tiff == 0 {
			return nil, nil
		}
		bitmap := mbSend(mbClass("NSBitmapImageRep"), "imageRepWithData:", tiff)
		png = mbSend(bitmap, "representationUsingType:properties:", uintptr(4), mbSend(mbClass("NSDictionary"), "dictionary")) // NSBitmapImageFileTypePNG
	}
	if png == 0 {
		return nil, fmt.Errorf("could not convert the dropped image to PNG")
	}
	length := uintptr(mbSend(png, "length"))
	bytes := objc.Send[unsafe.Pointer](png, objc.RegisterName("bytes"))
	dir, err := os.MkdirTemp("", "bucketeer-drop-")
	if err != nil {
		return nil, err
	}
	// The unique directory suffix also gives the uploaded image a unique name.
	path := filepath.Join(dir, filepath.Base(dir)+".png")
	if err := os.WriteFile(path, unsafe.Slice((*byte)(bytes), length), 0600); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	m.app.menuBarTemps = append(m.app.menuBarTemps, dir)
	return []string{path}, nil
}
