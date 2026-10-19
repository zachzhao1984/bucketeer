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
		a.textDialog(c, "New folder", "Name", "photos", "Create", a.makeFolder)
	case dialogNewBucket:
		a.textDialog(c, "New bucket", "Name", "my-media-bucket", "Create", a.makeBucket)
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
	chosen := false

	ctlDialog(c, &open, 380, func() {
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Start).Children(func() {
				mark := ui.Box(c).Size(34, 34).Radius(10).Center().Shrink(0).
					Background(ui.Transparent).Border(0, ui.Transparent)
				mark.Draw(func(p *ui.Painter, r ui.Rect) {
					p.Fill(r, t.Danger.Alpha(0.12), 10)
				})
				mark.Children(func() {
					ui.Icon(c, iconAlert).FontSize(17).TextColor(t.Danger)
				})
				ui.Column(c).Grow(1).Gap(4).Children(func() {
					ui.Text(c, a.confirm.title).
						FontSize(typeHeadline).
						FontWeight(weightMedium).
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
					chosen = true
				}
			})
		})
	})

	a.confirm.open = open
	if chosen {
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

func (a *App) textDialog(c *ui.Context, title, label, placeholder, action string, accept func(string)) {
	t := c.Theme()
	open := true
	submitted := false

	ctlDialog(c, &open, 400, func() {
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			ui.Text(c, title).
				FontSize(typeHeadline).
				FontWeight(weightMedium).
				TextColor(t.Text)

			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, label).FontSize(typeBody).TextColor(t.TextMuted)
				field := ui.TextInputBase(c.Key("name"), &a.input).
					Placeholder(placeholder).FontSize(typeBody).AutoFocus()
				submitted = field.Submitted()
				ctlField(c, field).FillWidth()
			})

			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
				cancel := ctlButton(c, "Cancel")
				if cancel.Clicked() {
					a.dlg = dialogNone
				}
				done := ctlPrimary(c, action)
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
	a.textDialog(c, "Rename", "New name", "Enter a new name", "Rename", func(name string) {
		a.rename(key, name)
	})
}

func (a *App) moveDialog(c *ui.Context) {
	t := c.Theme()
	open := true

	ctlDialog(c, &open, 440, func() {
		ui.Column(c).FillWidth().Gap(14).Children(func() {
			ui.Text(c, "Move "+plural(a.sel.Len(), "item", "items")).
				FontSize(typeHeadline).
				FontWeight(weightMedium).
				TextColor(t.Text)
			ui.Text(c, "Enter a folder in “"+a.bucket+"”. Leave it empty for the bucket root; folders are created as needed.").
				FontSize(typeBody).
				TextColor(t.TextMuted)

			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "Destination").FontSize(typeBody).TextColor(t.TextMuted)
				field := ui.TextInputBase(c.Key("dest"), &a.dest).
					Placeholder("documents/archives/").FontSize(typeBody).AutoFocus()
				ctlField(c, field).FillWidth()
			})

			if folders := a.movableFolders(); len(folders) > 0 {
				ui.Text(c, "Folders").FontSize(typeCaption).TextColor(t.TextMuted)
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
	title := "Add a connection"
	if a.conn.editing {
		title = "Edit connection"
	}

	labels := a.providerLabels()
	chosen := a.providerLabel()

	ctlDialog(c, &open, 560, func() {
		ui.Column(c).FillWidth().Gap(16).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, title).
					FontSize(typeTitle).
					FontWeight(weightSemi).
					TextColor(t.Text)
				ui.Text(c, "Connect Cloudflare R2 with an access key, or configure any S3-compatible bucket.").
					FontSize(typeBody).
					TextColor(t.TextMuted)
			})

			ui.Scroll(c).MaxHeight(440).Children(func() {
				ui.Column(c).FillWidth().Gap(14).Children(func() {
					ui.Column(c).Gap(6).Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
							ui.Text(c, "Provider").FontSize(typeBody).TextColor(t.Text).Grow(1)
						})
						btn := ctlMenuButton(c, "Provider", chosen, a.providerOpen, 240)
						if btn.Clicked() {
							a.providerOpen = !a.providerOpen
						}
						ui.Popover(c, btn, &a.providerOpen, func() {
							ctlMenuPanel(c, 240).Children(func() {
								ui.Column(c).Gap(1).Children(func() {
									for _, label := range labels {
										l := label
										if menuItem(c, l, "", a.providerLabel() == l, "") {
											a.providerOpen = false
											a.applyProvider(l)
										}
									}
								})
							})
						})
					})

					a.formField(c, "Connection name", "", "Personal R2", &a.conn.name, false)
					a.formField(c, "S3 API endpoint", "from the bucket settings", "https://<account>.r2.cloudflarestorage.com", &a.conn.endpoint, false)

					ui.Row(c).FillWidth().Gap(10).Children(func() {
						ui.Column(c).Grow(1).Children(func() {
							a.formField(c, "Bucket", "", "my-bucket", &a.conn.bucket, false)
						})
						ui.Column(c).Width(120).Children(func() {
							a.formField(c, "Region", "usually auto", "auto", &a.conn.region, false)
						})
					})

					a.formField(c, "Access Key ID", "", "", &a.conn.accessKey, false)
					a.formField(c, "Secret Access Key", secretHint(a.conn), secretPlaceholder(a.conn), &a.conn.secretKey, true)

					ui.Row(c).FillWidth().Gap(9).AlignItems(ui.Center).Children(func() {
						ctlSwitch(c, "pathstyle", &a.conn.pathStyle, "Path-style requests")
						ui.Text(c, "Use path-style requests").FontSize(typeBody).TextColor(t.Text)
					})

					if a.conn.err != "" {
						statusBanner(c, t.Danger.Alpha(0.08), iconAlert, a.conn.err, t.Text, t.Danger)
					}
					if result := a.conn.result; result != nil {
						if result.OK {
							took := fmt.Sprintf("· %d ms", result.Took.Milliseconds())
							statusBanner(c, t.Success.Alpha(0.08), iconCheck,
								result.Message+" "+took, t.Text, t.Success)
						} else {
							statusBanner(c, t.Danger.Alpha(0.08), iconAlert,
								result.Message, t.Text, t.Danger)
						}
					}
				})
			})

			ui.Row(c).FillWidth().Children(func() { rule(c, 0, t.Border) })

			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				if a.conn.busy {
					ui.Spinner(c).Label("Working")
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
				test := ctlButton(c, "Test connection").Disabled(a.conn.busy)
				if test.Clicked() {
					a.testConnection()
				}
				save := ctlPrimary(c, "Save & connect").Disabled(a.conn.busy)
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

// formField is one labelled field of the connection form. The label
// belongs to the field, so a click on it puts the caret in.
func (a *App) formField(c *ui.Context, label, hint, placeholder string, value *string, secret bool) {
	t := c.Theme()
	ui.Column(c).Gap(6).Children(func() {
		if hint != "" {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Text(c, label).FontSize(typeBody).TextColor(t.Text).Grow(1)
				ui.Text(c, hint).FontSize(typeCaption).TextColor(t.TextMuted)
			})
			field := ui.TextInputBase(c.Key(label), value).Placeholder(placeholder).FontSize(typeBody)
			if secret {
				field.Password()
			}
			ctlField(c, field).FillWidth()
			return
		}
		ui.Field(c, label, func() {
			field := ui.TextInputBase(c.Key(label), value).Placeholder(placeholder).FontSize(typeBody)
			if secret {
				field.Password()
			}
			ctlField(c, field).FillWidth()
		})
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

func secretPlaceholder(form connForm) string {
	if form.editing {
		return "Leave empty to keep the saved key"
	}
	return ""
}

func secretHint(form connForm) string {
	if form.editing {
		return "a new key replaces the saved one"
	}
	return "stored in your system keychain"
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

	ctlDialog(c, &open, 540, func() {
		ui.Column(c).FillWidth().Gap(14).Children(func() {
			ui.Text(c, "Share “"+baseName(a.share.key)+"”").
				FontSize(typeHeadline).
				FontWeight(weightMedium).
				TextColor(t.Text)
			ui.Text(c, "A presigned link gives temporary read access straight from the endpoint, with no sign-in.").
				FontSize(typeBody).
				TextColor(t.TextMuted)

			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Expires in").
					FontSize(typeBody).
					TextColor(t.TextMuted).
					Grow(1)

				label := ttlLabel(a.share.hours)
				btn := ctlMenuButton(c, "Expiration", label, a.shareOpen, 130)
				if btn.Clicked() {
					a.shareOpen = !a.shareOpen
				}
				ui.Popover(c, btn, &a.shareOpen, func() {
					ctlMenuPanel(c, 168).Children(func() {
						ui.Column(c).Gap(1).Children(func() {
							for _, hours := range []int{1, 6, 12, 24, 72, 168} {
								h := hours
								if menuItem(c, ttlLabel(h), "", a.share.hours == h, "") {
									a.shareOpen = false
									a.shareLink(a.share.key, h)
								}
							}
						})
					})
				})

				if a.share.busy {
					ui.Spinner(c).Label("Generating link")
				}
			})

			if a.share.err != "" {
				statusBanner(c, t.Danger.Alpha(0.08), iconAlert, a.share.err, t.Text, t.Danger)
			}

			if a.share.url != "" {
				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					link := ui.TextInputBase(c.Key("url"), &a.share.url).ReadOnly(true).FontSize(typeBody)
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
   Preview

   One sheet: the object's name and its actions on a bar, the preview
   itself on a recessed plate, and its facts in a footer. Nothing is
   asked for that the app cannot answer.
   ------------------------------------------------------------------------- */

func (a *App) detailDialog(c *ui.Context) {
	t := c.Theme()
	ch := chromeOf(t)
	open := true
	_, winH := c.Size()

	name := baseName(a.detail.key)

	ui.DialogBase(c, &open, func(back, panel ui.Element) {
		back.Background(ch.scrim)
		winW, _ := c.Size()
		panel.Width(min(760, winW-64)).MaxHeight(winH - 60).Radius(16).Padding(0)
		panel = panel.Background(ui.Transparent).Border(0, ui.Transparent)
		panel.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Shadow(r, 16, 0, 24, 70, 0, ch.floatShadow)
			p.Fill(r, ch.plate, 16)
		})

		ui.Column(c).FillWidth().Gap(0).Children(func() {
			// The title bar of the sheet: the mark, the name, and what
			// can be done to it.
			ui.Row(c).FillWidth().Height(52).Padding(0, 16).Gap(10).
				AlignItems(ui.Center).Children(func() {
				a.cellMark(c, store.Entry{Key: a.detail.key, Name: name}, kindOf(a.detail.key, false), 26)

				ui.Text(c, name).
					FontSize(typeBody).
					FontWeight(weightMedium).
					TextColor(t.Text).
					Grow(1).
					SingleLine().
					Ellipsis("…")

				link := ctlIconButtonBare(c, "Copy link")
				link.Children(func() { ui.Icon(c, iconLink).FontSize(15).TextColor(t.TextMuted) })
				if link.Clicked() {
					a.copyLink(a.detail.key)
				}

				openIn := ctlIconButtonBare(c, "Open in browser")
				openIn.Children(func() { ui.Icon(c, iconExternal).FontSize(15).TextColor(t.TextMuted) })
				if openIn.Clicked() {
					a.openInBrowser(a.detail.key)
				}

				save := ctlIconButtonBare(c, "Download")
				save.Children(func() { ui.Icon(c, iconDownload).FontSize(15).TextColor(t.TextMuted) })
				if save.Clicked() {
					a.chooseOnly(a.detail.key)
					a.saveAs(a.detail.key)
				}

				ui.Box(c).Width(1).Height(20).Shrink(0).Background(t.Border)

				closeBtn := ctlIconButtonBare(c, "Close preview")
				closeBtn.Children(func() {
					ui.Box(c).Size(22, 22).Radius(11).Center().Background(ui.Transparent).
						Children(func() {
							ui.Icon(c, iconClose).FontSize(12).TextColor(t.TextMuted)
						})
				})
				if closeBtn.Clicked() {
					a.dlg = dialogNone
				}
			})

			// The preview itself, on a recessed plate with room around it.
			preview := ui.Box(c).FillWidth().Height(max(240, winH-330)).Margin(0, 16).
				Radius(10).Clip().Center().Background(ui.Transparent).Border(0, ui.Transparent)
			preview.Draw(func(p *ui.Painter, r ui.Rect) {
				p.Fill(r, ch.inset, 10)
			})
			preview.Children(func() {
				a.preview(c)
			})

			// The facts, in one line under it.
			ui.Row(c).FillWidth().Padding(14, 18).Gap(20).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Type").FontSize(typeCaption).TextColor(t.TextMuted)
					ctlChip(c, typeChip(store.Entry{Key: a.detail.key}), t.TextMuted)
				})
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Size").FontSize(typeCaption).TextColor(t.TextMuted)
					ui.Text(c, formatBytes(a.detail.info.Size)).FontSize(typeBody).TextColor(t.Text)
				})
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "Modified").FontSize(typeCaption).TextColor(t.TextMuted)
					ui.Text(c, formatDate(a.detail.info.LastModified)).FontSize(typeBody).TextColor(t.Text)
				})
				ui.Spacer(c)
				ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(1).Children(func() {
					ui.Text(c, "Key").FontSize(typeCaption).TextColor(t.TextMuted)
					ui.Text(c, a.detail.key).
						FontSize(typeCaption).
						TextColor(t.Text).
						SingleLine().
						Ellipsis("…")
					copy := ctlIconButtonBare(c, "Copy key")
					copy.Children(func() { ui.Icon(c, iconCopy).FontSize(12).TextColor(t.TextMuted) })
					if copy.Clicked() {
						a.copyText(a.detail.key, "Object key copied")
					}
				})
			})
		})
	})

	if !open && a.dlg != dialogNone {
		a.dlg = dialogNone
	}
}

// preview shows what the app can draw of the object, and says so plainly
// when it cannot draw anything.
func (a *App) preview(c *ui.Context) {
	t := c.Theme()

	switch {
	case a.detail.busy:
		ui.Column(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c).Label("Loading preview")
		})
	case a.detail.err != "":
		ui.Column(c).Gap(8).Center().Children(func() {
			ui.Icon(c, iconAlert).FontSize(28).TextColor(t.Danger)
			ui.Text(c, a.detail.err).FontSize(typeBody).TextColor(t.TextMuted).TextAlign(ui.Center).MaxWidth(420)
		})
	case a.detail.img != nil:
		ui.Image(c, a.detail.img).Fit(ui.Contain).Fill().Padding(12)
	case a.detail.text != nil && a.detail.text.Binary:
		a.noPreview(c, "This file holds data the app cannot draw.")
	case a.detail.text != nil:
		text := a.detail.text.Text
		if a.detail.text.Truncated {
			text += "\n\n… [preview truncated at 1 MB] …"
		}
		ui.ScrollBoth(c).Fill().Padding(14).Children(func() {
			ui.Text(c, text).FontSize(typeCaption).Font("monospace")
		})
	default:
		a.noPreview(c, "No inline preview for this file type.")
	}
}

// noPreview is the quiet state of a file the app cannot draw: a mark and
// one line saying so.
func (a *App) noPreview(c *ui.Context, message string) {
	t := c.Theme()

	ui.Column(c).Gap(10).AlignItems(ui.Center).Children(func() {
		mark := ui.Box(c).Size(56, 56).Radius(14).Center().Shrink(0).
			Background(ui.Transparent).Border(0, ui.Transparent)
		mark.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(r, chromeOf(t).plate, 14)
			p.Stroke(r, t.Border, 14, 1)
		})
		mark.Children(func() {
			ui.Icon(c, iconOther).FontSize(26).TextColor(t.TextMuted)
		})
		ui.Text(c, message).
			FontSize(typeBody).
			TextColor(t.TextMuted).
			TextAlign(ui.Center).
			MaxWidth(380)
	})
}

func baseName(key string) string {
	trimmed := strings.TrimSuffix(key, "/")
	if i := strings.LastIndex(trimmed, "/"); i >= 0 {
		return trimmed[i+1:]
	}
	return trimmed
}
