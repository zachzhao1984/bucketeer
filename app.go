package main

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
)

// Section is the part of the app the sidebar has chosen.
type Section string

const (
	sectionDashboard Section = "dashboard"
	sectionFiles     Section = "files"
	sectionSettings  Section = "settings"
)

// ViewMode is how the files are drawn.
type ViewMode string

const (
	viewGrid ViewMode = "grid"
	viewList ViewMode = "list"
)

// SortField is what the listing is ordered by.
type SortField string

const (
	sortName     SortField = "name"
	sortSize     SortField = "size"
	sortModified SortField = "modified"
)

// App is the whole application: the storage service, the state every view
// reads, and the actions that change it.
//
// Views run on the main thread and read these fields directly. Background
// work changes them through Window.Update, so no field needs a lock: a
// goroutine that has a result to publish posts it, and it lands between
// two frames.
type App struct {
	svc            *Service
	prefs          Settings
	settingsWrites chan Settings
	win            *mygo.Window
	// ctx is cancelled when the window closes, so work in flight stops.
	ctx    context.Context
	cancel context.CancelFunc

	// Where the app is looking.
	section Section
	bucket  string
	prefix  string

	// What it knows.
	connections []store.Profile
	session     SessionInfo
	buckets     []store.Bucket
	listing     store.Listing
	stat        *store.Stat
	info        AppInfo

	// How the files are shown. search is the filter over what is already
	// loaded; results holds a deep search of the whole bucket, which the
	// file area shows in place of the current folder.
	mode          ViewMode
	sort          SortField
	sortDesc      bool
	filter        string
	search        string
	searchResults *store.Listing

	// Status of the load in flight.
	loading     bool
	statLoading bool
	err         string

	// Interaction: what the user has chosen, and where each list is
	// scrolled. files drives the table and grid drives the grid of cards;
	// only one of them builds in a frame, which is why they are separate.
	sel           ui.Selection[string]
	fileList      ui.ListState
	grid          ui.GridState
	transfersOpen bool
	// sidebarID is what the sidebar shows as chosen; transfersChoice is
	// the transient press on its Transfers item, which toggles a panel
	// rather than staying chosen.
	sidebarID       string
	transfersChoice string
	foldersOpen     bool
	chipOpen        bool
	menuOpen        bool
	selMenuOpen     bool
	crumb           int
	cursor          int
	tableSort       ui.SortOrder

	// Thumbnails are the pictures the grid shows for image objects, by
	// key. A key maps to nil when the app tried and there is nothing to
	// show, so it does not try again every frame.
	thumbs       map[string]*ui.Bitmap
	thumbLoading map[string]bool

	// The themes for the system's two appearances, built once.
	lightTheme *ui.Theme
	darkTheme  *ui.Theme

	// services is the window's clipboard and URL services, kept from the
	// first frame so work that outlives a frame still has them.
	services ui.Services

	// notice is a message the next frame shows as a toast and forgets.
	notice string

	// The dialogs, and the form each one is filling in.
	dlg     dialog
	conn    connForm
	input   string
	dest    string
	share   shareState
	detail  detailState
	confirm confirmState

	// seq guards against a slow result overwriting a newer one.
	seq           int
	statSeq       int
	connectionSeq int
	thumbSeq      int
	thumbSlots    chan struct{}
	formSeq       int
	shareSeq      int
}

// dialog names the modal the app is showing.
type dialog int

const (
	dialogNone dialog = iota
	dialogConnection
	dialogNewFolder
	dialogNewBucket
	dialogRename
	dialogMove
	dialogShare
	dialogDetail
)

// connForm is the connection dialog's fields. An id means the dialog edits
// that saved connection; an empty secret keeps the one already stored.
type connForm struct {
	id        string
	editing   bool
	name      string
	provider  string
	endpoint  string
	region    string
	bucket    string
	accessKey string
	secretKey string
	pathStyle bool
	busy      bool
	result    *TestResult
	err       string
}

// shareState is the link the share dialog made.
type shareState struct {
	key   string
	url   string
	hours int
	err   string
	busy  bool
}

// detailState is what the object dialog shows: its metadata, and a preview
// when the app has one.
type detailState struct {
	key  string
	info store.ObjectInfo
	text *TextPreview
	img  *ui.Bitmap
	err  string
	busy bool
}

// confirmState is a question the app asks before doing something it cannot
// undo.
type confirmState struct {
	open    bool
	title   string
	message string
	button  string
	action  func()
}

// newApp builds the app around a service.
func newApp(svc *Service) *App {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		svc:            svc,
		prefs:          svc.GetSettings(),
		settingsWrites: make(chan Settings, 128),
		cursor:         -1,
		thumbSlots:     make(chan struct{}, 4),
		ctx:            ctx,
		cancel:         cancel,
		section:        sectionDashboard,
		mode:           viewGrid,
		sort:           sortName,
		filter:         "all",
		info:           svc.Info(),

		thumbs:       map[string]*ui.Bitmap{},
		thumbLoading: map[string]bool{},

		sortDesc:    false,
		foldersOpen: true,
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case settings := <-a.settingsWrites:
				if err := svc.SetSettings(settings); err != nil {
					a.update(func() { a.fail(err) })
				}
			}
		}
	}()
	return a
}

// attach records the window and starts the app: the saved connections, the
// settings, and whatever the first view needs.
func (a *App) attach(win *mygo.Window) {
	a.win = win
	a.mode = ViewMode(a.svc.GetSettings().ViewMode)
	if a.mode != viewList {
		a.mode = viewGrid
	}
	a.svc.setTransferListener(func() { a.update(func() {}) })
	a.setTitle()
	a.refreshConnections()

}

// close stops the work in flight and the transfers.
func (a *App) close() {
	a.cancel()
	a.svc.Close()
}

/* -------------------------------------------------------------------------
   Talking to the window
   ------------------------------------------------------------------------- */

// update runs fn on the main thread and redraws, from any goroutine. It is
// how background work publishes its result.
func (a *App) update(fn func()) {
	if a.ctx.Err() != nil || a.win == nil || a.win.IsDestroyed() {
		return
	}
	a.win.Update(func() {
		if a.ctx.Err() == nil {
			fn()
		}
	})
}

// next returns a token for an operation whose result must not overwrite a
// newer one, as a slow listing arriving after the user opened another
// folder.
func (a *App) next() int {
	a.seq++
	return a.seq
}

// current reports whether a token is still the newest of its kind.
func (a *App) current(token int) bool { return token == a.seq }

/* -------------------------------------------------------------------------
   Where the app is looking
   ------------------------------------------------------------------------- */

// browsing reports whether a bucket is open.
func (a *App) browsing() bool { return a.bucket != "" }

// setSection shows another part of the app.
func (a *App) setSection(s Section) {
	if a.section == s {
		return
	}
	a.section = s
	if s == sectionDashboard && a.browsing() && a.stat == nil && !a.statLoading {
		a.loadStat()
	}
}

// openBucket browses a bucket from its root.
func (a *App) openBucket(name string) {
	a.bucket = name
	a.prefix = ""
	a.dlg = dialogNone
	a.section = sectionFiles
	a.stat = nil
	a.statLoading = false
	a.statSeq++
	a.resetThumbnails()
	a.resetListing()
	a.setTitle()
	a.loadListing()
}

// openPrefix browses a folder inside the open bucket.
func (a *App) openPrefix(prefix string) {
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	a.prefix = prefix
	a.resetListing()
	a.setTitle()
	a.loadListing()
}

// openEntry follows an entry: a folder opens, a file shows its detail.
func (a *App) openEntry(e store.Entry) {
	if e.Folder {
		a.openPrefix(e.Key)
		return
	}
	a.showDetail(e.Key)
}

// goUp returns to the parent folder, or to the bucket list at the root.
func (a *App) goUp() {
	if a.prefix != "" {
		parent := strings.TrimSuffix(a.prefix, "/")
		if i := strings.LastIndex(parent, "/"); i >= 0 {
			a.openPrefix(parent[:i+1])
		} else {
			a.openPrefix("")
		}
		return
	}
	if a.browsing() {
		a.leaveBucket()
	}
}

// leaveBucket returns to the list of buckets.
func (a *App) leaveBucket() {
	a.bucket = ""
	a.prefix = ""
	a.stat = nil
	a.searchResults = nil
	a.search = ""
	a.resetListing()
	a.loadBuckets()
}

// resetListing clears what belongs to the folder being left, so nothing
// from it shows in the one being opened.
func (a *App) resetListing() {
	a.listing = store.Listing{}
	a.search = ""
	a.searchResults = nil
	a.sel.Clear()
	a.fileList = ui.ListState{}
	a.grid = ui.GridState{}
	a.err = ""
	a.cursor = -1
	a.loading = true
}

// crumbs is the path to the folder being browsed, each part with the
// prefix it opens.
type crumb struct {
	Label  string
	Prefix string
}

// crumbs returns the breadcrumb of the current location.
func (a *App) crumbs() []crumb {
	if !a.browsing() {
		return nil
	}
	out := []crumb{{Label: a.bucket, Prefix: ""}}
	acc := ""
	for _, segment := range strings.Split(a.prefix, "/") {
		if segment == "" {
			continue
		}
		acc += segment + "/"
		out = append(out, crumb{Label: segment, Prefix: acc})
	}
	return out
}

// folderName is the name of the folder being browsed, for messages.
func (a *App) folderName() string {
	if a.prefix == "" {
		return a.bucket
	}
	return path.Base(strings.TrimSuffix(a.prefix, "/"))
}

// setTitle names the window after where the app is looking.
func (a *App) setTitle() {
	if a.win == nil {
		return
	}
	name := appName
	switch {
	case a.browsing() && a.prefix != "":
		name = a.folderName() + " — " + appName
	case a.browsing():
		name = a.bucket + " — " + appName
	case a.session.Profile != nil:
		name = a.session.Profile.Name + " — " + appName
	}
	a.win.SetTitle(name)
}

/* -------------------------------------------------------------------------
   What the file area shows
   ------------------------------------------------------------------------- */

// source returns the listing the file area is showing: a deep search's
// results when there is one, the current folder otherwise.
func (a *App) source() store.Listing {
	if a.searchResults != nil {
		return *a.searchResults
	}
	return a.listing
}

// visible returns the entries the file area shows, filtered and sorted.
// Folders lead, as in a file manager, whatever the sort.
func (a *App) visible() []store.Entry {
	items := slices.Clone(a.source().Items)
	if query := strings.ToLower(strings.TrimSpace(a.search)); query != "" && a.searchResults == nil {
		items = slices.DeleteFunc(items, func(e store.Entry) bool { return !strings.Contains(strings.ToLower(e.Name), query) })
	}
	if a.filter != "all" {
		items = slices.DeleteFunc(items, func(e store.Entry) bool {
			if a.filter == "folders" {
				return !e.Folder
			}
			return e.Folder || string(kindOf(e.Key, false)) != a.filter
		})
	}
	desc := 1
	if a.sortDesc {
		desc = -1
	}
	slices.SortStableFunc(items, func(x, y store.Entry) int {
		if x.Folder != y.Folder {
			if x.Folder {
				return -1
			}
			return 1
		}
		switch a.sort {
		case sortSize:
			return desc * compareInt64(x.Size, y.Size)
		case sortModified:
			return desc * x.LastModified.Compare(y.LastModified)
		default:
			return desc * strings.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name))
		}
	})
	return items
}

func compareInt64(x, y int64) int {
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	default:
		return 0
	}
}

// chosen returns the entries the selection refers to.
func (a *App) chosen() []store.Entry {
	items := a.source().Items
	out := make([]store.Entry, 0, a.sel.Len())
	for _, e := range items {
		if a.sel.Has(e.Key) {
			out = append(out, e)
		}
	}
	return out
}

// chosenKeys returns the selected keys, sorted, as the service wants them.
func (a *App) chosenKeys() []string {
	keys := slices.Collect(a.sel.All())
	slices.Sort(keys)
	return keys
}

// chosenFolders reports whether everything chosen is a folder, which is
// what the app needs to know to leave folders out of a share, a preview or
// a download of one file.
func (a *App) chosenFolders() bool {
	chosen := a.chosen()
	if len(chosen) == 0 {
		return false
	}
	for _, e := range chosen {
		if !e.Folder {
			return false
		}
	}
	return true
}

/* -------------------------------------------------------------------------
   Loading
   ------------------------------------------------------------------------- */

// refreshConnections reloads the saved connections.
func (a *App) refreshConnections() {
	go func() {
		list, err := a.svc.ListConnections()
		a.update(func() {
			if err != nil {
				a.err = err.Error()
				return
			}
			a.connections = list
			a.session = a.svc.CurrentConnection()
			a.setTitle()
		})
	}()
}

// loadBuckets lists the endpoint's buckets.
func (a *App) loadBuckets() {
	if !a.svc.Connected() {
		a.loading = false
		return
	}
	token := a.next()
	a.loading = true
	go func() {
		buckets, err := a.svc.ListBuckets(a.ctx)
		a.update(func() {
			if !a.current(token) {
				return
			}
			a.loading = false
			if err != nil {
				a.err = err.Error()
				return
			}
			a.err = ""
			a.buckets = buckets
		})
	}()
}

// loadListing lists the bucket and folder being browsed.
func (a *App) loadListing() {
	if !a.svc.Connected() {
		a.loading = false
		return
	}
	token := a.next()
	bucket, prefix := a.bucket, a.prefix
	a.loading = true
	go func() {
		listing, err := a.svc.ListObjects(a.ctx, bucket, prefix)
		a.update(func() {
			if !a.current(token) || bucket != a.bucket || prefix != a.prefix {
				return
			}
			a.loading = false
			if err != nil {
				a.err = err.Error()
				return
			}
			a.err = ""
			a.listing = listing
			a.loadThumbnails(listing.Items)
		})
	}()
}

// loadStat measures the bucket for the dashboard.
func (a *App) loadStat() {
	if !a.svc.Connected() || !a.browsing() {
		return
	}
	a.statSeq++
	token := a.statSeq
	bucket := a.bucket
	a.statLoading = true
	go func() {
		stat, err := a.svc.BucketStat(a.ctx, bucket, 50_000)
		a.update(func() {
			if token != a.statSeq || bucket != a.bucket {
				return
			}
			a.statLoading = false
			if err != nil {
				a.err = err.Error()
				return
			}
			a.stat = &stat
		})
	}()
}

// deepSearch looks for a query among every object in the bucket, not just
// the folder being browsed.
func (a *App) deepSearch() {
	query := strings.TrimSpace(a.search)
	if !a.svc.Connected() || !a.browsing() || query == "" {
		a.searchResults = nil
		a.loadListing()
		return
	}
	token := a.next()
	bucket := a.bucket
	a.loading = true
	go func() {
		listing, err := a.svc.SearchObjects(a.ctx, bucket, "", query, 2000)
		a.update(func() {
			if !a.current(token) || bucket != a.bucket {
				return
			}
			a.loading = false
			if err != nil {
				a.err = err.Error()
				return
			}
			a.err = ""
			a.searchResults = &listing
			a.sel.Clear()
			a.loadThumbnails(listing.Items)
		})
	}()
}

// clearSearch leaves the deep-search results and shows the folder again.
func (a *App) clearSearch() {
	a.search = ""
	if a.searchResults != nil {
		a.searchResults = nil
		a.sel.Clear()
		a.loadListing()
	}
}

// refresh reloads whatever the current section shows.
func (a *App) refresh() {
	switch a.section {
	case sectionDashboard:
		a.stat = nil
		a.loadStat()
	case sectionFiles:
		if a.searchResults != nil {
			a.deepSearch()
			return
		}
		if a.browsing() {
			a.loadListing()
		} else {
			a.loadBuckets()
		}
	default:
		a.refreshConnections()
	}
}

// loadThumbnails reads the pictures the grid shows for the images in a
// listing, a few at a time and only for the kinds the app can decode. It
// runs on the main thread (it is called from an update), and starts one
// goroutine per object, which publishes through update in turn.
func (a *App) resetThumbnails() {
	a.thumbSeq++
	a.thumbs = map[string]*ui.Bitmap{}
	a.thumbLoading = map[string]bool{}
}

func (a *App) loadThumbnails(items []store.Entry) {
	if !a.prefs.ShowThumbnails {
		return
	}
	bucket, generation := a.bucket, a.thumbSeq
	for _, e := range items {
		if len(a.thumbs)+len(a.thumbLoading) >= 100 {
			break
		}
		if e.Folder || !isImageKey(e.Key) || e.Size > MaxImageBytes {
			continue
		}
		if _, tried := a.thumbs[e.Key]; tried || a.thumbLoading[e.Key] {
			continue
		}
		a.thumbLoading[e.Key] = true
		key := e.Key
		go func() {
			select {
			case a.thumbSlots <- struct{}{}:
			case <-a.ctx.Done():
				return
			}
			defer func() { <-a.thumbSlots }()
			data, err := a.svc.ReadImage(a.ctx, bucket, key)
			var bmp *ui.Bitmap
			if err == nil {
				bmp, _ = decodePreview(data)
			}
			a.update(func() {
				if generation != a.thumbSeq || bucket != a.bucket {
					return
				}
				delete(a.thumbLoading, key)
				a.thumbs[key] = bmp
			})
		}()
	}
}

/* -------------------------------------------------------------------------
   Connecting
   ------------------------------------------------------------------------- */

// connect opens a saved connection and browses its buckets.
func (a *App) connect(id string) {
	a.connectionSeq++
	token := a.connectionSeq
	a.next()
	a.statSeq++
	a.loading = true
	go func() {
		session, err := a.svc.Connect(a.ctx, id)
		a.update(func() {
			if token != a.connectionSeq {
				return
			}
			a.loading = false
			if err != nil {
				a.fail(err)
				return
			}
			a.err = ""
			a.session = session
			a.section = sectionFiles
			a.resetThumbnails()
			if session.Profile != nil && session.Profile.Bucket != "" {
				a.openBucket(session.Profile.Bucket)
			} else {
				a.leaveBucket()
			}
			a.setTitle()
		})
	}()
}

// disconnect closes the connection and returns to the welcome screen.
func (a *App) disconnect() {
	a.connectionSeq++
	a.next()
	a.statSeq++
	a.statLoading = false
	a.loading = false
	a.resetThumbnails()
	a.svc.Disconnect()
	a.session = SessionInfo{}
	a.buckets = nil
	a.listing = store.Listing{}
	a.stat = nil
	a.searchResults = nil
	a.section = sectionFiles
	a.bucket = ""
	a.prefix = ""
	a.err = ""
	a.thumbs = map[string]*ui.Bitmap{}
	a.thumbLoading = map[string]bool{}
	a.sel.Clear()
	a.setTitle()
}

/* -------------------------------------------------------------------------
   Settings
   ------------------------------------------------------------------------- */

// saveSettings writes the preferences, and applies the ones the app keeps
// a copy of.
func (a *App) saveSettings() {
	settings := a.svc.GetSettings()
	go func() {
		if err := a.svc.SetSettings(settings); err != nil {
			a.update(func() { a.err = err.Error() })
		}
	}()
}

// setViewMode switches between the grid of cards and the table.
func (a *App) setViewMode(mode ViewMode) {
	a.mode = mode
	a.prefs.ViewMode = string(mode)
	a.persistSettings()
}

/* -------------------------------------------------------------------------
   Transfers
   ------------------------------------------------------------------------- */

// transfers returns the live transfer list.
func (a *App) transfers() []store.Transfer { return a.svc.ListTransfers() }

// activeTransfers counts the transfers still going.
func (a *App) activeTransfers() int { return a.svc.ActiveTransfers() }

/* -------------------------------------------------------------------------
   Messages
   ------------------------------------------------------------------------- */

// fail shows an error in the banner over the file area. Errors that came
// from a cancelled context are the app's own doing, so they are dropped.
func (a *App) fail(err error) {
	if err == nil || a.ctx.Err() != nil {
		return
	}
	a.err = err.Error()
}

// toast shows a message near the bottom of the window. Views call it with
// the Context they are building.
func toast(c *ui.Context, format string, args ...any) {
	c.Toast(fmt.Sprintf(format, args...))
}

// now is the time the interface formats relative dates against. A single
// value per frame keeps a listing consistent.
func now() time.Time { return time.Now() }

// appName is the product name, shown in the window title.
const appName = "Bucketeer"
