package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
)

// The actions: everything the interface does beyond showing state. Each
// runs on the main thread when a control reports input, and hands the slow
// part to a goroutine that publishes its result through Window.Update.
//
// A dialog is opened by setting the app's dialog field and filling in the
// form it edits; the views build it from there.

/* -------------------------------------------------------------------------
   Uploading and downloading
   ------------------------------------------------------------------------- */

// upload asks for files and sends them to the folder being browsed.
func (a *App) upload() {
	if !a.browsing() || !a.session.Connected {
		a.notice = "Open a bucket before uploading."
		return
	}
	bucket, prefix := a.bucket, a.prefix
	go func() {
		paths, err := a.svc.PickUploadFiles(a.win)
		if err != nil {
			a.update(func() { a.fail(err) })
			return
		}
		if len(paths) == 0 {
			return
		}
		a.queueUpload(bucket, prefix, paths)
	}()
}

func (a *App) uploadPaths(paths []string) {
	if !a.browsing() || !a.session.Connected {
		a.notice = "Open a bucket before uploading."
		return
	}
	bucket, prefix := a.bucket, a.prefix
	go a.queueUpload(bucket, prefix, append([]string(nil), paths...))
}

func (a *App) queueUpload(bucket, prefix string, paths []string) {
	queued, err := a.svc.UploadPaths(a.ctx, bucket, prefix, paths)
	a.update(func() {
		if queued > 0 {
			a.transfersOpen = true
			a.notice = fmt.Sprintf("Queued %d uploads to %s/%s", queued, bucket, prefix)
		}
		a.fail(err)
	})
}

// download asks where to save the chosen entries and queues them.
func (a *App) download() {
	chosen := a.chosen()
	if len(chosen) == 0 {
		return
	}
	keys := a.chosenKeys()
	bucket := a.bucket
	go func() {
		dir, err := a.svc.PickDownloadDir(a.win)
		if err != nil || dir == "" {
			return
		}
		result, err := a.svc.DownloadObjects(a.ctx, bucket, keys, dir)
		a.update(func() {
			if err != nil {
				a.fail(err)
				return
			}
			a.transfersOpen = true
			message := fmt.Sprintf("Downloading %s", plural(result.Queued, "object", "objects"))
			if result.Truncated {
				message += " (the folder held more, and the rest were left alone)"
			}
			a.notice = message
		})
	}()
}

// saveAs asks where to save one object.
func (a *App) saveAs(key string) {
	bucket := a.bucket
	go func() {
		path, err := a.svc.SaveObjectAs(a.win, bucket, key)
		a.update(func() {
			if err != nil {
				a.fail(err)
				return
			}
			if path == "" {
				return
			}
			a.transfersOpen = true
			a.notice = "Saving " + path
		})
	}()
}

/* -------------------------------------------------------------------------
   Making, renaming, moving and removing
   ------------------------------------------------------------------------- */

// makeFolder creates a folder in the folder being browsed.
func (a *App) makeFolder(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	bucket, prefix := a.bucket, a.prefix
	a.dlg = dialogNone
	go func() {
		if _, err := a.svc.CreateFolder(a.ctx, bucket, prefix, name); err != nil {
			a.update(func() { a.fail(err) })
			return
		}
		a.update(func() {
			a.notice = "Created " + name
			a.loadListing()
		})
	}()
}

// makeBucket creates a bucket on the endpoint.
func (a *App) makeBucket(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	a.dlg = dialogNone
	go func() {
		if err := a.svc.CreateBucket(a.ctx, name); err != nil {
			a.update(func() { a.fail(err) })
			return
		}
		a.update(func() {
			a.notice = "Created bucket " + name
			a.loadBuckets()
		})
	}()
}

// rename gives an object another name in the folder it is in.
func (a *App) rename(key, name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	bucket := a.bucket
	a.dlg = dialogNone
	go func() {
		if err := a.svc.RenameObject(a.ctx, bucket, key, name); err != nil {
			a.update(func() { a.fail(err) })
			return
		}
		a.update(func() {
			a.notice = "Renamed to " + name
			a.loadListing()
		})
	}()
}

// move sends the chosen entries into another folder of the same bucket.
// The destination may be a folder that does not exist yet.
func (a *App) move(dest string) {
	keys := a.chosenKeys()
	if len(keys) == 0 {
		return
	}
	dest = strings.TrimSpace(strings.TrimPrefix(dest, "/"))
	if dest != "" && !strings.HasSuffix(dest, "/") {
		dest += "/"
	}
	bucket := a.bucket
	a.dlg = dialogNone
	go func() {
		moved, err := a.svc.MoveObjects(a.ctx, bucket, keys, dest)
		if err != nil {
			a.update(func() { a.fail(err) })
			return
		}
		a.update(func() {
			where := dest
			if where == "" {
				where = "the bucket root"
			} else {
				where = store.FolderName(dest)
			}
			a.notice = fmt.Sprintf("Moved %s to %s", plural(moved, "object", "objects"), where)
			a.sel.Clear()
			a.loadListing()
		})
	}()
}

// remove deletes the chosen entries. Folders go with everything under
// them, which is why the app asks first.
func (a *App) remove() {
	keys := a.chosenKeys()
	if len(keys) == 0 {
		return
	}
	bucket := a.bucket
	a.sel.Clear()
	go func() {
		deleted, err := a.svc.DeleteObjects(a.ctx, bucket, keys)
		if err != nil {
			a.update(func() { a.fail(err) })
			return
		}
		a.update(func() {
			a.notice = "Deleted " + plural(deleted, "object", "objects")
			a.loadListing()
		})
	}()
}

// removeBucket deletes an empty bucket.
func (a *App) removeBucket(name string) {
	go func() {
		if err := a.svc.DeleteBucket(a.ctx, name); err != nil {
			a.update(func() { a.fail(err) })
			return
		}
		a.update(func() {
			a.notice = "Deleted bucket " + name
			a.loadBuckets()
		})
	}()
}

// deleteChosen asks before removing, when the preferences say to, and does
// it at once otherwise.
func (a *App) deleteChosen() {
	chosen := a.chosen()
	if len(chosen) == 0 {
		return
	}
	if !a.prefs.ConfirmDelete {
		a.remove()
		return
	}
	folders := 0
	for _, e := range chosen {
		if e.Folder {
			folders++
		}
	}
	message := "This cannot be undone."
	switch {
	case folders > 0 && len(chosen) == folders:
		message = "The folders are removed with everything inside them. This cannot be undone."
	case folders > 0:
		message = "The folders are removed with everything inside them, and the files are deleted. This cannot be undone."
	}
	a.ask("Delete "+describe(chosen)+"?", message, "Delete", a.remove)
}

// describe names what a set of entries is, for a question or a message.
func describe(items []store.Entry) string {
	if len(items) == 1 {
		if items[0].Folder {
			return "the folder “" + items[0].Name + "”"
		}
		return "“" + items[0].Name + "”"
	}
	return plural(len(items), "item", "items")
}

// ask puts a question to the user, and runs action if they agree.
func (a *App) ask(title, message, confirm string, action func()) {
	a.confirm = confirmState{open: true, title: title, message: message, action: action, button: confirm}
}

/* -------------------------------------------------------------------------
   Sharing and previewing
   ------------------------------------------------------------------------- */

// shareLink makes a temporary link to an object.
func (a *App) shareLink(key string, hours int) {
	a.shareSeq++
	token := a.shareSeq
	a.share = shareState{key: key, hours: hours, busy: true}
	bucket := a.bucket
	go func() {
		url, err := a.svc.PresignObject(a.ctx, bucket, key, hours)
		a.update(func() {
			if token != a.shareSeq || a.dlg != dialogShare {
				return
			}
			a.share.busy = false
			if err != nil {
				a.share.err = err.Error()
				return
			}
			a.share.url = url
		})
	}()
}

// showDetail opens the dialog describing an object, reading a preview when
// the app has one to show.
func (a *App) showDetail(key string) {
	a.dlg = dialogDetail
	a.detail = detailState{key: key, busy: true}
	bucket := a.bucket
	go func() {
		info, err := a.svc.ObjectInfo(a.ctx, bucket, key)
		var text *TextPreview
		var img *ui.Bitmap
		switch {
		case err == nil && isTextKey(key):
			if preview, readErr := a.svc.ReadText(a.ctx, bucket, key); readErr == nil {
				text = &preview
			}
		case err == nil && isImageKey(key):
			if data, readErr := a.svc.ReadImage(a.ctx, bucket, key); readErr == nil {
				if decoded, decodeErr := decodePreview(data); decodeErr == nil {
					img = decoded
				}
			}
		}
		a.update(func() {
			if a.dlg != dialogDetail || a.detail.key != key {
				return // the user closed it, or opened another object
			}
			a.detail.busy = false
			if err != nil {
				a.detail.err = err.Error()
				return
			}
			a.detail.info = info
			a.detail.text = text
			a.detail.img = img
		})
	}()
}

// openInBrowser opens a temporary link to an object in the user's browser,
// which is how anything the app cannot draw itself is shown: a video, a
// PDF, a sound.
func (a *App) openInBrowser(key string) {
	bucket := a.bucket
	go func() {
		url, err := a.svc.PresignObject(a.ctx, bucket, key, 1)
		if err != nil {
			a.update(func() { a.fail(err) })
			return
		}
		a.update(func() { a.services.OpenURL(url) })
	}()
}

// copyText puts text on the clipboard and says so.
func (a *App) copyText(text, what string) {
	a.services.WriteClipboard(text)
	a.notice = what
}

/* -------------------------------------------------------------------------
   The connection dialog
   ------------------------------------------------------------------------- */

// openConnection opens the connection dialog, editing a saved connection
// or making a new one.
func (a *App) openConnection(existing *store.Profile) {
	a.formSeq++
	form := connForm{provider: string(store.ProviderR2), region: "auto"}
	if existing != nil {
		form = connForm{
			id:        existing.ID,
			editing:   true,
			name:      existing.Name,
			provider:  string(existing.Provider),
			endpoint:  existing.Endpoint,
			region:    existing.Region,
			bucket:    existing.Bucket,
			accessKey: existing.AccessKeyID,
			pathStyle: existing.PathStyle,
		}
	}
	a.conn = form
	a.dlg = dialogConnection
}

// testConnection checks the credentials in the form without saving them.
func (a *App) testConnection() {
	if a.conn.busy {
		return
	}
	token := a.formSeq
	in, ok := a.connInput()
	if !ok {
		return
	}
	a.conn.busy = true
	a.conn.result = nil
	a.conn.err = ""
	go func() {
		result, err := a.svc.TestConnection(in)
		a.update(func() {
			if a.dlg != dialogConnection || token != a.formSeq {
				return
			}
			a.conn.busy = false
			if err != nil {
				a.conn.err = err.Error()
				return
			}
			a.conn.result = &result
		})
	}()
}

// saveConnection stores the form's connection and connects to it.
func (a *App) saveConnection() {
	if a.conn.busy {
		return
	}
	token := a.formSeq
	in, ok := a.connInput()
	if !ok {
		return
	}
	id := a.conn.id
	a.conn.busy = true
	a.conn.err = ""
	go func() {
		profile, err := a.svc.SaveConnection(id, in)
		a.update(func() {
			if a.dlg != dialogConnection || token != a.formSeq {
				return
			}
			a.conn.busy = false
			if err != nil {
				a.conn.err = err.Error()
				return
			}
			a.dlg = dialogNone
			a.refreshConnections()
			a.notice = "Saved " + profile.Name
			a.connect(profile.ID)
		})
	}()
}

// connInput reads the form into the input the service takes, asking for
// the fields a connection cannot do without.
func (a *App) connInput() (store.Input, bool) {
	form := &a.conn
	if strings.TrimSpace(form.endpoint) == "" {
		form.err = "Enter the endpoint, as s3.example.com."
		return store.Input{}, false
	}
	if strings.TrimSpace(form.accessKey) == "" {
		form.err = "Enter the access key ID."
		return store.Input{}, false
	}
	if !form.editing && strings.TrimSpace(form.secretKey) == "" {
		form.err = "Enter the secret access key."
		return store.Input{}, false
	}
	if form.endpoint == "" {
		form.err = "Enter the endpoint."
		return store.Input{}, false
	}
	form.err = ""
	return store.Input{
		Name:        form.name,
		Provider:    form.provider,
		Endpoint:    form.endpoint,
		Region:      form.region,
		Bucket:      form.bucket,
		AccessKeyID: form.accessKey,
		SecretKey:   form.secretKey,
		PathStyle:   form.pathStyle,
	}, true
}

// removeConnection forgets a saved connection.
func (a *App) removeConnection(profile store.Profile) {
	id := profile.ID
	go func() {
		if err := a.svc.DeleteConnection(id); err != nil {
			a.update(func() { a.fail(err) })
			return
		}
		a.update(func() {
			if p := a.session.Profile; p != nil && p.ID == id {
				a.disconnect()
			}
			a.notice = "Removed " + profile.Name
			a.refreshConnections()
		})
	}()
}

/* -------------------------------------------------------------------------
   Selection
   ------------------------------------------------------------------------- */

// selectAll chooses every entry the file area shows.
func (a *App) selectAll() {
	for _, e := range a.visible() {
		a.sel.Add(e.Key)
	}
}

// chooseOnly makes one entry the whole selection.
func (a *App) chooseOnly(key string) {
	a.sel.Clear()
	a.sel.Add(key)
}

// hasSelection reports whether anything is chosen.
func (a *App) hasSelection() bool { return a.sel.Len() > 0 }

// one reports the single chosen entry, and whether there is exactly one.
func (a *App) one() (store.Entry, bool) {
	chosen := a.chosen()
	if len(chosen) != 1 {
		return store.Entry{}, false
	}
	return chosen[0], true
}

/* -------------------------------------------------------------------------
   Transfers
   ------------------------------------------------------------------------- */

// cancelTransfer stops a transfer.
func (a *App) cancelTransfer(id string) {
	a.svc.CancelTransfer(id)
	a.update(func() {})
}

// clearTransfers forgets the transfers that have stopped.
func (a *App) clearTransfers() {
	a.svc.ClearFinishedTransfers()
	a.update(func() {})
}

// busy marks the app as working, for a message the interface shows. The
// notice is cleared as the frame shows it, so a result replaces it.
func (a *App) busy(message string) {
	if a.notice == "" {
		a.notice = message
	}
}

// errNoConnection is what an action reports when nothing is connected.
var errNoConnection = errors.New("connect to a storage endpoint first")
