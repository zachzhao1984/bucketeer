package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
)

func panel(c *ui.Context, title string, body func()) {
	t := c.Theme()
	ui.Column(c).
		FillWidth().
		Padding(16).
		Gap(12).
		Border(1, t.Border).
		Radius(t.Radius).
		Background(t.Surface).
		Children(func() {
			ui.Text(c, title).Bold().FontSize(14).TextColor(t.Text)
			body()
		})
}

/* -------------------------------------------------------------------------
   Dashboard
   ------------------------------------------------------------------------- */

func (a *App) dashboard(c *ui.Context) {
	t := c.Theme()
	if a.bucket == "" {
		emptyState(c, iconDashboard, "Select a storage bucket",
			"Open a bucket from All Files to inspect its storage breakdown and analytics.", func() {
				if ui.PrimaryButton(c, "Browse buckets").Clicked() {
					a.setSection(sectionFiles)
					a.loadBuckets()
				}
			})
		return
	}

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().Padding(20).Gap(18).Children(func() {
			// Bucket Header
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(36, 36).Radius(8).Background(kindSoftBg(t, kindFolder)).Center().Children(func() {
					ui.Icon(c, iconBucket).FontSize(20).TextColor(t.Accent)
				})
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, a.bucket).FontSize(20).Bold().TextColor(t.Text)
					ui.Text(c, "S3 Storage Analytics").FontSize(11.5).TextColor(t.TextMuted)
				})

				measureBtn := ui.Button(c, "Re-measure").Disabled(a.statLoading)
				if measureBtn.Clicked() {
					a.loadStat()
				}
			})

			if a.stat == nil {
				ui.Box(c).Padding(32).Center().Children(func() {
					ui.Text(c, "Click Measure bucket above to scan storage distribution.").TextColor(t.TextMuted)
				})
				return
			}

			s := a.stat

			// Hero Metric Cards
			ui.Row(c).Gap(14).Children(func() {
				avgSize := "—"
				if s.Objects > 0 {
					avgSize = formatBytes(s.Bytes / s.Objects)
				}
				metrics := []struct {
					label string
					value string
					sub   string
				}{
					{"Total storage", formatBytes(s.Bytes), plural(int(s.Objects), "object", "objects")},
					{"Objects count", formatCount(int(s.Objects)), plural(int(s.Folders), "folder", "folders")},
					{"Average object size", avgSize, "across all keys"},
				}

				for _, m := range metrics {
					ui.Column(c).
						Grow(1).
						Padding(14, 16).
						Radius(8).
						Border(1, t.Border).
						Background(t.Surface).
						Gap(6).
						Children(func() {
							ui.Text(c, m.label).FontSize(11.5).Bold().TextColor(t.TextMuted)
							ui.Text(c, m.value).FontSize(22).Bold().TextColor(t.Text)
							ui.Text(c, m.sub).FontSize(11).TextColor(t.TextMuted)
						})
				}
			})

			if s.Partial {
				ui.Row(c).Padding(10, 14).Radius(8).Background(t.Warning.Alpha(0.12)).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconAlert).FontSize(15).TextColor(t.Warning)
					ui.Text(c, "Scan capped at 50,000 objects. Totals represent a lower bound.").FontSize(12).TextColor(t.Warning)
				})
			}

			// Where the space goes (Top Folders)
			if len(s.TopFolders) > 0 {
				panel(c, "Where the space goes", func() {
					var largest int64 = 1
					for _, f := range s.TopFolders {
						largest = max(largest, f.Bytes)
					}
					ui.Column(c).Gap(10).Children(func() {
						for _, f := range s.TopFolders {
							folder := f
							pct := float64(folder.Bytes) / float64(max(s.Bytes, 1)) * 100
							ui.Row(c.Key(folder.Prefix)).Gap(12).AlignItems(ui.Center).Children(func() {
								ui.Icon(c, iconFolder).FontSize(16).TextColor(kindTint(t, kindFolder))
								ui.Text(c, folder.Name).Width(150).FontSize(12.5).Bold().SingleLine().Ellipsis("…")
								ui.Progress(c, float64(folder.Bytes)/float64(largest)).Grow(1)
								ui.Text(c, fmt.Sprintf("%.1f%%", pct)).Width(46).FontSize(11).TextColor(t.TextMuted)
								ui.Text(c, formatBytes(folder.Bytes)).Width(80).FontSize(12).Bold().TextAlign(ui.End)
							})
						}
					})
				})
			}

			// Storage classes
			if len(s.ByClass) > 0 {
				panel(c, "Storage classes distribution", func() {
					ui.Column(c).Gap(8).Children(func() {
						for _, cl := range s.ByClass {
							class := cl
							ui.Row(c.Key(class.Class)).Padding(6, 10).Radius(6).Background(t.SurfaceHover).Gap(12).AlignItems(ui.Center).Children(func() {
								ui.Badge(c, class.Class)
								ui.Text(c, plural(int(class.Count), "object", "objects")).FontSize(12).TextColor(t.TextMuted).Grow(1)
								ui.Text(c, formatBytes(class.Bytes)).FontSize(12.5).Bold()
							})
						}
					})
				})
			}

			// Largest objects
			if len(s.Largest) > 0 {
				panel(c, "Largest objects", func() {
					ui.Column(c).Gap(6).Children(func() {
						for _, entry := range s.Largest {
							e := entry
							ui.Row(c.Key(e.Key)).
								Padding(6, 8).
								Radius(6).
								Gap(10).
								AlignItems(ui.Center).
								Children(func() {
									ui.Icon(c, entryIcon(e)).FontSize(15).TextColor(kindTint(t, kindOf(e.Key, false)))
									btn := ui.Button(c, e.Name).Grow(1)
									btn.OnClick(func() { a.showDetail(e.Key) })
									ui.Text(c, formatBytes(e.Size)).FontSize(12).TextColor(t.TextMuted)
								})
						}
					})
				})
			}
		})
	})
}

/* -------------------------------------------------------------------------
   Settings
   ------------------------------------------------------------------------- */

func (a *App) settings(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().Padding(20).Gap(18).Children(func() {
			ui.Text(c, "Settings").FontSize(22).Bold().TextColor(t.Text)

			// Preferences Card
			panel(c, "Browsing preferences", func() {
				ui.Column(c).Gap(14).Children(func() {
					// Row 1: Thumbnails
					ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
						ui.Column(c).Grow(1).Gap(2).Children(func() {
							ui.Text(c, "Show image thumbnails").FontSize(13).Bold()
							ui.Text(c, "Decode and display previews for images in grid cards.").FontSize(11.5).TextColor(t.TextMuted)
						})
						ui.Checkbox(c.Key("thumbnails"), &a.prefs.ShowThumbnails, "").OnChange(func() {
							a.persistSettings()
							if a.prefs.ShowThumbnails {
								a.loadThumbnails(a.source().Items)
							}
						})
					})

					ui.Divider(c)

					// Row 2: Confirm delete
					ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
						ui.Column(c).Grow(1).Gap(2).Children(func() {
							ui.Text(c, "Confirm before deleting").FontSize(13).Bold()
							ui.Text(c, "Display confirmation alert before permanently deleting objects.").FontSize(11.5).TextColor(t.TextMuted)
						})
						ui.Checkbox(c.Key("confirm-delete"), &a.prefs.ConfirmDelete, "").OnChange(a.persistSettings)
					})

					ui.Divider(c)

					// Row 3: Presign TTL
					ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
						ui.Column(c).Grow(1).Gap(2).Children(func() {
							ui.Text(c, "Default share link expiration").FontSize(13).Bold()
							ui.Text(c, "How long presigned share links remain accessible.").FontSize(11.5).TextColor(t.TextMuted)
						})
						ttl := ttlLabel(a.prefs.PresignTTLHours)
						ui.Select(c.Key("lifetime"), &ttl, []string{"1 hour", "6 hours", "12 hours", "1 day", "3 days", "7 days"}).
							Width(130).
							OnChange(func() {
								a.prefs.PresignTTLHours = map[string]int{
									"1 hour": 1, "6 hours": 6, "12 hours": 12,
									"1 day": 24, "3 days": 72, "7 days": 168,
								}[ttl]
								a.persistSettings()
							})
					})
				})
			})

			// Saved Connections Card
			panel(c, "Storage connections", func() {
				ui.Column(c).Gap(10).Children(func() {
					if len(a.connections) == 0 {
						ui.Text(c, "No saved connections yet.").TextColor(t.TextMuted).Padding(6, 0)
					}
					for _, p := range a.connections {
						profile := p
						ui.Row(c.Key(p.ID)).
							Padding(10, 14).
							Radius(8).
							Border(1, t.Border).
							Background(t.SurfaceHover).
							Gap(12).
							AlignItems(ui.Center).
							Children(func() {
								ui.Avatar(c, profile.Name, nil).FontSize(11)
								ui.Column(c).Grow(1).Gap(2).Children(func() {
									ui.Text(c, profile.Name).Bold().FontSize(13)
									detail := profile.Host()
									if profile.Bucket != "" {
										detail += " · bucket: " + profile.Bucket
									}
									ui.Text(c, detail).FontSize(11).TextColor(t.TextMuted)
								})

								connBtn := ui.Button(c, "Connect")
								connBtn.OnClick(func() { a.connect(profile.ID) })

								editBtn := ui.Button(c, "Edit")
								editBtn.OnClick(func() { a.openConnection(&profile) })

								removeBtn := ui.Button(c, "Remove")
								removeBtn.OnClick(func() {
									a.ask("Remove “"+profile.Name+"”?",
										"This removes the saved connection profile and deletes its credentials from Keychain.",
										"Remove", func() { a.removeConnection(profile) })
								})
							})
					}

					newConn := ui.PrimaryButton(c, "Add connection")
					newConn.OnClick(func() { a.openConnection(nil) })
				})
			})

			// About App Card
			panel(c, "About Bucketeer", func() {
				ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(48, 48).Radius(12).Background(t.SurfaceHover).Border(1, t.Border).Center().Children(func() {
						ui.Icon(c, iconCloud).FontSize(24).TextColor(t.Accent)
					})
					ui.Column(c).Grow(1).Gap(3).Children(func() {
						ui.Text(c, a.info.Name).Bold().FontSize(15).TextColor(t.Text)
						ui.Text(c, fmt.Sprintf("Version %s (%s)", a.info.Version, a.info.Platform)).FontSize(12).TextColor(t.TextMuted)
						ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, iconCheck).FontSize(12).TextColor(t.Success)
							ui.Text(c, "System Keychain (hardware-encrypted)").FontSize(11.5).TextColor(t.TextMuted)
						})
					})
				})
			})
		})
	})
}

func (a *App) persistSettings() {
	settings := a.prefs
	a.settingsWrites <- settings
}

/* -------------------------------------------------------------------------
   Transfers Panel
   ------------------------------------------------------------------------- */

func (a *App) transfersPanel(c *ui.Context) {
	t := c.Theme()
	transfers := a.transfers()

	ui.Column(c).
		Height(210).
		Background(t.Surface).
		Border(1, t.Border).
		Radius(t.Radius).
		Padding(10, 14).
		Gap(8).
		Children(func() {
			// Header
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconTransfer).FontSize(15).TextColor(t.Accent)
				ui.Text(c, "Transfers").Bold().FontSize(13)
				if n := a.activeTransfers(); n > 0 {
					ui.Badge(c, fmt.Sprintf("%d active", n))
				}
				ui.Spacer(c)
				if ui.Button(c, "Clear finished").OnClick(a.clearTransfers).Clicked() {
					a.clearTransfers()
				}
				closeBtn := ui.Button(c.Key("close-transfers"), "").Label("Close")
				closeBtn.Children(func() { ui.Icon(c, iconClose).FontSize(13) })
				if closeBtn.Clicked() {
					a.transfersOpen = false
				}
			})

			ui.Divider(c)

			// Transfer List
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).FillWidth().Gap(6).Children(func() {
					if len(transfers) == 0 {
						ui.Box(c).FillWidth().Padding(20).Center().Children(func() {
							ui.Text(c, "No transfers queued or active.").FontSize(12).TextColor(t.TextMuted)
						})
						return
					}
					for _, tr := range transfers {
						x := tr
						ui.Row(c.Key(x.ID)).
							Padding(6, 10).
							Radius(6).
							Background(t.SurfaceHover).
							Gap(10).
							AlignItems(ui.Center).
							Children(func() {
								glyph := iconDownload
								glyphColor := t.Accent
								if x.Kind == store.TransferKind("upload") {
									glyph = iconUpload
									glyphColor = t.Success
								}
								ui.Icon(c, glyph).FontSize(15).TextColor(glyphColor)

								ui.Column(c).Grow(1).Gap(3).Children(func() {
									ui.Text(c, x.Name).FontSize(12).Bold().SingleLine().Ellipsis("…")
									ui.Text(c, fmt.Sprintf("%s · %s / %s", x.State, formatBytes(x.Done), formatBytes(x.Size))).
										FontSize(10.5).TextColor(t.TextMuted)
									if x.Size > 0 && !x.Finished() {
										ui.Progress(c, float64(x.Done)/float64(x.Size))
									}
									if x.Error != "" {
										ui.Text(c, x.Error).FontSize(11).TextColor(t.Danger)
									}
								})

								if !x.Finished() {
									ui.Button(c, "Cancel").OnClick(func() { a.cancelTransfer(x.ID) })
								}
							})
					}
				})
			})
		})
}
