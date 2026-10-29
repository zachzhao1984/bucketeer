//go:build darwin

package main

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// Exercise the same AppKit bridge used by mygo dev with CGO_ENABLED=0.
func TestMenuBarNativePasteboardDrops(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	_, err := purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_GLOBAL|purego.RTLD_NOW)
	if err != nil {
		t.Fatal(err)
	}
	pool := mbSend(mbClass("NSAutoreleasePool"), "new")
	defer mbSend(pool, "drain")
	pb := mbSend(mbClass("NSPasteboard"), "pasteboardWithUniqueName")
	defer mbSend(pb, "releaseGlobally")
	m := &nativeMenuBar{}
	defer func() {
		for _, dir := range m.temporaryDirectories {
			_ = os.RemoveAll(dir)
		}
	}()

	path := filepath.Join(t.TempDir(), "图片 with spaces.png")
	if err := os.WriteFile(path, []byte("file contents"), 0600); err != nil {
		t.Fatal(err)
	}
	url := mbSend(mbClass("NSURL"), "fileURLWithPath:", mbString(path))
	mbSend(pb, "writeObjects:", mbArray(url))
	paths, err := m.droppedPaths(pb)
	if err != nil || len(paths) != 1 || paths[0] != path {
		t.Fatalf("file drop: %v, %v", paths, err)
	}

	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	data := imageBytes.Bytes()
	nsdata := mbSend(mbClass("NSData"), "dataWithBytes:length:", unsafe.Pointer(&data[0]), len(data))
	bitmap := mbSend(mbClass("NSBitmapImageRep"), "imageRepWithData:", nsdata)
	tiff := mbSend(bitmap, "representationUsingType:properties:", uintptr(0), mbSend(mbClass("NSDictionary"), "dictionary"))
	for _, sample := range []struct {
		typ  string
		data objc.ID
	}{{"public.png", nsdata}, {"public.tiff", tiff}} {
		mbSend(pb, "clearContents")
		mbSend(pb, "setData:forType:", sample.data, mbString(sample.typ))
		paths, err := m.droppedPaths(pb)
		if err != nil || len(paths) != 1 {
			t.Fatalf("%s drop: %v, %v", sample.typ, paths, err)
		}
		saved, err := os.ReadFile(paths[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := png.Decode(bytes.NewReader(saved)); err != nil {
			t.Fatalf("%s conversion: %v", sample.typ, err)
		}
		info, err := os.Stat(paths[0])
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("image permissions = %o", info.Mode().Perm())
		}
	}
	mbSend(pb, "clearContents")
	if paths, err := m.droppedPaths(pb); err != nil || len(paths) != 0 {
		t.Fatalf("unsupported drop: %v, %v", paths, err)
	}
	if _, err := registerMenuBarDropView(); err != nil {
		t.Fatalf("register native callbacks: %v", err)
	}
	runtime.KeepAlive(data)
}
