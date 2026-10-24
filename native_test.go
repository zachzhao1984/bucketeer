package main

import (
	"bucketeer/internal/cli"
	"bucketeer/internal/store"
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func nativeTestApp(t *testing.T) *App {
	t.Helper()
	a := newApp(newTestService(t))
	t.Cleanup(a.close)
	return a
}

func TestNativeNavigationAndConnectionForm(t *testing.T) {
	a := nativeTestApp(t)
	tester := ui.NewTester(a.view, 1180, 760)
	tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
	if !tester.HasText("Add a connection") {
		t.Fatal(tester.Texts())
	}
	if err := tester.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !tester.HasText("Preferences") {
		t.Fatalf("Settings did not open: %v", tester.Texts())
	}
	if err := tester.Click("Connections"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if err := tester.Click("Add connection"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !tester.HasText("Add a connection") {
		t.Fatal("connection form missing")
	}
	if err := tester.Click("Connection name"); err != nil {
		t.Fatal(err)
	}
	tester.Type("Test account")
	tester.Frame()
	if a.conn.name != "Test account" {
		t.Fatalf("input did not persist: %q", a.conn.name)
	}
	if err := tester.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if a.dlg != dialogNone {
		t.Fatal("cancel did not close dialog")
	}
}

func TestNativeProviderSelectionAndEndpoint(t *testing.T) {
	a := nativeTestApp(t)
	a.openConnection(nil)
	tester := ui.NewTester(a.view, 1180, 760)
	tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
	tester.Frame()
	if err := tester.Click("Choose provider"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	for _, preset := range store.Presets {
		if !tester.HasText(preset.Label) {
			t.Fatalf("missing provider %q in menu", preset.Label)
		}
	}
	if err := tester.Click("Tencent Cloud COS"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if a.conn.provider != string(store.ProviderCOS) || a.conn.endpoint != "" {
		t.Fatalf("COS selection: provider=%q endpoint=%q", a.conn.provider, a.conn.endpoint)
	}
	a.conn.name = "Beijing"
	a.conn.region = "ap-beijing"
	a.conn.accessKey = "AKIDEXAMPLE"
	a.conn.secretKey = "secret"
	tester.Frame()
	if !tester.HasText(a.preset().Note) {
		t.Fatal("COS instructions missing")
	}
	in, ok := a.connInput()
	if !ok || in.Endpoint != "https://cos.ap-beijing.myqcloud.com" || a.conn.endpoint != "" {
		t.Fatalf("derived endpoint=%q, ok=%v", in.Endpoint, ok)
	}
	a.conn.endpoint = "https://custom.example.com"
	in, ok = a.connInput()
	if !ok || in.Endpoint != a.conn.endpoint {
		t.Fatalf("manual endpoint=%q, ok=%v", in.Endpoint, ok)
	}

	// Saved Spaces profiles can still be opened and edited after the menu
	// changes. They retain their original provider and endpoint.
	p := store.Profile{ID: "old", Name: "Spaces", Provider: store.ProviderSpaces,
		Endpoint: "https://sfo3.digitaloceanspaces.com", Region: "sfo3"}
	a.openConnection(&p)
	tester.Frame()
	if a.providerLabel() != "DigitalOcean Spaces" || a.conn.endpoint != p.Endpoint {
		t.Fatalf("existing Spaces profile: provider=%q endpoint=%q", a.conn.provider, a.conn.endpoint)
	}
	tester.SetSize(900, 560)
	tester.Frame()
	if r, found := tester.Find("Save & connect"); !found || r.Y+r.H > 560 {
		t.Fatalf("connection actions are offscreen at the smallest window: found=%v bounds=%+v", found, r)
	}
}

func TestNativeViews(t *testing.T) {
	a := nativeTestApp(t)
	p := store.Profile{ID: "demo", Name: "Acme Media", Endpoint: "https://s3.example.com"}
	a.session = SessionInfo{Connected: true, Profile: &p, Host: "s3.example.com"}
	a.bucket = "acme-media"
	a.section = sectionFiles
	a.listing = store.Listing{Items: []store.Entry{{Key: "images/", Name: "images", Folder: true}, {Key: "notes.txt", Name: "notes.txt", Size: 412, LastModified: time.Now()}, {Key: "photo.png", Name: "photo.png", Size: 2048}}}
	tester := ui.NewTester(a.view, 1180, 760)
	tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
	if !tester.HasText("notes.txt") {
		t.Fatal(tester.Texts())
	}
	for _, mode := range []ViewMode{viewGrid, viewList} {
		a.mode = mode
		tester.Frame()
		tester.SetDark(true)
		tester.Frame()
		tester.SetDark(false)
	}
	a.stat = &store.Stat{Bucket: a.bucket, Objects: 3, Bytes: 2460, TopFolders: []store.FolderTotal{{Prefix: "images/", Name: "images", Bytes: 2048}}, Largest: a.listing.Items[1:]}
	a.section = sectionDashboard
	tester.Frame()
	a.transfersOpen = true
	tester.Frame()
	a.section = sectionSettings
	tester.Frame()
	a.openNewFolder()
	tester.Frame()
	a.dlg = dialogNone
	tester.Frame()
	a.openConnection(&p)
	tester.Frame()
	a.dlg = dialogNone
	tester.Frame()
	tester.SetSize(900, 560)
	tester.Frame()
	tester.SetTitleBar(ui.TitleBar{})
	tester.Frame()
}

func TestGenerateShots(t *testing.T) {
	dir := os.Getenv("PAIL_SHOTS")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	save := func(tester *ui.Tester, name string) {
		t.Helper()
		tester.Frame()
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tester.Image()); err != nil {
			t.Fatal(err)
		}
	}

	p := store.Profile{ID: "demo", Name: "Acme Media", Endpoint: "https://s3.example.com"}

	// 1. Welcome (disconnected)
	{
		a := nativeTestApp(t)
		tester := ui.NewTester(a.view, 1180, 760)
		tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
		save(tester, "welcome.png")
	}

	// 2. Files (grid, light and dark)
	{
		a := nativeTestApp(t)
		a.session = SessionInfo{Connected: true, Profile: &p, Host: "s3.example.com"}
		a.bucket = "acme-media"
		a.section = sectionFiles
		a.mode = viewGrid
		a.listing = store.Listing{Items: []store.Entry{
			{Key: "images/", Name: "images", Folder: true},
			{Key: "documents/", Name: "documents", Folder: true},
			{Key: "notes.txt", Name: "notes.txt", Size: 412, LastModified: time.Now()},
			{Key: "photo.png", Name: "photo.png", Size: 2048},
			{Key: "archive.zip", Name: "archive.zip", Size: 1048576},
		}}
		tester := ui.NewTester(a.view, 1180, 760)
		tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
		save(tester, "files.png")

		tester.SetDark(true)
		save(tester, "files-dark.png")
		tester.SetDark(false)

		// 3. List mode
		a.mode = viewList
		save(tester, "list.png")

		// 4. Transfers open
		a.transfersOpen = true
		save(tester, "transfers.png")
		a.transfersOpen = false

		// 5. Modal dialog
		a.openNewFolder()
		save(tester, "prompt.png")
		a.dlg = dialogNone

		a.openConnection(&p)
		save(tester, "dialog.png")
		a.dlg = dialogNone

		// Detail
		a.detail = detailState{
			key: "photo.png",
			info: store.ObjectInfo{
				Key:          "photo.png",
				Name:         "photo.png",
				Size:         2048,
				ContentType:  "image/png",
				StorageClass: "STANDARD",
				LastModified: time.Now(),
			},
		}
		a.dlg = dialogDetail
		save(tester, "detail.png")
		a.dlg = dialogNone

		// Confirm
		a.ask("Delete “photo.png”?", "This cannot be undone.", "Delete", func() {})
		save(tester, "confirm.png")
		a.confirm.open = false

		// 8. The narrowest the window is allowed to get.
		a.mode = viewGrid
		tester.SetSize(900, 560)
		save(tester, "files-compact.png")
		a.mode = viewList
		save(tester, "list-compact.png")
		tester.SetSize(1180, 760)
	}

	// 6. Dashboard (light and dark)
	{
		a := nativeTestApp(t)
		a.session = SessionInfo{Connected: true, Profile: &p, Host: "s3.example.com"}
		a.bucket = "acme-media"
		a.section = sectionDashboard
		a.stat = &store.Stat{
			Bucket:  a.bucket,
			Objects: 124,
			Bytes:   1024 * 1024 * 420,
			Folders: 8,
			TopFolders: []store.FolderTotal{
				{Prefix: "images/", Name: "images", Bytes: 1024 * 1024 * 280},
				{Prefix: "videos/", Name: "videos", Bytes: 1024 * 1024 * 110},
				{Prefix: "backups/", Name: "backups", Bytes: 1024 * 1024 * 30},
			},
			ByKind: []store.KindTotal{
				{Kind: "image", Bytes: 1024 * 1024 * 280, Count: 86},
				{Kind: "video", Bytes: 1024 * 1024 * 110, Count: 12},
				{Kind: "document", Bytes: 1024 * 1024 * 30, Count: 26},
			},
			Recent:       17,
			LastModified: time.Now().Add(-3 * time.Hour),
			Largest: []store.Entry{
				{Key: "videos/launch.mp4", Name: "launch.mp4", Size: 1024 * 1024 * 85},
				{Key: "images/hero.raw", Name: "hero.raw", Size: 1024 * 1024 * 45},
			},
		}
		tester := ui.NewTester(a.view, 1180, 760)
		tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
		save(tester, "dashboard.png")

		tester.SetDark(true)
		save(tester, "dashboard-dark.png")
	}

	// 7. Settings
	{
		a := nativeTestApp(t)
		a.connections = []store.Profile{p}
		a.section = sectionSettings
		tester := ui.NewTester(a.view, 1180, 760)
		tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
		save(tester, "settings.png")

		a.settingsPage = pageConnections
		save(tester, "settings-connections.png")
		a.settingsPage = pageCLI
		a.refreshCLIState()
		save(tester, "settings-cli.png")
		a.settingsPage = pageAbout
		save(tester, "settings-about.png")
	}

	// 9. Recent files
	{
		a := nativeTestApp(t)
		a.session = SessionInfo{Connected: true, Profile: &p, Host: "s3.example.com"}
		a.bucket = "acme-media"
		a.section = sectionRecent
		a.recent = []recentEntry{
			{Bucket: "acme-media", Key: "images/hero.png", Name: "hero.png", Size: 2_048_000, At: time.Now().Add(-4 * time.Minute)},
			{Bucket: "acme-media", Key: "notes/todo.md", Name: "todo.md", Size: 1_204, At: time.Now().Add(-2 * time.Hour)},
			{Bucket: "acme-media", Key: "videos/launch.mp4", Name: "launch.mp4", Size: 91_400_000, At: time.Now().Add(-26 * time.Hour)},
		}
		tester := ui.NewTester(a.view, 1180, 760)
		tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
		save(tester, "recent.png")
	}
}

// TestFlows walks the interactions a person performs: the type filter,
// the sort field, the view switch, the sidebar's places, the transfers
// dock, and the pages of the settings sheet.
func TestFlows(t *testing.T) {
	a := nativeTestApp(t)
	p := store.Profile{ID: "demo", Name: "Acme Media", Endpoint: "https://s3.example.com"}
	a.session = SessionInfo{Connected: true, Profile: &p, Host: "s3.example.com"}
	a.bucket = "acme-media"
	a.section = sectionFiles
	a.listing = store.Listing{Items: []store.Entry{
		{Key: "images/", Name: "images", Folder: true},
		{Key: "a.png", Name: "a.png", Size: 2048, LastModified: time.Now()},
		{Key: "b.txt", Name: "b.txt", Size: 412, LastModified: time.Now()},
	}}
	tester := ui.NewTester(a.view, 1180, 760)
	tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
	tester.Frame()

	// The type filter is a menu of the kinds, and it narrows the listing.
	if err := tester.Click("Type"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if err := tester.Click("Images"); err != nil {
		t.Fatalf("type menu: %v", err)
	}
	tester.Frame()
	if a.filter != "image" {
		t.Fatalf("filter=%q", a.filter)
	}
	if got := len(a.visible()); got != 1 {
		t.Fatalf("the images filter left %d entries", got)
	}
	a.filter = "all"
	tester.Frame()

	// The sort menu orders the listing by date.
	if err := tester.Click("Sort"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if err := tester.Click("Date modified"); err != nil {
		t.Fatalf("sort menu: %v", err)
	}
	tester.Frame()
	if a.sort != sortModified {
		t.Fatalf("sort=%q", a.sort)
	}

	// The view switch changes how the listing is drawn.
	if err := tester.Click("List view"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if a.mode != viewList {
		t.Fatalf("mode=%q", a.mode)
	}

	// The sidebar's places.
	if err := tester.Click("Recent files"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if a.section != sectionRecent {
		t.Fatalf("section=%q", a.section)
	}
	if err := tester.Click("All files"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if a.section != sectionFiles {
		t.Fatalf("section=%q", a.section)
	}

	// Transfers is a panel over the corner, not a place in the sidebar.
	a.transfersOpen = true
	tester.Frame()
	if !tester.HasText("No transfers yet") {
		t.Fatalf("the transfers dock is missing: %v", tester.Texts())
	}
	a.transfersOpen = false
	tester.Frame()

	// Settings is one sheet with three pages.
	if err := tester.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	for _, page := range []string{"Connections", "About", "Preferences"} {
		if err := tester.Click(page); err != nil {
			t.Fatalf("page %s: %v", page, err)
		}
		tester.Frame()
	}
}

// TestSelectionActionsReplaceTheToolbar checks that choosing a row swaps
// the toolbar for the actions that apply to it, and that deselecting
// brings the usual actions back.
func TestSelectionActionsReplaceTheToolbar(t *testing.T) {
	a := nativeTestApp(t)
	p := store.Profile{ID: "demo", Name: "Acme Media", Endpoint: "https://s3.example.com"}
	a.session = SessionInfo{Connected: true, Profile: &p, Host: "s3.example.com"}
	a.bucket = "acme-media"
	a.section = sectionFiles
	a.listing = store.Listing{Items: []store.Entry{
		{Key: "a.png", Name: "a.png", Size: 2048, LastModified: time.Now()},
	}}
	tester := ui.NewTester(a.view, 1180, 760)
	tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
	tester.Frame()

	if err := tester.Click("Select all"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !a.hasSelection() {
		t.Fatal("select all chose nothing")
	}
	for _, action := range []string{"Download", "Move", "Delete", "Deselect"} {
		if err := tester.Click(action); err != nil && action != "Delete" {
			t.Fatalf("%s: %v", action, err)
		}
		tester.Frame()
		if action == "Deselect" {
			break
		}
	}
	if a.hasSelection() {
		t.Fatal("deselect left the selection in place")
	}
	if !tester.HasText("New folder") {
		t.Fatalf("the usual actions did not come back: %v", tester.Texts())
	}
}

// TestNativeCommandLinePage checks the Settings page that installs the
// command line and the skill: it lists the agents on this machine, and the
// copy buttons hand over a command that names this build.
func TestNativeCommandLinePage(t *testing.T) {
	a := nativeTestApp(t)
	a.section = sectionSettings
	a.settingsPage = pageCLI
	a.refreshCLIState()
	tester := ui.NewTester(a.view, 1180, 1000)
	tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
	tester.Frame()

	for _, want := range []string{
		"Command line", "bkt command",
		"Skill for coding agents", "Commands",
	} {
		if !tester.HasText(want) {
			t.Fatalf("the page is missing %q", want)
		}
	}
	// The reference has to name the commands the binary actually accepts.
	for name := range cli.Commands() {
		if !tester.HasText("bkt " + name) {
			t.Errorf("the reference does not show bkt %s", name)
		}
	}

	// Copying a sample puts something runnable on the clipboard, with the
	// program's own path quoted when it has spaces in it.
	a.copyCLICommand("ls", "Command")
	if got := tester.Clipboard(); !strings.Contains(got, " ls") {
		t.Fatalf("clipboard = %q", got)
	}

	// The smallest window still shows the page's first action.
	tester.SetSize(900, 560)
	tester.Frame()
	if !tester.HasText("bkt command") {
		t.Fatal("the page did not survive the smallest window")
	}
}

// TestNativeCommandLineInstall checks that the page's install action writes
// both the link and the skill, and that Remove takes them away again.
func TestNativeCommandLineInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".claude", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}

	a := nativeTestApp(t)
	a.section = sectionSettings
	a.settingsPage = pageCLI
	a.refreshCLIState()

	// Use a stand-in program path so the test never points PATH at a
	// temporary test binary.
	a.installCLIAt("/opt/Bucketeer.app/Contents/MacOS/Bucketeer")

	link := filepath.Join(home, ".local", "bin", "bkt")
	if target, err := os.Readlink(link); err != nil || target != "/opt/Bucketeer.app/Contents/MacOS/Bucketeer" {
		t.Fatalf("link = %q, %v", target, err)
	}
	entry := filepath.Join(home, ".claude", "skills", "bucketeer", "SKILL.md")
	body, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "/opt/Bucketeer.app/Contents/MacOS/Bucketeer") {
		t.Fatal("the installed skill does not name the installed program")
	}

	// The page reads its state for the program it installed for.
	a.refreshCLIStateAt("/opt/Bucketeer.app/Contents/MacOS/Bucketeer")
	if !a.cli.LinkExists || !a.cli.LinkCurrent {
		t.Fatalf("the page does not see its own link: exists=%v current=%v", a.cli.LinkExists, a.cli.LinkCurrent)
	}
	for _, status := range a.cli.Targets {
		if status.Name == "Claude Code" && (!status.Installed || !status.Current) {
			t.Fatalf("the page does not see the installed skill: %+v", status)
		}
	}

	a.removeCLIAt("/opt/Bucketeer.app/Contents/MacOS/Bucketeer")
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("remove kept the link: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(entry)); !os.IsNotExist(err) {
		t.Fatal("remove kept the skill")
	}
}

// TestSettingsSidebarAlignsLeft pins the layout the settings sheet needs:
// every row puts its glyph in the same slot and its label on the same left
// edge, so the list reads as one column. It measures the rows' ink, so a
// row that centres its contents, or whose glyphs are different widths,
// fails it.
func TestSettingsSidebarAlignsLeft(t *testing.T) {
	a := nativeTestApp(t)
	a.section = sectionSettings
	a.settingsPage = pagePreferences
	tester := ui.NewTester(a.view, 1180, 820)
	tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
	tester.Frame()

	// The labels' first ink, per row. Rows only, so no page title matches.
	// glyphCenter is the middle of the glyph's ink: the slot it occupies is
	// the same for every row, though the shapes' own ink starts and ends
	// differently.
	type edges struct{ glyphCenter, label int }
	var rows []edges
	for _, name := range []string{"Connections", "Command line", "About"} {
		row, ok := tester.Find(name)
		if !ok {
			t.Fatalf("%q is missing from the settings sidebar", name)
		}
		img := tester.Image()
		y := int(row.Y + row.H/2)
		var runs [][2]int
		inRun, start := false, 0
		for x := int(row.X); x < int(row.X+row.W); x++ {
			p := img.RGBAAt(x, y)
			ink := p.R < 210 || p.G < 210 || p.B < 210
			if ink && !inRun {
				inRun, start = true, x
			}
			if !ink && inRun {
				inRun = false
				runs = append(runs, [2]int{start, x - 1})
			}
		}
		if len(runs) < 3 {
			t.Fatalf("%q drew %d ink runs; it should draw a glyph and a label", name, len(runs))
		}
		// The glyph is the first run; the label begins where the gap after
		// it ends and the ink starts again, which is the run after next.
		glyphMiddle := (runs[0][0] + runs[0][1]) / 2
		rows = append(rows, edges{glyphCenter: glyphMiddle, label: runs[1][0]})
	}

	for i, r := range rows {
		if i == 0 {
			continue
		}
		if d := r.glyphCenter - rows[0].glyphCenter; d > 2 || d < -2 {
			t.Errorf("row %d centres its glyph at x=%d, but the first row centres it at %d", i, r.glyphCenter, rows[0].glyphCenter)
		}
		// The label follows the glyph in the same gap: its start may differ
		// by the width of the glyph's own ink, but not by a centring shift.
		if d := r.label - rows[0].label; d > 8 || d < -8 {
			t.Errorf("row %d starts its label at x=%d, while the first row starts at %d", i, r.label, rows[0].label)
		}
	}
	// The glyph sits against the row's left edge, not centred in it.
	row, _ := tester.Find("About")
	if inset := rows[0].glyphCenter - int(row.X); inset > 24 {
		t.Errorf("the glyph starts %d points in from the row's left edge, so the row is centred", inset)
	}
}

// TestSettingsPageTitles checks the settings pages all open with a title,
// and that the window's own title follows the page.
func TestSettingsPageTitles(t *testing.T) {
	a := nativeTestApp(t)
	a.section = sectionSettings
	tester := ui.NewTester(a.view, 1180, 820)
	tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})

	for _, page := range []struct {
		page  settingsPage
		title string
	}{
		{pagePreferences, "Preferences"},
		{pageConnections, "Connections"},
		{pageCLI, "Command line"},
		{pageAbout, "About"},
	} {
		a.settingsPage = page.page
		tester.Frame()
		if !tester.HasText(page.title) {
			t.Errorf("the %s page has no title", page.title)
		}
		if got := a.sectionTitle(); got != page.title {
			t.Errorf("the window title says %q on the %s page, want %q", got, page.title, page.title)
		}
	}
}

// TestAppearanceFollowsSystemOrPins checks the three-way choice: the
// default follows the system's appearance, and a pin holds against it.
func TestAppearanceFollowsSystemOrPins(t *testing.T) {
	a := nativeTestApp(t)
	if a.prefs.Appearance != "system" {
		t.Fatalf("default appearance = %q, want system", a.prefs.Appearance)
	}
	tester := ui.NewTester(a.view, 1180, 820)
	tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})

	tester.SetDark(true)
	tester.Frame()
	if !a.appearanceDark {
		t.Fatal("with the system dark, the app drew light")
	}
	tester.SetDark(false)
	tester.Frame()
	if a.appearanceDark {
		t.Fatal("with the system light, the app drew dark")
	}

	a.setAppearance("dark")
	tester.SetDark(false)
	tester.Frame()
	if !a.appearanceDark {
		t.Fatal("a pinned dark appearance did not hold against the system light")
	}
	a.setAppearance("light")
	tester.SetDark(true)
	tester.Frame()
	if a.appearanceDark {
		t.Fatal("a pinned light appearance did not hold against the system dark")
	}
	a.setAppearance("system")
	tester.SetDark(true)
	tester.Frame()
	if !a.appearanceDark {
		t.Fatal("returning to system did not resume following the system")
	}
}

// TestFocusFilterWithCmdF checks that ⌘F puts the caret in the filter
// field, where the next keystrokes land.
func TestFocusFilterWithCmdF(t *testing.T) {
	a := nativeTestApp(t)
	p := store.Profile{ID: "demo", Name: "Acme", Endpoint: "https://s3.example.com", Bucket: "magpie"}
	a.session = SessionInfo{Connected: true, Profile: &p, Host: "s3.example.com"}
	a.bucket = "magpie"
	a.section = sectionFiles
	tester := ui.NewTester(a.view, 1180, 820)
	tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})

	// ⌘F from another section first walks to the files.
	a.section = sectionDashboard
	tester.Frame()
	tester.Key(ui.Cmd, ui.KeyF)
	tester.Frame()
	tester.Type("todo")
	tester.Frame()
	if a.section != sectionFiles {
		t.Fatalf("section = %v, want files", a.section)
	}
	if a.search != "todo" {
		t.Fatalf("search = %q, want the typing to have landed in the filter", a.search)
	}
}

// TestSpaceTogglesPreview checks that Space opens the one chosen object's
// detail, and that Escape — the way out of every dialog in the app —
// closes it again.
func TestSpaceTogglesPreview(t *testing.T) {
	a := nativeTestApp(t)
	p := store.Profile{ID: "demo", Name: "Acme", Endpoint: "https://s3.example.com", Bucket: "magpie"}
	a.session = SessionInfo{Connected: true, Profile: &p, Host: "s3.example.com"}
	a.bucket = "magpie"
	a.section = sectionFiles
	a.listing = store.Listing{Items: []store.Entry{
		{Key: "photo.png", Name: "photo.png", Size: 2048},
		{Key: "notes.md", Name: "notes.md", Size: 412},
	}}
	tester := ui.NewTester(a.view, 1180, 820)
	tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 79})
	tester.Frame()

	// Nothing chosen: Space does nothing.
	tester.Key(0, ui.KeySpace)
	tester.Frame()
	if a.dlg != dialogNone {
		t.Fatal("Space with no selection opened a dialog")
	}

	a.sel.Add("photo.png")
	tester.Key(0, ui.KeySpace)
	tester.Frame()
	if a.dlg != dialogDetail {
		t.Fatalf("Space with a selection opened %v, want the detail", a.dlg)
	}
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if a.dlg != dialogNone {
		t.Fatal("Escape did not close the detail")
	}
}

// TestOpenSettingsPutsPreferencesForward checks the handler the app menu's
// Settings… item runs.
func TestOpenSettingsPutsPreferencesForward(t *testing.T) {
	a := nativeTestApp(t)
	a.section = sectionFiles
	a.settingsPage = pageCLI
	a.openSettings()
	if a.section != sectionSettings || a.settingsPage != pagePreferences {
		t.Fatalf("openSettings landed on %v / %v", a.section, a.settingsPage)
	}
}

// TestSidebarFoldersStayAtRoot checks the fixed Folders list: the sidebar
// keeps showing the bucket's own folders wherever the right side moves,
// and the highlight lands on the root folder that holds the prefix.
func TestSidebarFoldersStayAtRoot(t *testing.T) {
	a := nativeTestApp(t)
	p := store.Profile{ID: "demo", Name: "Acme", Endpoint: "https://s3.example.com", Bucket: "magpie"}
	a.session = SessionInfo{Connected: true, Profile: &p, Host: "s3.example.com"}
	a.bucket = "magpie"
	a.section = sectionFiles
	a.rootFolders = []store.Entry{
		{Key: "photos/", Name: "photos", Folder: true},
		{Key: "media/", Name: "media", Folder: true},
	}
	a.listing = store.Listing{Items: []store.Entry{
		{Key: "photos/2026-10/", Name: "2026-10", Folder: true},
		{Key: "photos/2026-09/", Name: "2026-09", Folder: true},
	}}

	// Inside photos/2026-10/, the sidebar still offers both root folders
	// and highlights photos.
	a.prefix = "photos/2026-10/"
	a.syncSidebarChoice()
	if a.sidebarID != "folder:photos/" {
		t.Fatalf("sidebar choice = %q, want the photos root folder", a.sidebarID)
	}
	folders := a.sidebarFolders()
	if len(folders) != 2 || folders[0].Key != "photos/" || folders[1].Key != "media/" {
		t.Fatalf("sidebar folders followed the browse: %+v", folders)
	}

	// Back at the root, no folder is highlighted.
	a.prefix = ""
	a.syncSidebarChoice()
	if a.sidebarID != navFiles {
		t.Fatalf("sidebar choice at root = %q, want all files", a.sidebarID)
	}
}
