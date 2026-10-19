package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
)

// dialogs draws the modal the app has open, if any.
func (a *App) dialogs(c *ui.Context) {
	switch a.dlg {
	case dialogConnection:
		a.connectionDialog(c)
	case dialogNewFolder:
		a.textDialog(c, "New Folder", "Folder name", "photos", a.makeFolder)
	case dialogNewBucket:
		a.textDialog(c, "New Bucket", "Bucket name", "my-media-bucket", a.makeBucket)
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

/* -------------------------------------------------------------------------
   Confirmation
   ------------------------------------------------------------------------- */

func (a *App) confirmDialog(c *ui.Context) {
	if !a.confirm.open {
		return
	}
	t := c.Theme()
	buttonLabel := a.confirm.button
	if buttonLabel == "" {
		buttonLabel = "Delete"
	}

	open := a.confirm.open
	chosen := -1

	ctlDialog(c, &open, 360, func() {
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Start).Children(func() {
				ui.Icon(c, iconAlert).FontSize(20).TextColor(t.Danger).Shrink(0).Margin(0, 2)
				ui.Column(c).Grow(1).Gap(4).Children(func() {
					ui.Text(c, a.confirm.title).
						FontSize(typeHeadline).
						FontWeight(weightSemi).
						TextColor(t.Text)
					ui.Text(c, a.confirm.message).
						FontSize(typeBody).
						TextColor(t.TextMuted)
				})
			})
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				cancel := ctlButton(c, "Cancel")
				if cancel.Clicked() {
					a.confirm.open = false
				}
				remove := ctlDanger(c, buttonLabel)
				if remove.Clicked() {
					chosen = 1
				}
			})
		})
	})

	a.confirm.open = open
	if chosen == 1 {
		a.confirm.open = false
		action := a.confirm.action
		a.confirm.action = nil
		if action != nil {
			action()
		}
	}
}

/* -------------------------------------------------------------------------
   Single input dialogs
   ------------------------------------------------------------------------- */

func (a *App) openNewFolder() {
	a.input = ""
	a.dlg = dialogNewFolder
}

func (a *App) openNewBucket() {
	a.input = ""
	a.dlg = dialogNewBucket
}

func (a *App) openRename(e store.Entry) {
	a.input = e.Name
	a.dlg = dialogRename
	a.dest = e.Key
}

func (a *App) openMove() {
	a.dest = ""
	a.dlg = dialogMove
}

func (a *App) textDialog(c *ui.Context, title, label, placeholder string, accept func(string)) {
	t := c.Theme()
	open := true
	submitted := false

	ctlDialog(c, &open, 380, func() {
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			ui.Text(c, title).
				FontSize(typeHeadline).
				FontWeight(weightSemi).
				TextColor(t.Text)

			ui.Field(c, label, func() {
				field := ui.TextInput(c.Key("name"), &a.input).Placeholder(placeholder).AutoFocus()
				submitted = field.Submitted()
				ctlField(c, field).FillWidth()
			})

			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				cancel := ctlButton(c, "Cancel")
				if cancel.Clicked() {
					a.dlg = dialogNone
				}
				done := ctlPrimary(c, "Create")
				if submitted || done.Clicked() {
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
}

func (a *App) renameDialog(c *ui.Context) {
	key := a.dest
	a.textDialog(c, "Rename", "New name", "Enter new name", func(name string) {
		a.rename(key, name)
	})
}

func (a *App) moveDialog(c *ui.Context) {
	t := c.Theme()
	open := true

	ctlDialog(c, &open, 440, func() {
		ui.Column(c).FillWidth().Gap(14).Children(func() {
			ui.Text(c, "Move "+plural(a.sel.Len(), "Item", "Item")).
				FontSize(typeHeadline).
				FontWeight(weightSemi).
				TextColor(t.Text)
			ui.Text(c, "Enter a folder in "+a.bucket+". Leave it empty for the bucket root; folders are created as needed.").
				FontSize(typeBody).
				TextColor(t.TextMuted)

			ui.Field(c, "Destination", func() {
				field := ui.TextInput(c.Key("dest"), &a.dest).Placeholder("documents/archives/").AutoFocus()
				ctlField(c, field).FillWidth()
			})

			if folders := a.movableFolders(); len(folders) > 0 {
				menuHeading(c, "Folders")
				ui.Column(c).FillWidth().Gap(1).Children(func() {
					for _, folder := range folders {
						prefix := folder
						pick := ctlButton(c, prefix).Grow(1)
						pick = pick.Justify(ui.Start)
						if pick.Clicked() {
							a.move(prefix)
						}
					}
				})
			}

			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				cancel := ctlButton(c, "Cancel")
				if cancel.Clicked() {
					a.dlg = dialogNone
				}
				move := ctlPrimary(c, "Move")
				if move.Clicked() {
					a.move(a.dest)
				}
			})
		})
	})

	if !open && a.dlg != dialogNone {
		a.dlg = dialogNone
	}
}

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
   Connection Dialog
   ------------------------------------------------------------------------- */

func (a *App) connectionDialog(c *ui.Context) {
	t := c.Theme()
	open := true
	title := "New Connection"
	if a.conn.editing {
		title = "Edit Connection"
	}

	labels := a.providerLabels()
	chosen := a.providerLabel()

	ctlDialog(c, &open, 520, func() {
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			ui.Text(c, title).
				FontSize(typeHeadline).
				FontWeight(weightSemi).
				TextColor(t.Text)

			ui.Scroll(c).MaxHeight(430).Children(func() {
				ui.Column(c).FillWidth().Gap(14).Children(func() {
					// The service, as a pop-up rather than a shelf of
					// seven keys: it is one choice out of a list, and
					// the list is longer than the shelf.
					ui.Form(c, func() {
						ui.Field(c, "Service", func() {
							sel := ctlSelect(c, "provider", &chosen, labels, 220)
							sel.Label("Service")
							if sel.Changed() {
								a.applyProvider(chosen)
							}
						})

						ui.Field(c, "Name", func() {
							ctlField(c, ui.TextInput(c.Key("name"), &a.conn.name).
								Placeholder("Cloudflare R2 Production")).FillWidth()
						})
						ui.Field(c, "Endpoint URL", func() {
							ctlField(c, ui.TextInput(c.Key("endpoint"), &a.conn.endpoint).
								Placeholder("https://<account-id>.r2.cloudflarestorage.com")).FillWidth()
						})
						ui.Field(c, "Bucket", func() {
							ctlField(c, ui.TextInput(c.Key("bucket"), &a.conn.bucket).
								Placeholder("All buckets the key can reach")).FillWidth()
						}).Description("Only needed for a key scoped to one bucket.")
						ui.Field(c, "Region", func() {
							ctlField(c, ui.TextInput(c.Key("region"), &a.conn.region).
								Placeholder("auto")).FillWidth()
						}).Description("Use auto for R2, or us-east-1 for AWS and MinIO.")
						ui.Field(c, "Access Key ID", func() {
							ctlField(c, ui.TextInput(c.Key("access"), &a.conn.accessKey).
								Placeholder("Access key ID")).FillWidth()
						})
						ui.Field(c, "Secret Access Key", func() {
							ctlField(c, ui.TextInput(c.Key("secret"), &a.conn.secretKey).Password().
								Placeholder(secretPlaceholder(a.conn))).FillWidth()
						}).Description(secretDescription(a.conn))
					})

					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ctlSwitch(c, "pathstyle", &a.conn.pathStyle, "").
							Tooltip("Required by MinIO and some S3-compatible gateways")
						ui.Text(c, "Use path-style requests").FontSize(typeBody)
					})

					if a.conn.err != "" {
						statusBanner(c, t.Danger.Alpha(0.09), iconAlert, a.conn.err, t.Text, t.Danger)
					}
					if result := a.conn.result; result != nil {
						if result.OK {
							took := fmt.Sprintf("· %d ms", result.Took.Milliseconds())
							statusBanner(c, t.Success.Alpha(0.09), iconCheck,
								result.Message+" "+took, t.Text, t.Success)
						} else {
							statusBanner(c, t.Danger.Alpha(0.09), iconAlert,
								result.Message, t.Text, t.Danger)
						}
					}
				})
			})

			ui.Row(c).FillWidth().Padding(0, 2).Children(func() {
				rule(c, 0, chromeOf(t).hairline)
			})

			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				if a.conn.busy {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Spinner(c).Label("Working")
					})
				}
				ui.Spacer(c)

				if a.conn.editing {
					remove := ctlButton(c, "Remove")
					if remove.Clicked() {
						id := a.conn.id
						a.dlg = dialogNone
						for _, p := range a.connections {
							if p.ID == id {
								a.removeConnection(p)
								break
							}
						}
					}
				}
				cancel := ctlButton(c, "Cancel")
				if cancel.Clicked() {
					a.dlg = dialogNone
				}
				test := ctlButton(c, "Test Connection").Disabled(a.conn.busy)
				if test.Clicked() {
					a.testConnection()
				}
				save := ctlPrimary(c, "Save & Connect").Disabled(a.conn.busy)
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

// statusBanner is the one-line result of trying something.
func statusBanner(c *ui.Context, wash ui.Color, glyph *ui.SVG, message string, textColor, glyphColor ui.Color) {
	row := ui.Row(c).
		FillWidth().
		Padding(7, 10).
		Radius(btnRadius).
		Gap(8).
		AlignItems(ui.Start).
		Background(wash).
		Border(0, ui.Transparent)
	row.Children(func() {
		ui.Icon(c, glyph).FontSize(14).TextColor(glyphColor).Shrink(0).Margin(0, 1)
		ui.Text(c, message).FontSize(typeCaption).Grow(1).TextColor(textColor)
	})
}

// providerLabels lists the services a new connection can be to.
func (a *App) providerLabels() []string {
	providers := a.svc.Providers()
	out := make([]string, 0, len(providers))
	for _, p := range providers {
		out = append(out, p.Label)
	}
	return out
}

// providerLabel is the service the form is filled in for.
func (a *App) providerLabel() string {
	for _, p := range a.svc.Providers() {
		if string(p.ID) == a.conn.provider {
			return p.Label
		}
	}
	if len(a.svc.Providers()) > 0 {
		return a.svc.Providers()[0].Label
	}
	return ""
}

// applyProvider fills the form from a service's own defaults.
func (a *App) applyProvider(label string) {
	for _, preset := range a.svc.Providers() {
		if preset.Label != label {
			continue
		}
		a.conn.provider = string(preset.ID)
		a.conn.endpoint = preset.Endpoint
		a.conn.region = preset.Region
		a.conn.pathStyle = preset.PathStyle
		if a.conn.name == "" {
			a.conn.name = preset.Label
		}
		return
	}
}

func (a *App) providerNote() string {
	for _, preset := range a.svc.Providers() {
		if string(preset.ID) == a.conn.provider {
			return preset.Note
		}
	}
	return ""
}

func secretPlaceholder(form connForm) string {
	if form.editing {
		return "Leave empty to keep the saved key"
	}
	return "Secret access key"
}

func secretDescription(form connForm) string {
	if form.editing {
		return "A new key replaces the saved one in the Keychain."
	}
	return "Stored in the System Keychain, encrypted by hardware."
}

/* -------------------------------------------------------------------------
   Sharing Dialog
   ------------------------------------------------------------------------- */

func (a *App) openShare(key string) {
	a.dlg = dialogShare
	hours := a.prefs.PresignTTLHours
	a.shareLink(key, hours)
}

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
			a.notice = "Presigned link copied to clipboard"
		})
	}()
}

func (a *App) shareDialog(c *ui.Context) {
	t := c.Theme()
	open := true

	ctlDialog(c, &open, 520, func() {
		ui.Column(c).FillWidth().Gap(14).Children(func() {
			ui.Text(c, "Share “"+baseName(a.share.key)+"”").
				FontSize(typeHeadline).
				FontWeight(weightSemi).
				TextColor(t.Text)
			ui.Text(c, "A presigned link gives temporary read access straight from the S3 endpoint, with no sign-in.").
				FontSize(typeBody).
				TextColor(t.TextMuted)

			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Expires In").
					FontSize(typeBody).
					TextColor(t.TextMuted).
					Grow(1)

				hours := []string{"1 hour", "6 hours", "12 hours", "1 day", "3 days", "7 days"}
				values := []int{1, 6, 12, 24, 72, 168}
				current := ttlLabel(a.share.hours)
				sel := ctlSelect(c, "ttl", &current, hours, 124)
				sel.Label("Expiration")
				if sel.Changed() {
					for i, label := range hours {
						if label == current {
							a.shareLink(a.share.key, values[i])
							break
						}
					}
				}

				if a.share.busy {
					ui.Spinner(c).Label("Generating link")
				}
			})

			if a.share.err != "" {
				statusBanner(c, t.Danger.Alpha(0.09), iconAlert, a.share.err, t.Text, t.Danger)
			}

			if a.share.url != "" {
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					link := ui.TextInput(c.Key("url"), &a.share.url).ReadOnly(true).Label("Link")
					ctlField(c, link).Grow(1)

					copy := ctlButton(c, "Copy")
					if copy.Clicked() {
						a.services.WriteClipboard(a.share.url)
						a.notice = "Share link copied"
					}
					open := ctlButton(c, "Open")
					if open.Clicked() {
						a.services.OpenURL(a.share.url)
					}
				})
			}

			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				done := ctlPrimary(c, "Done")
				if done.Clicked() {
					a.dlg = dialogNone
				}
			})
		})
	})

	if !open && a.dlg != dialogNone {
		a.dlg = dialogNone
	}
}

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
   Object Detail Dialog
   ------------------------------------------------------------------------- */

func (a *App) detailDialog(c *ui.Context) {
	t := c.Theme()
	open := true

	ctlDialog(c, &open, 580, func() {
		ui.Column(c).MaxHeight(560).FillWidth().Gap(14).Children(func() {
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, entryIcon(store.Entry{Key: a.detail.key, Name: baseName(a.detail.key)})).
					FontSize(19).
					TextColor(kindTint(t, kindOf(a.detail.key, false)))
				ui.Text(c, baseName(a.detail.key)).
					FontSize(typeHeadline).
					FontWeight(weightSemi).
					Grow(1).
					SingleLine().
					Ellipsis("…")
				if a.detail.busy {
					ui.Spinner(c).Label("Loading")
				}
			})

			if a.detail.err != "" {
				statusBanner(c, t.Danger.Alpha(0.09), iconAlert, a.detail.err, t.Text, t.Danger)
			}

			a.preview(c)

			ui.Column(c).FillWidth().Gap(5).Children(func() {
				a.detailRow(c, "Kind", kindLabel(kindOf(a.detail.key, false), a.detail.key))
				a.detailRow(c, "Size", formatBytes(a.detail.info.Size))
				a.detailRow(c, "Modified", formatDate(a.detail.info.LastModified))
				a.detailRow(c, "Storage Class", a.detail.info.StorageClass)
				a.detailRow(c, "Content Type", a.detail.info.ContentType)
				a.detailRow(c, "ETag", strings.Trim(a.detail.info.ETag, `"`))
				a.detailRow(c, "Key", a.detail.key)
			})

			ui.Row(c).FillWidth().Padding(0, 2).Children(func() {
				rule(c, 0, chromeOf(t).hairline)
			})

			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				copy := ctlButton(c, "Copy Key")
				if copy.Clicked() {
					a.copyText(a.detail.key, "Object key copied")
				}
				browse := ctlButton(c, "Open in Browser")
				if browse.Clicked() {
					a.openInBrowser(a.detail.key)
				}
				saveAs := ctlButton(c, "Save As…")
				if saveAs.Clicked() {
					a.saveAs(a.detail.key)
				}
				done := ctlPrimary(c, "Done")
				if done.Clicked() {
					a.dlg = dialogNone
				}
			})
		})
	})

	if !open && a.dlg != dialogNone {
		a.dlg = dialogNone
	}
}

// detailRow is one fact about the object, its name set to a fixed width
// so the values line up under each other.
func (a *App) detailRow(c *ui.Context, label, value string) {
	if value == "" || value == "—" {
		return
	}
	t := c.Theme()
	ui.Row(c).FillWidth().Gap(14).Children(func() {
		ui.Text(c, label).
			Width(110).
			FontSize(typeCaption).
			TextColor(t.TextMuted).
			SingleLine()
		ui.Text(c, value).FontSize(typeBody).Grow(1).SingleLine().Ellipsis("…")
	})
}

// preview shows what the app can draw of the object, and says so
// plainly when it cannot draw anything.
func (a *App) preview(c *ui.Context) {
	t := c.Theme()
	ch := chromeOf(t)

	screen := func(height float32) ui.Element {
		box := ui.Box(c).FillWidth().Height(height).Radius(btnRadius).Clip().Center().
			Background(ui.Transparent).Border(0, ui.Transparent)
		box.Draw(func(p *ui.Painter, r ui.Rect) {
			paintSurface(p, r, btnRadius, ch.inset, ui.Transparent)
		})
		return box
	}

	switch {
	case a.detail.img != nil:
		screen(240).Children(func() {
			ui.Image(c, a.detail.img).Fit(ui.Contain).Grow(1)
		})
	case a.detail.text != nil && a.detail.text.Binary:
		screen(64).Children(func() {
			ui.Text(c, "This file holds data the app cannot draw.").
				FontSize(typeBody).
				TextColor(t.TextMuted)
		})
	case a.detail.text != nil:
		text := a.detail.text.Text
		if a.detail.text.Truncated {
			text += "\n\n… [preview truncated at 1 MB] …"
		}
		screen(210).Children(func() {
			ui.ScrollBoth(c).Padding(10).Children(func() {
				ui.Text(c, text).FontSize(typeCaption).Font("monospace")
			})
		})
	case !a.detail.busy && a.detail.err == "":
		screen(64).Children(func() {
			ui.Text(c, "No preview for this format. Open it in the browser, or save it.").
				FontSize(typeBody).
				TextColor(t.TextMuted)
		})
	}
}

func baseName(key string) string {
	trimmed := strings.TrimSuffix(key, "/")
	if i := strings.LastIndex(trimmed, "/"); i >= 0 {
		return trimmed[i+1:]
	}
	return trimmed
}
