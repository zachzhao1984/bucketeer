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
		a.textDialog(c, "New folder", "Folder name", "e.g. photos", a.makeFolder)
	case dialogNewBucket:
		a.textDialog(c, "New bucket", "Bucket name", "e.g. my-media-bucket", a.makeBucket)
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

func (a *App) confirmDialog(c *ui.Context) {
	if !a.confirm.open {
		return
	}
	buttonLabel := a.confirm.button
	if buttonLabel == "" {
		buttonLabel = "Delete"
	}
	switch ui.AlertDialog(c, &a.confirm.open, a.confirm.title, a.confirm.message, "Cancel", buttonLabel) {
	case 1:
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
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(380).Gap(14).Children(func() {
			ui.Text(c, title).FontSize(16).Bold().TextColor(t.Text)
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
}

func (a *App) renameDialog(c *ui.Context) {
	key := a.dest
	a.textDialog(c, "Rename item", "New name", "Enter new name", func(name string) {
		a.rename(key, name)
	})
}

func (a *App) moveDialog(c *ui.Context) {
	t := c.Theme()
	open := true
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(440).Gap(14).Children(func() {
			ui.Text(c, "Move "+plural(a.sel.Len(), "item", "items")).FontSize(16).Bold().TextColor(t.Text)
			ui.Text(c, "Enter target folder prefix in "+a.bucket+", or leave empty for root. Folders will be created as needed.").
				FontSize(12).TextColor(t.TextMuted)
			ui.Field(c, "Destination folder", func() {
				ui.TextInput(c.Key("dest"), &a.dest).Placeholder("documents/archives/").AutoFocus()
			}).Description("Target prefix ending with a slash.")
			if len(a.movableFolders()) > 0 {
				ui.Text(c, "Quick destinations:").FontSize(11.5).Bold().TextColor(t.TextMuted)
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
	title := "New connection"
	if a.conn.editing {
		title = "Edit Connection"
	}

	ui.Modal(c, &open, func() {
		ui.Scroll(c).Width(520).MaxHeight(500).Children(func() {
			ui.Column(c).Gap(14).Padding(6, 4).Children(func() {
				ui.Text(c, title).FontSize(17).Bold().TextColor(t.Text)

				// Provider Presets Selector
				ui.Row(c).Gap(6).Wrap().Children(func() {
					for _, preset := range a.svc.Providers() {
						chosen := a.conn.provider == string(preset.ID)
						label := preset.Label
						btn := ui.Button(c.Key(string(preset.ID)), label)
						if chosen {
							btn.Background(t.Accent).TextColor(t.AccentText)
						}
						if btn.Clicked() {
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
					ui.Row(c).Gap(8).AlignItems(ui.Start).Padding(8, 10).Radius(6).
						Background(t.Accent.Alpha(0.10)).Children(func() {
						ui.Icon(c, iconInfo).FontSize(14).TextColor(t.Accent)
						ui.Text(c, note).FontSize(11.5).Grow(1).TextColor(t.TextMuted)
					})
				}

				// Form Fields
				ui.Form(c, func() {
					ui.Field(c, "Name", func() {
						ui.TextInput(c.Key("name"), &a.conn.name).Placeholder("Cloudflare R2 Production")
					})
					ui.Field(c, "Endpoint URL", func() {
						ui.TextInput(c.Key("endpoint"), &a.conn.endpoint).Placeholder("https://<account-id>.r2.cloudflarestorage.com")
					})
					ui.Field(c, "Bucket name (optional)", func() {
						ui.TextInput(c.Key("bucket"), &a.conn.bucket).Placeholder("my-bucket")
					}).Description("Required for tokens scoped to a single bucket.")
					ui.Field(c, "Region", func() {
						ui.TextInput(c.Key("region"), &a.conn.region).Placeholder("auto")
					}).Description("Use auto for R2, or us-east-1 for AWS/MinIO.")
					ui.Field(c, "Access Key ID", func() {
						ui.TextInput(c.Key("access"), &a.conn.accessKey).Placeholder("Enter access key ID")
					})
					ui.Field(c, "Secret Access Key", func() {
						ui.TextInput(c.Key("secret"), &a.conn.secretKey).Password().Placeholder(secretPlaceholder(a.conn))
					}).Description(secretDescription(a.conn))
					ui.Toggle(c.Key("pathstyle"), &a.conn.pathStyle, "Use path-style requests").
						Tooltip("Required by MinIO and certain S3-compatible gateways")
				})

				// Feedback banners
				if a.conn.err != "" {
					ui.Row(c).Padding(8, 10).Radius(6).Background(t.Danger.Alpha(0.12)).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, iconAlert).FontSize(14).TextColor(t.Danger)
						ui.Text(c, a.conn.err).FontSize(12).Grow(1).TextColor(t.Danger)
					})
				}
				if result := a.conn.result; result != nil {
					bgColor := t.Success.Alpha(0.12)
					iconColor := t.Success
					if !result.OK {
						bgColor = t.Danger.Alpha(0.12)
						iconColor = t.Danger
					}
					ui.Row(c).Padding(8, 10).Radius(6).Background(bgColor).Gap(8).AlignItems(ui.Center).Children(func() {
						glyph := iconCheck
						if !result.OK {
							glyph = iconAlert
						}
						ui.Icon(c, glyph).FontSize(14).TextColor(iconColor)
						ui.Text(c, result.Message).FontSize(12).Grow(1)
						if result.OK {
							ui.Text(c, fmt.Sprintf("%d ms", result.Took.Milliseconds())).FontSize(11).TextColor(t.TextMuted)
						}
					})
				}

				// Action buttons
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					if a.conn.busy {
						ui.Spinner(c).Label("Testing")
					}
					ui.Spacer(c)
					if a.conn.editing {
						id := a.conn.id
						delBtn := ui.Button(c, "").Label("Remove")
						delBtn.Children(func() { ui.Text(c, "Remove").TextColor(t.Danger) })
						if delBtn.Clicked() {
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
					testBtn := ui.Button(c, "Test connection").Disabled(a.conn.busy)
					if testBtn.Clicked() {
						a.testConnection()
					}
					saveBtn := ui.PrimaryButton(c, "Save & Connect").Disabled(a.conn.busy)
					if saveBtn.Clicked() || (!a.conn.busy && c.Shortcut(ui.Cmd, ui.KeyEnter)) {
						a.saveConnection()
					}
				})
			})
		})
	})
	if !open && a.dlg != dialogNone {
		a.dlg = dialogNone
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
		return "Leave empty to retain saved secret key"
	}
	return "Enter secret access key"
}

func secretDescription(form connForm) string {
	if form.editing {
		return "New key will replace the existing one in system Keychain."
	}
	return "Saved in system Keychain with hardware encryption."
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
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(520).Gap(14).Children(func() {
			ui.Text(c, "Share “"+baseName(a.share.key)+"”").FontSize(17).Bold().TextColor(t.Text)
			ui.Text(c, "Presigned URLs grant temporary read access directly from the S3 endpoint without requiring account sign-in.").
				FontSize(12).TextColor(t.TextMuted)

			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Link expires in:").FontSize(12).TextColor(t.TextMuted)
				hours := []string{"1 hour", "6 hours", "12 hours", "1 day", "3 days", "7 days"}
				values := []int{1, 6, 12, 24, 72, 168}
				chosen := ttlLabel(a.share.hours)
				if ui.Select(c.Key("ttl"), &chosen, hours).Label("Expiration").Width(130).Changed() {
					for i, label := range hours {
						if label == chosen {
							a.shareLink(a.share.key, values[i])
							break
						}
					}
				}
				ui.Spacer(c)
				if a.share.busy {
					ui.Spinner(c).Label("Generating link")
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
						a.notice = "Share link copied"
					}
					if ui.Button(c, "Open in browser").Clicked() {
						a.services.OpenURL(a.share.url)
					}
				})
			}

			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
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
	ui.Modal(c, &open, func() {
		ui.Column(c).Width(600).MaxHeight(560).Gap(12).Children(func() {
			// Title row
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, entryIcon(store.Entry{Key: a.detail.key, Name: baseName(a.detail.key)})).
					FontSize(20).TextColor(t.Accent)
				ui.Text(c, baseName(a.detail.key)).FontSize(17).Bold().Grow(1).SingleLine().Ellipsis("…")
				if a.detail.busy {
					ui.Spinner(c).Label("Loading")
				}
			})

			if a.detail.err != "" {
				ui.Text(c, a.detail.err).FontSize(12).TextColor(t.Danger)
			}

			a.preview(c)

			// Metadata List
			ui.Column(c).
				Padding(10, 12).
				Radius(8).
				Background(t.SurfaceHover).
				Gap(6).
				Children(func() {
					a.detailRow(c, "Kind", kindLabel(kindOf(a.detail.key, false), a.detail.key))
					a.detailRow(c, "Size", formatBytes(a.detail.info.Size))
					a.detailRow(c, "Modified", formatDate(a.detail.info.LastModified))
					a.detailRow(c, "Key path", a.detail.key)
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

			// Footer Actions
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Spacer(c)
				if ui.Button(c, "Copy key").Clicked() {
					a.copyText(a.detail.key, "Object key copied")
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

func (a *App) detailRow(c *ui.Context, label, value string) {
	if value == "" || value == "—" {
		return
	}
	t := c.Theme()
	ui.Row(c).Gap(12).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(t.TextMuted).Width(100)
		ui.Text(c, value).FontSize(12).Grow(1).SingleLine().Ellipsis("…")
	})
}

func (a *App) preview(c *ui.Context) {
	t := c.Theme()
	switch {
	case a.detail.img != nil:
		ui.Box(c).FillWidth().MaxHeight(260).Radius(8).Clip().Center().Children(func() {
			ui.Image(c, a.detail.img).Fit(ui.Contain).Grow(1)
		})
	case a.detail.text != nil && a.detail.text.Binary:
		ui.Box(c).FillWidth().Padding(14).Radius(8).Background(t.SurfaceHover).Center().Children(func() {
			ui.Text(c, "Binary file content cannot be previewed directly.").FontSize(12).TextColor(t.TextMuted)
		})
	case a.detail.text != nil:
		text := a.detail.text.Text
		if a.detail.text.Truncated {
			text += "\n\n… [preview truncated at 1 MB] …"
		}
		ui.Box(c).FillWidth().Height(220).Radius(8).Background(t.SurfaceHover).Children(func() {
			ui.ScrollBoth(c).Padding(10).Children(func() {
				ui.Text(c, text).FontSize(11.5).Font("monospace")
			})
		})
	case !a.detail.busy && a.detail.err == "":
		ui.Box(c).FillWidth().Padding(14).Radius(8).Background(t.SurfaceHover).Center().Children(func() {
			ui.Text(c, "Preview unavailable for this format. Use Open in browser or Save as.").
				FontSize(12).TextColor(t.TextMuted)
		})
	}
}

func baseName(key string) string {
	if i := strings.LastIndex(strings.TrimSuffix(key, "/"), "/"); i >= 0 {
		return strings.TrimSuffix(key, "/")[i+1:]
	}
	return strings.TrimSuffix(key, "/")
}
