package main

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"bucketeer/internal/store"
	"github.com/egoist/mygo/ui"
)

func TestSettingsModalPreservesBrowserAndReusesController(t *testing.T) {
	a := nativeTestApp(t)
	a.section = sectionFiles
	a.bucket, a.prefix = "photos", "2026/"
	a.sel.Add("2026/image.png")
	a.openSettings()
	child := a.settingsApp
	if child == nil || child == a || child.settingsOwner != a {
		t.Fatal("settings must own a separate controller")
	}
	a.openSettingsPage(pageConnections)
	if a.settingsApp != child || child.settingsPage != pageConnections {
		t.Fatal("settings entry did not reuse the existing controller")
	}
	a.openSettings()
	if child.settingsPage != pagePreferences {
		t.Fatal("Settings command did not select Preferences")
	}
	if a.section != sectionFiles || a.bucket != "photos" || a.prefix != "2026/" || !a.sel.Has("2026/image.png") {
		t.Fatal("opening Settings changed the browser")
	}
	child.openConnection(nil)
	if a.dlg != dialogNone || child.dlg != dialogConnection {
		t.Fatal("settings dialog leaked into main window")
	}
}

func TestSettingsPreferencesSynchronizeAndPersist(t *testing.T) {
	a := nativeTestApp(t)
	a.openSettings()
	child := a.settingsApp
	stored := a.svc.GetSettings()
	stored.PicGo.Prefix = "notebooks"
	if err := a.svc.SetSettings(stored); err != nil {
		t.Fatal(err)
	}
	child.prefs.Appearance = "dark"
	child.prefs.ShowMenuBarIcon = false
	child.persistSettings()
	if a.prefs.Appearance != "dark" || a.prefs.ShowMenuBarIcon {
		t.Fatal("main window preferences not synchronized")
	}
	if a.prefs.PicGo.Prefix != "notebooks" {
		t.Fatal("general preferences replaced the image hosting configuration")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		data, err := os.ReadFile(settingsPath(a.svc.dataDir))
		var saved Settings
		if err == nil && json.Unmarshal(data, &saved) == nil && saved.Appearance == "dark" && !saved.ShowMenuBarIcon {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("settings were not persisted: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Changes in the main window must also reach the existing settings window.
	a.setViewMode(viewList)
	if child.prefs.ViewMode != "list" {
		t.Fatal("settings window has stale browsing preferences")
	}
}

func TestMenuBarVisibilityDefaultsForExistingSettings(t *testing.T) {
	s := newTestService(t)
	if !s.GetSettings().ShowMenuBarIcon {
		t.Fatal("menu bar should be visible by default")
	}
	if err := os.WriteFile(settingsPath(s.dataDir), []byte(`{"appearance":"dark"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.loadSettings(s.dataDir); err != nil {
		t.Fatal(err)
	}
	if !s.GetSettings().ShowMenuBarIcon {
		t.Fatal("existing settings unexpectedly hide the icon")
	}
	settings := s.GetSettings()
	settings.ShowMenuBarIcon = false
	settings.MenuBarUpload = MenuBarTarget{ProfileID: "saved-profile", Bucket: "images"}
	if err := s.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := s.loadSettings(s.dataDir); err != nil {
		t.Fatal(err)
	}
	if got := s.GetSettings(); got.ShowMenuBarIcon || got.MenuBarUpload != settings.MenuBarUpload {
		t.Fatal("visibility or default upload target did not survive reload")
	}
}

func TestSettingsMenuBarSwitch(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS menu bar preference")
	}
	a := nativeTestApp(t)
	a.openSettings()
	child := a.settingsApp
	tester := ui.NewTester(a.view, 1180, 760)
	tester.Frame()
	if err := tester.Click("Show Bucketeer in menu bar"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Frame()
	if a.prefs.ShowMenuBarIcon || child.prefs.ShowMenuBarIcon {
		t.Fatal("switch did not hide the icon in shared preferences")
	}
	if err := tester.Click("Show Bucketeer in menu bar"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Frame()
	if !a.prefs.ShowMenuBarIcon {
		t.Fatal("switch did not restore the icon")
	}
}

func TestSettingsConnectionSnapshot(t *testing.T) {
	a := nativeTestApp(t)
	a.openSettings()
	p := store.Profile{ID: "new", Name: "New account"}
	a.connections = []store.Profile{p}
	a.session = SessionInfo{Connected: true, Profile: &p}
	a.syncSettingsConnection()
	if len(a.settingsApp.connections) != 1 || a.settingsApp.session.Profile.ID != p.ID {
		t.Fatal("settings connection list did not update")
	}
}

func TestSettingsModalDismissalAndInputScope(t *testing.T) {
	a := nativeTestApp(t)
	a.section = sectionFiles
	profile := store.Profile{ID: "test", Name: "Test"}
	a.session = SessionInfo{Connected: true, Profile: &profile}
	a.bucket, a.prefix = "images", "photos/"
	a.sel.Add("photos/image.png")
	tester := ui.NewTester(a.view, 1180, 760)
	a.openSettings()
	tester.Frame()
	if !tester.HasText("Show menu bar icon") && runtime.GOOS == "darwin" {
		t.Fatal("Preferences not rendered inside the main window modal")
	}
	if err := tester.Click("Dashboard"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if a.section != sectionFiles {
		t.Fatal("modal allowed background navigation")
	}
	// A nested connection dialog gets Escape before its parent modal.
	a.openSettingsPage(pageConnections)
	a.settingsApp.openConnection(nil)
	tester.Frame()
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if !a.settingsOpen || a.settingsApp.dlg != dialogNone {
		t.Fatal("Escape did not close only the nested dialog")
	}
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if a.settingsOpen || !a.sel.Has("photos/image.png") || a.prefix != "photos/" {
		t.Fatal("dismissing Settings changed browser state")
	}
	a.openSettings()
	tester.SetSize(900, 560)
	tester.Frame()
	if r, ok := tester.Find("Close Settings"); !ok || r.X+r.W > 900 || r.Y+r.H > 560 {
		t.Fatal("modal close control is outside the smallest window")
	}
	if err := tester.Click("Close Settings"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if a.settingsOpen {
		t.Fatal("Close Settings did not dismiss the modal")
	}
	// Backdrop dismissal uses the same path and keeps the browser intact.
	a.openSettings()
	tester.Frame()
	tester.ClickAt(8, 8)
	tester.Frame()
	if a.settingsOpen || !a.sel.Has("photos/image.png") {
		t.Fatal("backdrop dismissal changed browser selection")
	}
}

func TestSettingsModalShots(t *testing.T) {
	dir := os.Getenv("SETTINGS_SHOTS")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	a := nativeTestApp(t)
	a.openSettings()
	child := a.settingsApp
	tester := ui.NewTester(a.view, 1180, 760)
	for _, appearance := range []string{"light", "dark"} {
		a.prefs.Appearance = appearance
		child.prefs.Appearance = appearance
		tester.Frame()
		file, err := os.Create(filepath.Join(dir, appearance+".png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(file, tester.Image())
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}
