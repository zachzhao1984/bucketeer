package main

import (
	"bucketeer/internal/store"
	"fmt"
	"github.com/egoist/mygo/ui"
)

func panel(c *ui.Context, title string, body func()) {
	t := c.Theme()
	ui.Column(c).FillWidth().Padding(16).Gap(12).Border(1, t.Border).Radius(10).Children(func() {
		ui.Text(c, title).Bold().FontSize(15)
		body()
	})
}

func (a *App) dashboard(c *ui.Context) {
	if a.bucket == "" {
		emptyState(c, iconDashboard, "Pick a bucket", "Open a bucket from All files to measure its storage.", func() { ui.Button(c, "All files").OnClick(func() { a.setSection(sectionFiles); a.loadBuckets() }) })
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().Padding(20).Gap(16).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.bucket).FontSize(22).Bold().Grow(1)
				ui.Button(c, "Measure bucket").Disabled(a.statLoading).OnClick(a.loadStat)
				if a.statLoading {
					ui.Spinner(c)
				}
			})
			if a.stat == nil {
				ui.Text(c, "Measure this bucket to see its storage summary.")
				return
			}
			s := a.stat
			ui.Row(c).Gap(20).Children(func() {
				for _, v := range []struct{ label, value string }{{"Total size", formatBytes(s.Bytes)}, {"Objects", formatCount(int(s.Objects))}, {"Folder markers", formatCount(int(s.Folders))}} {
					ui.Column(c).Grow(1).Padding(16).Background(c.Theme().SurfaceHover).Radius(10).Gap(8).Children(func() { ui.Text(c, v.label).TextColor(c.Theme().TextMuted); ui.Text(c, v.value).FontSize(24).Bold() })
				}
			})
			if s.Partial {
				ui.Text(c, "Partial scan: totals are a lower bound (50,000-object limit).").TextColor(c.Theme().Warning)
			}
			panel(c, "Where the space goes", func() {
				var largest int64 = 1
				for _, f := range s.TopFolders {
					largest = max(largest, f.Bytes)
				}
				for _, f := range s.TopFolders {
					ui.Row(c.Key(f.Prefix)).Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Text(c, f.Name).Width(150).SingleLine()
						ui.Progress(c, float64(f.Bytes)/float64(largest)).Grow(1)
						ui.Text(c, formatBytes(f.Bytes)).Width(90)
					})
				}
			})
			panel(c, "Storage classes", func() {
				for _, cl := range s.ByClass {
					ui.Text(c, fmt.Sprintf("%s · %s · %d objects", cl.Class, formatBytes(cl.Bytes), cl.Count))
				}
			})
			panel(c, "Largest objects", func() {
				for _, entry := range s.Largest {
					e := entry
					ui.Row(c.Key(e.Key)).Gap(10).Children(func() {
						ui.Button(c, e.Name).Grow(1).OnClick(func() { a.showDetail(e.Key) })
						ui.Text(c, formatBytes(e.Size))
					})
				}
			})
		})
	})
}

func (a *App) settings(c *ui.Context) {
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().Padding(20).Gap(16).Children(func() {
			ui.Text(c, "Settings").FontSize(22).Bold()
			panel(c, "Browsing", func() {
				ui.Checkbox(c.Key("thumbnails"), &a.prefs.ShowThumbnails, "Show thumbnails").OnChange(func() {
					a.persistSettings()
					if a.prefs.ShowThumbnails {
						a.loadThumbnails(a.source().Items)
					}
				})
				ui.Checkbox(c.Key("confirm-delete"), &a.prefs.ConfirmDelete, "Confirm before deleting").OnChange(a.persistSettings)
				ttl := ttlLabel(a.prefs.PresignTTLHours)
				ui.Field(c, "Share links last", func() {
					ui.Select(c.Key("lifetime"), &ttl, []string{"1 hour", "6 hours", "12 hours", "1 day", "3 days", "7 days"}).OnChange(func() {
						a.prefs.PresignTTLHours = map[string]int{"1 hour": 1, "6 hours": 6, "12 hours": 12, "1 day": 24, "3 days": 72, "7 days": 168}[ttl]
						a.persistSettings()
					})
				})
			})
			panel(c, "Connections", func() {
				for _, p := range a.connections {
					profile := p
					ui.Row(c.Key(p.ID)).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Grow(1).Children(func() {
							ui.Text(c, profile.Name).Bold()
							host := profile.Host()
							if profile.Bucket != "" {
								host += " · " + profile.Bucket
							}
							ui.Text(c, host).TextColor(c.Theme().TextMuted)
						})
						ui.Button(c, "Connect").OnClick(func() { a.connect(profile.ID) })
						ui.Button(c, "Edit").OnClick(func() { a.openConnection(&profile) })
						ui.Button(c, "Remove").OnClick(func() {
							a.ask("Remove “"+profile.Name+"”?", "This forgets the saved connection and its credentials.", "Remove", func() { a.removeConnection(profile) })
						})
					})
				}
				ui.PrimaryButton(c, "Add connection").OnClick(func() { a.openConnection(nil) })
			})
			panel(c, "About", func() {
				ui.Text(c, a.info.Name+" · "+a.info.Version+" · "+a.info.Platform)
				ui.Text(c, a.info.DataDir).Selectable()
				if a.info.SecretOK {
					ui.Text(c, "Credentials use the system credential store.")
				} else {
					ui.Text(c, a.info.SecretWhy)
				}
			})
		})
	})
}

func (a *App) persistSettings() {
	settings := a.prefs
	// Serialize saves; all writes finish in the order the user made them.
	a.settingsWrites <- settings
}

func (a *App) transfersPanel(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Height(210).Background(t.Surface).Border(1, t.Border).Radius(10).Padding(12).Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Transfers").Bold().Grow(1)
			ui.Button(c, "Clear finished").OnClick(a.clearTransfers)
			ui.Button(c, "Close").OnClick(func() { a.transfersOpen = false })
		})
		transfers := a.transfers()
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(10).Children(func() {
				if len(transfers) == 0 {
					ui.Text(c, "No transfers yet.").TextColor(t.TextMuted)
				}
				for _, tr := range transfers {
					x := tr
					ui.Row(c.Key(x.ID)).Gap(12).AlignItems(ui.Center).Children(func() {
						glyph := iconDownload
						if x.Kind == store.TransferKind("upload") {
							glyph = iconUpload
						}
						ui.Icon(c, glyph)
						ui.Column(c).Grow(1).Gap(4).Children(func() {
							ui.Text(c, x.Name).SingleLine()
							ui.Text(c, fmt.Sprintf("%s · %s / %s", x.State, formatBytes(x.Done), formatBytes(x.Size))).FontSize(11).TextColor(t.TextMuted)
							if x.Size > 0 {
								ui.Progress(c, float64(x.Done)/float64(x.Size))
							}
							if x.Error != "" {
								ui.Text(c, x.Error).TextColor(t.Danger)
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
