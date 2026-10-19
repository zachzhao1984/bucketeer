package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
)

// panel is a section of a page: a heading, the room around it, and the
// body beneath. Nothing here is a plate standing on another plate; the
// page is one plane and these are marks on it.
func panel(c *ui.Context, title string, body func()) {
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		panelHeading(c, title)
		ui.Column(c).FillWidth().Children(body)
	})
}

/* -------------------------------------------------------------------------
   Dashboard
   ------------------------------------------------------------------------- */

func (a *App) dashboard(c *ui.Context) {
	t := c.Theme()

	if a.bucket == "" {
		emptyState(c, iconDashboard, "Select a Storage Bucket",
			"Open a bucket from Files to inspect its storage breakdown.", func() {
				if ctlPrimary(c, "Browse Buckets").Clicked() {
					a.setSection(sectionFiles)
					a.loadBuckets()
				}
			})
		return
	}

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().Padding(22, 26).Gap(28).Children(func() {
			a.dashboardHeader(c)

			if a.stat == nil {
				ui.Box(c).Padding(48).Center().Children(func() {
					ui.Text(c, "Measure the bucket to scan its storage distribution.").
						FontSize(typeBody).
						TextColor(t.TextMuted)
				})
				return
			}

			s := a.stat
			a.dashboardMetrics(c, s)

			if s.Partial {
				warn := ui.Row(c).
					Padding(8, 11).
					Radius(btnRadius).
					Gap(8).
					AlignItems(ui.Center).
					Background(t.Warning.Alpha(0.10)).
					Border(0, ui.Transparent)
				warn.Children(func() {
					ui.Icon(c, iconAlert).FontSize(14).TextColor(t.Warning).Shrink(0)
					ui.Text(c, "Scan capped at 50,000 objects. Totals are a lower bound.").
						FontSize(typeCaption).
						TextColor(t.Text)
				})
			}

			if len(s.TopFolders) > 0 {
				a.dashboardFolders(c, s)
			}
			if len(s.ByClass) > 0 {
				a.dashboardClasses(c, s)
			}
			if len(s.Largest) > 0 {
				a.dashboardLargest(c, s)
			}
		})
	})
}

// dashboardHeader names the bucket these figures belong to.
func (a *App) dashboardHeader(c *ui.Context) {
	t := c.Theme()

	ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
		mark := ui.Box(c).Size(38, 38).Radius(9).Center().Shrink(0).
			Background(ui.Transparent).Border(0, ui.Transparent)
		mark.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(r, kindWash(t, kindFolder), 9)
		})
		mark.Children(func() {
			ui.Icon(c, iconBucket).FontSize(20).TextColor(kindTint(t, kindFolder))
		})

		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, a.bucket).
				FontSize(typeTitle).
				FontWeight(weightSemi).
				TextColor(t.Text).
				SingleLine().
				Ellipsis("…")
			ui.Text(c, "Storage").FontSize(typeCaption).TextColor(t.TextMuted)
		})

		if a.statLoading {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c).Label("Measuring")
			})
		} else {
			remeasure := ctlButton(c, "Re-measure")
			if remeasure.Clicked() {
				a.loadStat()
			}
		}
	})
}

// dashboardMetrics are the three figures worth knowing at a glance.
func (a *App) dashboardMetrics(c *ui.Context, s *store.Stat) {
	t := c.Theme()

	avgSize := "—"
	if s.Objects > 0 {
		avgSize = formatBytes(s.Bytes / s.Objects)
	}
	figures := []struct {
		label string
		value string
		sub   string
	}{
		{"Total Storage", formatBytes(s.Bytes), plural(int(s.Objects), "object", "objects")},
		{"Objects", formatCount(int(s.Objects)), plural(int(s.Folders), "folder", "folders")},
		{"Average Object", avgSize, "across all keys"},
	}

	ui.Row(c).FillWidth().Gap(16).Children(func() {
		for _, f := range figures {
			figure := f
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, figure.label).
					FontSize(typeCaption).
					FontWeight(weightMedium).
					TextColor(t.TextMuted)
				ui.Text(c, figure.value).
					FontSize(typeTitle).
					FontWeight(weightSemi).
					TextColor(t.Text)
				ui.Text(c, figure.sub).
					FontSize(typeCaption).
					TextColor(t.TextMuted)
			})
		}
	})

	// The rule that ends the figures, under them and across the page.
	ui.Row(c).FillWidth().Children(func() { rule(c, 0, chromeOf(t).hairline) })
}

// dashboardFolders shows where the bytes are, as one bar of shares
// above the list that gives them names.
func (a *App) dashboardFolders(c *ui.Context, s *store.Stat) {
	t := c.Theme()

	largest := int64(1)
	for _, f := range s.TopFolders {
		largest = max(largest, f.Bytes)
	}

	panel(c, "Where the Space Goes", func() {
		// The whole bucket as one bar: a segment per folder, sized by its
		// share of the total, the largest darkest. It is set in a track
		// so it reads as a composition rather than as something filling
		// up.
		track := ui.Box(c).FillWidth().Height(12).Radius(6).Clip().
			Background(chromeOf(t).inset).Border(0, ui.Transparent)
		track.Children(func() {
			ui.Row(c).Fill().Gap(2).Children(func() {
				for i, f := range s.TopFolders {
					folder := f
					share := float32(float64(folder.Bytes) / float64(max(s.Bytes, 1)))
					rank := float32(0.42 + 0.48*float64(i)/float64(max(len(s.TopFolders)-1, 1)))
					ui.Box(c).
						Grow(max(share, 0.02)).
						FillHeight().
						Shrink(0).
						Tooltip(folder.Name + " · " + formatBytes(folder.Bytes)).
						Background(kindTint(t, kindFolder).Alpha(rank))
				}
			})
		})

		ui.Column(c).FillWidth().Margin(10).Gap(9).Children(func() {
			for _, f := range s.TopFolders {
				folder := f
				pct := float64(folder.Bytes) / float64(max(s.Bytes, 1)) * 100
				ui.Row(c.Key(folder.Prefix)).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconFolder).FontSize(14).TextColor(kindTint(t, kindFolder)).Shrink(0)
					ui.Text(c, folder.Name).
						Width(140).
						FontSize(typeBody).
						SingleLine().
						Ellipsis("…")
					ctlMeter(c, float64(folder.Bytes)/float64(largest)).Grow(1)
					ui.Text(c, fmt.Sprintf("%.1f%%", pct)).
						Width(48).
						FontSize(typeCaption).
						TextColor(t.TextMuted).
						TextAlign(ui.End)
					ui.Text(c, formatBytes(folder.Bytes)).
						Width(76).
						FontSize(typeBody).
						FontWeight(weightMedium).
						TextAlign(ui.End)
				})
			}
		})
	})
}

// dashboardClasses is what the bytes are billed as.
func (a *App) dashboardClasses(c *ui.Context, s *store.Stat) {
	t := c.Theme()
	total := s.Bytes
	if total <= 0 {
		total = 1
	}

	panel(c, "Storage Classes", func() {
		ui.Column(c).FillWidth().Gap(1).Children(func() {
			for _, cl := range s.ByClass {
				class := cl
				ui.Row(c.Key(class.Class)).
					FillWidth().
					Padding(5, 2).
					Radius(4).
					Gap(12).
					AlignItems(ui.Center).
					Children(func() {
						ui.Text(c, class.Class).
							Width(120).
							FontSize(typeBody).
							SingleLine().
							Ellipsis("…")
						ctlMeter(c, float64(class.Bytes)/float64(total)).Grow(1)
						ui.Text(c, plural(int(class.Count), "object", "objects")).
							Width(84).
							FontSize(typeCaption).
							TextColor(t.TextMuted).
							TextAlign(ui.End)
						ui.Text(c, formatBytes(class.Bytes)).
							Width(76).
							FontSize(typeBody).
							FontWeight(weightMedium).
							TextAlign(ui.End)
					})
			}
		})
	})
}

// dashboardLargest lists the heaviest objects, because those are the
// ones worth knowing about.
func (a *App) dashboardLargest(c *ui.Context, s *store.Stat) {
	t := c.Theme()

	panel(c, "Largest Objects", func() {
		ui.Column(c).FillWidth().Gap(1).Children(func() {
			for _, entry := range s.Largest {
				e := entry
				ui.Row(c.Key(e.Key)).
					FillWidth().
					Padding(4, 2).
					Radius(4).
					Gap(10).
					AlignItems(ui.Center).
					Children(func() {
						ui.Icon(c, entryIcon(e)).
							FontSize(14).
							TextColor(kindTint(t, kindOf(e.Key, false))).
							Shrink(0)
						if ctlLink(c, e.Name).Grow(1).Clicked() {
							a.showDetail(e.Key)
						}
						ui.Text(c, formatBytes(e.Size)).
							Width(76).
							FontSize(typeBody).
							TextColor(t.TextMuted).
							TextAlign(ui.End)
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
		ui.Column(c).FillWidth().MaxWidth(660).Padding(20, 24).Gap(26).Children(func() {
			panel(c, "Browsing", func() {
				ui.Column(c).FillWidth().Children(func() {
					a.settingRow(c, "Show Image Thumbnails",
						"Decode and display previews for images in grid cards.", func() {
							ctlCheck(c, "thumbnails", &a.prefs.ShowThumbnails, "").OnChange(func() {
								a.persistSettings()
								if a.prefs.ShowThumbnails {
									a.loadThumbnails(a.source().Items)
								}
							})
						})
					settingRule(c)
					a.settingRow(c, "Confirm Before Deleting",
						"Ask before permanently deleting objects.", func() {
							ctlCheck(c, "confirm-delete", &a.prefs.ConfirmDelete, "").OnChange(a.persistSettings)
						})
					settingRule(c)
					a.settingRow(c, "Default Share Link Expiration",
						"How long presigned links stay accessible.", func() {
							ttl := ttlLabel(a.prefs.PresignTTLHours)
							sel := ctlSelect(c, "lifetime", &ttl,
								[]string{"1 hour", "6 hours", "12 hours", "1 day", "3 days", "7 days"}, 124)
							sel.OnChange(func() {
								a.prefs.PresignTTLHours = map[string]int{
									"1 hour": 1, "6 hours": 6, "12 hours": 12,
									"1 day": 24, "3 days": 72, "7 days": 168,
								}[ttl]
								a.persistSettings()
							})
						})
				})
			})

			panel(c, "Connections", func() {
				ui.Column(c).FillWidth().Gap(4).Children(func() {
					if len(a.connections) == 0 {
						ui.Text(c, "No saved connections yet.").
							FontSize(typeBody).
							TextColor(t.TextMuted).
							Padding(0, 4, 4, 4)
					}
					for _, p := range a.connections {
						profile := p
						a.connectionSettingRow(c, profile)
					}
					ui.Row(c).Gap(8).Children(func() {
						if ctlButton(c, "Add Connection").Clicked() {
							a.openConnection(nil)
						}
					})
				})
			})

			panel(c, "About", func() {
				ui.Row(c).FillWidth().Gap(13).AlignItems(ui.Center).Children(func() {
					mark := ui.Box(c).Size(46, 46).Radius(11).Center().Shrink(0).
						Background(ui.Transparent).Border(0, ui.Transparent)
					mark.Draw(func(p *ui.Painter, r ui.Rect) {
						paintSurface(p, r, 11, t.Accent, ui.Transparent)
					})
					mark.Children(func() {
						ui.Icon(c, iconCloud).FontSize(23).TextColor(chromeOf(t).accentOnFill)
					})
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Text(c, a.info.Name).
							FontSize(typeHeadline).
							FontWeight(weightSemi).
							TextColor(t.Text)
						ui.Text(c, fmt.Sprintf("Version %s · %s", a.info.Version, a.info.Platform)).
							FontSize(typeCaption).
							TextColor(t.TextMuted)
						ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
							led(c, t.Success, 5)
							ui.Text(c, "Credentials in the System Keychain").
								FontSize(typeCaption).
								TextColor(t.TextMuted)
						})
					})
				})
			})
		})
	})
}

// connectionSettingRow is one saved connection, with its own actions.
func (a *App) connectionSettingRow(c *ui.Context, profile store.Profile) {
	t := c.Theme()
	ch := chromeOf(t)

	row := ui.Row(c).Key(profile.ID).
		FillWidth().
		Padding(6, 8).
		Gap(11).
		AlignItems(ui.Center).
		Radius(btnRadius).
		Background(ui.Transparent).
		Border(0, ui.Transparent)
	row.Draw(func(p *ui.Painter, r ui.Rect) {
		paintSurface(p, r, btnRadius, ui.Transparent, ch.hairlineFaint)
	})

	row.Children(func() {
		monogram(c, profile.Name, 28)

		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, profile.Name).FontSize(typeBody).FontWeight(weightMedium).SingleLine().Ellipsis("…")
			detail := profile.Host()
			if profile.Bucket != "" {
				detail += " · " + profile.Bucket
			}
			ui.Text(c, detail).FontSize(typeCaption).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
		})

		connect := ctlButton(c, "Connect")
		if connect.Clicked() {
			a.connect(profile.ID)
		}
		edit := ctlButton(c, "Edit")
		if edit.Clicked() {
			a.openConnection(&profile)
		}
		remove := ctlButton(c, "Remove")
		if remove.Clicked() {
			a.ask("Remove “"+profile.Name+"”?",
				"This deletes the saved connection and its credentials from the Keychain.",
				"Remove", func() { a.removeConnection(profile) })
		}
	})
}

// settingRow is a titled preference with its control on the right. The
// room above and below each row is what makes the list readable; the
// hairline between them only confirms it.
func (a *App) settingRow(c *ui.Context, title, detail string, control func()) {
	t := c.Theme()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(16).Padding(9, 4).Children(func() {
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, title).FontSize(typeBody).FontWeight(weightMedium).TextColor(t.Text)
			ui.Text(c, detail).FontSize(typeCaption).TextColor(t.TextMuted)
		})
		control()
	})
}

// settingRule is the hairline between two preferences.
func settingRule(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Margin(0, 4).Children(func() { rule(c, 0, chromeOf(t).hairline) })
}

func (a *App) persistSettings() {
	settings := a.prefs
	a.settingsWrites <- settings
}

/* -------------------------------------------------------------------------
   Transfers Panel
   ------------------------------------------------------------------------- */

// transfersPanel is the queue, docked under the content. It is a list of
// work in progress, so it takes a header and rows rather than a plate.
func (a *App) transfersPanel(c *ui.Context) {
	t := c.Theme()
	ch := chromeOf(t)
	transfers := a.transfers()

	dock := ui.Column(c).
		Height(210).
		Background(ui.Transparent).
		Border(0, ui.Transparent)
	dock.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: 1}, ch.hairline, 0)
		p.Fill(ui.Rect{X: r.X, Y: r.Y + 1, W: r.W, H: r.H - 1}, ch.inset, 0)
	})

	dock.Children(func() {
		ui.Row(c).FillWidth().Height(30).Padding(0, 16).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, iconTransfer).FontSize(14).TextColor(t.Accent).Shrink(0)
			ui.Text(c, "Transfers").FontSize(typeBody).FontWeight(weightSemi).TextColor(t.Text)
			if n := a.activeTransfers(); n > 0 {
				ctlBadge(c, fmt.Sprintf("%d Active", n), t.Accent)
			}
			ui.Spacer(c)

			if len(transfers) > 0 {
				clear := ctlButton(c, "Clear Finished")
				if clear.Clicked() {
					a.clearTransfers()
				}
			}
			close := ctlIconButtonBare(c, "Close Transfers")
			close.Children(func() { ui.Icon(c, iconClose).FontSize(12).TextColor(t.Text) })
			if close.Clicked() {
				a.transfersOpen = false
			}
		})

		ui.Row(c).FillWidth().Padding(0, 16).Children(func() { rule(c, 0, ch.hairline) })

		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Padding(6, 12).Gap(3).Children(func() {
				if len(transfers) == 0 {
					ui.Box(c).FillWidth().Padding(24).Center().Children(func() {
						ui.Text(c, "No transfers queued or active.").
							FontSize(typeBody).
							TextColor(t.TextMuted)
					})
					return
				}
				for _, tr := range transfers {
					a.transferRow(c, tr)
				}
			})
		})
	})
}

// transferRow is one transfer: where it is, how far along, and the one
// thing that can still be done to it.
func (a *App) transferRow(c *ui.Context, x store.Transfer) {
	t := c.Theme()

	glyph, tint := iconDownload, t.Accent
	if x.Kind == store.TransferKind("upload") {
		glyph, tint = iconUpload, t.Success
	}

	ui.Row(c).Key(x.ID).FillWidth().Padding(6, 8).Gap(10).AlignItems(ui.Center).
		Radius(btnRadius).
		Children(func() {
			ui.Icon(c, glyph).FontSize(15).TextColor(tint).Shrink(0)

			ui.Column(c).Grow(1).Gap(3).Children(func() {
				ui.Text(c, x.Name).FontSize(typeBody).SingleLine().Ellipsis("…")
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Text(c, string(x.State)).
						FontSize(typeCaption).
						TextColor(t.TextMuted).
						SingleLine()
					if x.Size > 0 {
						ui.Text(c, fmt.Sprintf("%s / %s", formatBytes(x.Done), formatBytes(x.Size))).
							FontSize(typeCaption).
							TextColor(t.TextMuted).
							SingleLine()
					}
					if x.Error != "" {
						ui.Text(c, x.Error).FontSize(typeCaption).TextColor(t.Danger).SingleLine().Ellipsis("…")
					}
				})
				if x.Size > 0 && !x.Finished() {
					ctlMeter(c, float64(x.Done)/float64(x.Size)).FillWidth()
				}
			})

			if !x.Finished() {
				cancel := ctlButton(c, "Cancel")
				if cancel.Clicked() {
					a.cancelTransfer(x.ID)
				}
			}
		})
}
