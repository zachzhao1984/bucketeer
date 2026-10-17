package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
)

// dialogs draws the modal the app has open, if any. Each is a function of
// the app's form state, so nothing is kept between frames but the state
// itself.
func (a *App) dialogs(c *ui.Context) {
	switch a.dlg {
	case dialogConnection:
		a.connectionDialog(c)
	case dialogNewFolder:
		a.textDialog(c, "New folder", "Name", "Folder", a.makeFolder)
	case dialogNewBucket:
		a.textDialog(c, "New bucket", "Name", "Bucket name", a.makeBucket)
	case dialogRename:
		a.renameDialog(c)
	case dialogMove:
		a.moveDialog(c)
	case dialogShare:
		a.shareDialog(c)
	case dialogDetail:
		a.detailDialog(c)
	}
	a.confirmDialog(c)
}

// confirmDialog asks the question set by ask, and runs its action when the
// user agrees.
func (a *App) confirmDialog(c *ui.Context) {
	if !a.confirm.open {
		return
	}
	switch ui.AlertDialog(c, &a.confirm.open, a.confirm.title, a.confirm.message, "Cancel", a.confirm.button) {
	case 1:
		action := a.confirm.action
		a.confirm.action = nil
		if action != nil {
			action()
		}
	}
}

/* -------------------------------------------------------------------------
   Small dialogs that take one name
   ------------------------------------------------------------------------- */

// openNewFolder asks for the name of a folder to make.
func (a *App) openNewFolder() {
	a.input = ""
	a.dlg = dialogNewFolder
}

// openNewBucket asks for the name of a bucket to make.
func (a *App) openNewBucket() {
	a.input = ""
	a.dlg = dialogNewBucket
}

// openRename asks for another name for an object.
func (a *App) openRename(e store.Entry) {
	a.input = e.Name
	a.dlg = dialogRename
	a.dest = e.Key
}

// openMove asks where to move the chosen entries.
func (a *App) openMove() {
	a.dest = ""
	a.dlg = dialogMove
}

// textDialog draws a modal that asks for one name and calls accept with it.
func (a *App) textDialog(c *ui.Context, title, label, placeholder string, accept func(string)) {
	t := c.Theme()
	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(360).Gap(14).Children(func() {
			ui.Text(c, title).FontSize(16).Bold()
			field := ui.TextInput(c.Key("name"), &a.input).Placeholder(placeholder).Label(label).AutoFocus()
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "Cancel").Clicked() {
					a.dlg = dialogNone
				}
				submit := ui.PrimaryButton(c, title)
				if field.Submitted() || submit.Clicked() {
					if strings.TrimSpace(a.input) != "" {
						accept(a.input)
					}
				}
			})
		})
	})
	if !open && a.dlg != dialogNone {
		a.dlg = dialogNone
	}
	_ = t
}

// renameDialog asks for another name for the object the app is renaming.
func (a *App) renameDialog(c *ui.Context) {
	key := a.dest
	a.textDialog(c, "Rename", "New name", "File name", func(name string) {
		a.rename(key, name)
	})
}

// moveDialog asks for the folder to move the chosen entries into.
func (a *App) moveDialog(c *ui.Context) {
	t := c.Theme()
	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(420).Gap(14).Children(func() {
			ui.Text(c, "Move "+plural(a.sel.Len(), "item", "items")).FontSize(16).Bold()
			ui.Text(c, "Give a folder in "+a.bucket+", or leave it empty for the bucket's root. A folder that does not exist is made.").
				FontSize(12).TextColor(t.TextMuted)
			ui.Field(c, "Destination", func() {
				ui.TextInput(c.Key("dest"), &a.dest).Placeholder("images/2026/").AutoFocus()
			}).Description("The path inside the bucket, ending in a slash.")
			if len(a.movableFolders()) > 0 {
				ui.Row(c).Gap(6).Wrap().Children(func() {
					for _, folder := range a.movableFolders() {
						prefix := folder
						if ui.Button(c.Key(prefix), prefix).Tooltip("Move here").Clicked() {
							a.move(prefix)
						}
					}
				})
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "Cancel").Clicked() {
					a.dlg = dialogNone
				}
				if ui.PrimaryButton(c, "Move").Clicked() {
					a.move(a.dest)
				}
			})
		})
	})
	if !open && a.dlg != dialogNone {
		a.dlg = dialogNone
	}
}

// movableFolders returns the folders of the current listing, which the
// move dialog offers as one-click destinations.
func (a *App) movableFolders() []string {
	var out []string
	for _, e := range a.listing.Items {
		if e.Folder && e.Key != a.prefix {
			out = append(out, e.Key)
		}
	}
	return out
}

/* -------------------------------------------------------------------------
   Connection
   ------------------------------------------------------------------------- */

// connectionDialog draws the form that makes or edits a connection.
func (a *App) connectionDialog(c *ui.Context) {
	t := c.Theme()
	open := true
	title := "New connection"
	if a.conn.editing {
		title = "Edit connection"
	}
	ui.Modal(c, &open, func() {
		ui.Scroll(c).Width(520).MaxHeight(440).Children(func() {
			ui.Text(c, title).FontSize(17).Bold()

			// The presets fill the endpoint and region in, which is what
			// most people need.
			ui.Row(c).Gap(6).Wrap().Children(func() {
				for _, preset := range a.svc.Providers() {
					chosen := a.conn.provider == string(preset.ID)
					label := preset.Label
					button := ui.Button(c.Key(string(preset.ID)), label)
					if chosen {
						button.Background(t.Accent).TextColor(t.AccentText)
					}
					if button.Clicked() {
						a.conn.provider = string(preset.ID)
						a.conn.endpoint = preset.Endpoint
						a.conn.region = preset.Region
						a.conn.pathStyle = preset.PathStyle
						if a.conn.name == "" {
							a.conn.name = preset.Label
						}
					}
				}
			})

			if note := a.providerNote(); note != "" {
				ui.Row(c).Gap(6).AlignItems(ui.Start).Padding(8).Radius(t.Radius).
					Background(t.Accent.Alpha(0.1)).Children(func() {
					ui.Icon(c, iconInfo).FontSize(14).TextColor(t.Accent)
					ui.Text(c, note).FontSize(11.5).Grow(1).TextColor(t.TextMuted)
				})
			}

			ui.Form(c, func() {
				ui.Field(c, "Name", func() {
					ui.TextInput(c.Key("name"), &a.conn.name).Placeholder("Work R2")
				})
				ui.Field(c, "Endpoint", func() {
					ui.TextInput(c.Key("endpoint"), &a.conn.endpoint).Placeholder("https://<account>.r2.cloudflarestorage.com")
				})
				ui.Field(c, "Bucket (optional)", func() {
					ui.TextInput(c.Key("bucket"), &a.conn.bucket).Placeholder("my-bucket")
				}).Description("Required for tokens scoped to a single bucket (e.g. Cloudflare R2).")
				ui.Field(c, "Region", func() {
					ui.TextInput(c.Key("region"), &a.conn.region).Placeholder("auto")
				}).Description("Most providers want auto or us-east-1.")
				ui.Field(c, "Access key ID", func() {
					ui.TextInput(c.Key("access"), &a.conn.accessKey).Placeholder("Access key ID")
				})
				ui.Field(c, "Secret access key", func() {
					ui.TextInput(c.Key("secret"), &a.conn.secretKey).Password().Placeholder(secretPlaceholder(a.conn))
				}).Description(secretDescription(a.conn))
				ui.Toggle(c.Key("pathstyle"), &a.conn.pathStyle, "Use path-style requests").
					Tooltip("Needed by MinIO and some gateways")
			})

			if a.conn.err != "" {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconAlert).FontSize(14).TextColor(t.Danger)
					ui.Text(c, a.conn.err).FontSize(12).Grow(1).TextColor(t.Danger)
				})
			}
			if result := a.conn.result; result != nil {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					if result.OK {
						ui.Icon(c, iconCheck).FontSize(14).TextColor(t.Success)
					} else {
						ui.Icon(c, iconAlert).FontSize(14).TextColor(t.Danger)
					}
					ui.Text(c, result.Message).FontSize(12).Grow(1)
					if result.OK {
						ui.Text(c, fmt.Sprintf("in %d ms", result.Took.Milliseconds())).FontSize(11).TextColor(t.TextMuted)
					}
				})
			}

			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if a.conn.busy {
					ui.Spinner(c).Label("Working")
				}
				ui.Spacer(c)
				if a.conn.editing {
					id := a.conn.id
					if ui.Button(c, "Remove").Tooltip("Forget this connection").Clicked() {
						a.dlg = dialogNone
						for _, p := range a.connections {
							if p.ID == id {
								a.removeConnection(p)
								break
							}
						}
					}
				}
				if ui.Button(c, "Cancel").Clicked() {
					a.dlg = dialogNone
				}
				if ui.Button(c, "Test").Disabled(a.conn.busy || (a.conn.editing && a.conn.secretKey == "")).Tooltip("Check the credentials without saving them").Clicked() {
					a.testConnection()
				}
				save := ui.PrimaryButton(c, "Save and connect").Disabled(a.conn.busy)
				if save.Clicked() || (!a.conn.busy && c.Shortcut(ui.Cmd, ui.KeyEnter)) {
					a.saveConnection()
				}
			})
		})
	})
	if !open && a.dlg != dialogNone {
		a.dlg = dialogNone
	}
}

// providerNote explains the preset the user chose.
func (a *App) providerNote() string {
	for _, preset := range a.svc.Providers() {
		if string(preset.ID) == a.conn.provider {
			return preset.Note
		}
	}
	return ""
}

// secretPlaceholder is what the secret field shows when editing a
// connection that already has one.
func secretPlaceholder(form connForm) string {
	if form.editing {
		return "Leave empty to keep the saved key"
	}
	return "Secret access key"
}

// secretDescription explains what happens to a saved secret.
func secretDescription(form connForm) string {
	if form.editing {
		return "A new key replaces the one in the system's credential store."
	}
	return "Kept in the system's credential store, never on disk."
}

/* -------------------------------------------------------------------------
   Sharing
   ------------------------------------------------------------------------- */

// openShare makes a temporary link to an object and shows it.
func (a *App) openShare(key string) {
	a.dlg = dialogShare
	hours := a.prefs.PresignTTLHours
	a.shareLink(key, hours)
}

// copyLink makes a link and puts it on the clipboard without showing the
// dialog.
func (a *App) copyLink(key string) {
	bucket := a.bucket
	go func() {
		url, err := a.svc.PresignObject(a.ctx, bucket, key, 0)
		a.update(func() {
			if err != nil {
				a.fail(err)
				return
			}
			a.services.WriteClipboard(url)
			a.notice = "Link copied"
		})
	}()
}

// shareDialog shows a temporary link to an object, and the ways to send
// it on.
func (a *App) shareDialog(c *ui.Context) {
	t := c.Theme()
	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(560).Gap(14).Children(func() {
			ui.Text(c, "Share “"+baseName(a.share.key)+"”").FontSize(17).Bold()
			ui.Text(c, "Anyone with this link can read the object until it expires. The link carries the credentials, so share it only with the people you mean to.").
				FontSize(12).TextColor(t.TextMuted)

			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Expires in").FontSize(12).TextColor(t.TextMuted)
				hours := []string{"1 hour", "6 hours", "12 hours", "1 day", "3 days", "7 days"}
				values := []int{1, 6, 12, 24, 72, 168}
				chosen := ttlLabel(a.share.hours)
				if ui.Select(c.Key("ttl"), &chosen, hours).Label("Lifetime").Width(130).Changed() {
					for i, label := range hours {
						if label == chosen {
							a.shareLink(a.share.key, values[i])
							break
						}
					}
				}
				ui.Spacer(c)
				if a.share.busy {
					ui.Spinner(c).Label("Making a link")
				}
			})

			if a.share.err != "" {
				ui.Text(c, a.share.err).FontSize(12).TextColor(t.Danger)
			}

			if a.share.url != "" {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.TextInput(c.Key("url"), &a.share.url).ReadOnly(true).Grow(1).Label("Link")
					if ui.Button(c, "Copy").Clicked() {
						a.services.WriteClipboard(a.share.url)
						a.notice = "Link copied"
					}
					if ui.Button(c, "Open").Tooltip("Open the link in the browser").Clicked() {
						a.services.OpenURL(a.share.url)
					}
				})
			}

			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "Done").Clicked() {
					a.dlg = dialogNone
				}
			})
		})
	})
	if !open && a.dlg != dialogNone {
		a.dlg = dialogNone
	}
}

// ttlLabel names a lifetime in hours the way the chooser shows it.
func ttlLabel(hours int) string {
	switch hours {
	case 1:
		return "1 hour"
	case 6:
		return "6 hours"
	case 12:
		return "12 hours"
	case 24:
		return "1 day"
	case 72:
		return "3 days"
	case 168:
		return "7 days"
	}
	return "1 day"
}

/* -------------------------------------------------------------------------
   Object detail
   ------------------------------------------------------------------------- */

// detailDialog describes an object, and shows a preview when the app has
// one. Anything it cannot draw itself opens in the browser instead.
func (a *App) detailDialog(c *ui.Context) {
	t := c.Theme()
	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(620).MaxHeight(560).Gap(12).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, entryIcon(store.Entry{Key: a.detail.key, Name: baseName(a.detail.key)})).
					FontSize(18).TextColor(t.Accent)
				ui.Text(c, baseName(a.detail.key)).FontSize(17).Bold().Grow(1).SingleLine().Ellipsis("…")
				if a.detail.busy {
					ui.Spinner(c).Label("Loading")
				}
			})

			if a.detail.err != "" {
				ui.Text(c, a.detail.err).FontSize(12).TextColor(t.Danger)
			}

			a.preview(c)

			ui.Column(c).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					a.detailRow(c, "Kind", kindLabel(kindOf(a.detail.key, false), a.detail.key))
					a.detailRow(c, "Size", formatBytes(a.detail.info.Size))
					a.detailRow(c, "Modified", formatDate(a.detail.info.LastModified))
					a.detailRow(c, "Folder", "/"+a.detail.info.Folder)
					if a.detail.info.StorageClass != "" {
						a.detailRow(c, "Storage class", a.detail.info.StorageClass)
					}
					if a.detail.info.ContentType != "" {
						a.detailRow(c, "Content type", a.detail.info.ContentType)
					}
					if a.detail.info.ETag != "" {
						a.detailRow(c, "ETag", strings.Trim(a.detail.info.ETag, `"`))
					}
				})
			}).Grow(1)

			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				if ui.Button(c, "Copy key").Clicked() {
					a.copyText(a.detail.key, "Key copied")
				}
				if ui.Button(c, "Open in browser").Clicked() {
					a.openInBrowser(a.detail.key)
				}
				if ui.Button(c, "Save as…").Clicked() {
					a.saveAs(a.detail.key)
				}
				if ui.PrimaryButton(c, "Done").Clicked() {
					a.dlg = dialogNone
				}
			})
		})
	})
	if !open && a.dlg != dialogNone {
		a.dlg = dialogNone
	}
}

// detailRow draws one line of an object's metadata.
func (a *App) detailRow(c *ui.Context, label, value string) {
	if value == "" || value == "—" {
		return
	}
	t := c.Theme()
	ui.Row(c).Gap(12).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(t.TextMuted).Width(110)
		ui.Text(c, value).FontSize(12).Grow(1).SingleLine().Ellipsis("…")
	})
}

// preview draws what the app can show of an object: its picture, the start
// of its text, or a note saying what to do instead.
func (a *App) preview(c *ui.Context) {
	t := c.Theme()
	switch {
	case a.detail.img != nil:
		ui.Box(c).FillWidth().MaxHeight(280).Radius(t.Radius).Clip().Center().Children(func() {
			ui.Image(c, a.detail.img).Fit(ui.Contain).Grow(1)
		})
	case a.detail.text != nil && a.detail.text.Binary:
		ui.Text(c, "This file does not look like text, so there is nothing to show here.").
			FontSize(12).TextColor(t.TextMuted)
	case a.detail.text != nil:
		text := a.detail.text.Text
		if a.detail.text.Truncated {
			text += "\n\n… the file goes on; the preview stops here."
		}
		ui.Box(c).FillWidth().Height(240).Radius(t.Radius).Background(t.SurfaceHover).Children(func() {
			ui.ScrollBoth(c).Padding(10).Children(func() {
				ui.Text(c, text).FontSize(11.5).Font("monospace")
			})
		})
	case !a.detail.busy && a.detail.err == "":
		ui.Text(c, "This kind of file has no preview. Open it in the browser or save it to look at it.").
			FontSize(12).TextColor(t.TextMuted)
	}
}

// baseName is the last segment of an object key.
func baseName(key string) string {
	if i := strings.LastIndex(strings.TrimSuffix(key, "/"), "/"); i >= 0 {
		return strings.TrimSuffix(key, "/")[i+1:]
	}
	return strings.TrimSuffix(key, "/")
}
