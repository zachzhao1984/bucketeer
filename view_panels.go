package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"bucketeer/internal/store"
)

/* -------------------------------------------------------------------------
   Dashboard

   Four figures across the top, then what the bytes are, by folder and by
   kind. Everything is a card standing on the canvas, with its own name on
   it and its figures under that.
   ------------------------------------------------------------------------- */

func (a *App) dashboard(c *ui.Context) {
	t := c.Theme()

	if a.bucket == "" {
		emptyState(c, iconDashboard, "Select a bucket",
			"Open a bucket from All files to see what is in it.", func() {
				if ctlPrimary(c, "Browse buckets").Clicked() {
					a.setSection(sectionFiles)
					a.loadBuckets()
				}
			})
		return
	}

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).FillWidth().Padding(18).Gap(14).Children(func() {
			if a.stat == nil {
				if a.statLoading {
					a.dashboardLoading(c)
					return
				}
				ui.Box(c).FillWidth().Padding(60).Center().Children(func() {
					ui.Column(c).Gap(10).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "Nothing measured yet").
							FontSize(typeBody).
							TextColor(t.TextMuted)
						if ctlPrimary(c, "Measure this bucket").Clicked() {
							a.loadStat()
						}
					})
				})
				return
			}

			s := a.stat
			a.dashboardFigures(c, s)

			if s.Partial {
				warn := ui.Row(c).
					Padding(9, 12).
					Radius(btnRadius).
					Gap(8).
					AlignItems(ui.Center).
					Background(t.Warning.Alpha(0.09)).
					Border(0, ui.Transparent)
				warn.Children(func() {
					ui.Icon(c, iconAlert).FontSize(14).TextColor(t.Warning).Shrink(0)
					ui.Text(c, "The scan stopped at 50,000 objects. The totals are a lower bound.").
						FontSize(typeCaption).
						TextColor(t.Text)
				})
			}

			ui.Row(c).FillWidth().Gap(14).Children(func() {
				ui.Column(c).Grow(1).Children(func() {
					a.dashboardFolders(c, s)
				})
			})

			ui.Row(c).FillWidth().Gap(14).Children(func() {
				ui.Column(c).Grow(1).Children(func() {
					a.dashboardKinds(c, s)
				})
			})
		})
	})
}

// dashboardLoading stands in for figures that have not been counted yet.
func (a *App) dashboardLoading(c *ui.Context) {
	ch := chromeOf(c.Theme())
	ui.Row(c).FillWidth().Gap(14).Children(func() {
		for i := range 4 {
			ui.Box(c.Key(i)).Grow(1).Height(128).Radius(12).Background(ui.Transparent).
				Draw(func(p *ui.Painter, r ui.Rect) {
					p.Fill(r, ch.inset, 12)
				})
		}
	})
}

// dashboardFigures are the four figures worth knowing at a glance, each
// on a card with a colored mark.
func (a *App) dashboardFigures(c *ui.Context, s *store.Stat) {
	t := c.Theme()

	avg := "—"
	if s.Objects > 0 {
		avg = formatBytes(s.Bytes / s.Objects)
	}
	largest := "—"
	if len(s.Largest) > 0 {
		largest = formatBytes(s.Largest[0].Size)
	}
	changed := "Never"
	if !s.LastModified.IsZero() {
		changed = formatRelative(s.LastModified, now())
	}

	figures := []struct {
		glyph *ui.SVG
		tint  ui.Color
		label string
		value string
		chip  string
		tally string
	}{
		{iconBucket, kindTint(t, kindFolder), "Total storage", formatBytes(s.Bytes), "Largest", largest},
		{iconFiles, kindTint(t, kindImage), "Objects", formatCount(int(s.Objects)), "Folders", formatCount(int(s.Folders))},
		{iconDashboard, kindTint(t, kindArchive), "Avg. object size", avg, "Largest object", largest},
		{iconRecent, kindTint(t, kindAudio), "Modified · 7 days", formatCount(int(s.Recent)), "Last changed", changed},
	}

	ui.Row(c).FillWidth().Gap(14).Children(func() {
		for _, f := range figures {
			fig := f
			box := card(c).Grow(1).Shrink(1).Padding(14).Gap(8)
			box.Children(func() {
				iconPlate(c, fig.glyph, fig.tint, 32)
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, fig.label).FontSize(typeBody).TextColor(t.TextMuted).SingleLine()
					ui.Text(c, fig.value).
						FontSize(typeDisplay).
						FontWeight(weightSemi).
						TextColor(t.Text).
						SingleLine().
						Ellipsis("…")
				})
				ui.Spacer(c)
				tally := ui.Row(c).FillWidth().Padding(5, 8).Radius(7).
					Background(chromeOf(t).inset).Gap(6).AlignItems(ui.Center)
				tally.Children(func() {
					ui.Text(c, fig.chip).FontSize(typeCaption).TextColor(t.TextMuted).Grow(1).SingleLine().Ellipsis("…")
					ui.Text(c, fig.tally).FontSize(typeCaption).FontWeight(weightMedium).TextColor(t.Text).SingleLine().Ellipsis("…")
				})
			})
		}
	})
}

// dashboardFolders is where the bytes are, as a bar of shares above the
// list that names them.
func (a *App) dashboardFolders(c *ui.Context, s *store.Stat) {
	t := c.Theme()
	if len(s.TopFolders) == 0 {
		return
	}

	box := card(c).FillWidth().Padding(16).Gap(12)
	box.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "Storage by folder").FontSize(typeBody).FontWeight(weightMedium).TextColor(t.Text)
				ui.Text(c, formatBytes(s.Bytes)).FontSize(typeTitle).FontWeight(weightSemi).TextColor(t.Text)
			})
			// The figures can be read as bytes or as a count, so the
			// switch is the one thing this card asks.
			sizeOn := !a.foldersByCount
			track := ctlSegmentTrack(c)
			track.Children(func() {
				seg := ctlSegment(c, &sizeOn, "Size")
				seg.Width(58)
				seg.Children(func() {
					ui.Text(c, "Size").FontSize(typeCaption).TextColor(segmentTint(sizeOn, t))
				})
				seg.OnClick(func() {
					sizeOn = true
					a.foldersByCount = false
				})

				countOn := a.foldersByCount
				seg2 := ctlSegment(c, &countOn, "Objects")
				seg2.Width(66)
				seg2.Children(func() {
					ui.Text(c, "Objects").FontSize(typeCaption).TextColor(segmentTint(countOn, t))
				})
				seg2.OnClick(func() {
					countOn = true
					a.foldersByCount = true
				})
			})
		})

		// The whole bucket as one bar: a segment per folder, sized by its
		// share of the total.
		bar := ui.Box(c).FillWidth().Height(12).Radius(6).Clip().Background(chromeOf(t).inset)
		bar.Children(func() {
			ui.Row(c).Fill().Gap(2).Children(func() {
				for i, f := range s.TopFolders {
					folder := f
					share := float32(float64(folder.Bytes) / float64(max(s.Bytes, 1)))
					rank := float32(0.38 + 0.52*float64(i)/float64(max(len(s.TopFolders)-1, 1)))
					ui.Box(c).
						Grow(max(share, 0.02)).
						FillHeight().
						Shrink(0).
						Tooltip(folder.Name + " · " + formatBytes(folder.Bytes)).
						Background(kindTint(t, kindFolder).Alpha(rank))
				}
			})
		})

		ui.Column(c).FillWidth().Gap(2).Children(func() {
			largest := int64(1)
			for _, f := range s.TopFolders {
				if a.foldersByCount {
					largest = max(largest, f.Objects)
				} else {
					largest = max(largest, f.Bytes)
				}
			}
			for _, f := range s.TopFolders {
				folder := f
				share, value := float64(folder.Bytes)/float64(largest), formatBytes(folder.Bytes)
				if a.foldersByCount {
					share, value = float64(folder.Objects)/float64(largest), formatCount(int(folder.Objects))
				}
				row := ui.Row(c.Key(folder.Prefix)).FillWidth().Height(34).Gap(10).
					AlignItems(ui.Center).Cursor(ui.CursorPointer).Radius(7).
					OnClick(func() {
						a.setSection(sectionFiles)
						a.openPrefix(folder.Prefix)
					})
				row.Label(folder.Name)
				hovered := row.Hovered()
				row.Background(ui.Transparent).Border(0, ui.Transparent).
					Draw(func(p *ui.Painter, r ui.Rect) {
						if hovered {
							p.Fill(r, chromeOf(t).hover, 7)
						}
					})
				row.Children(func() {
					ui.Icon(c, iconFolder).FontSize(15).TextColor(kindTint(t, kindFolder)).Shrink(0)
					ui.Text(c, folder.Name).Width(150).FontSize(typeBody).SingleLine().Ellipsis("…")
					ctlMeter(c, share).Grow(1)
					ui.Text(c, value).
						Width(86).
						FontSize(typeBody).
						TextColor(t.TextMuted).
						TextAlign(ui.End)
					ui.Text(c, fmt.Sprintf("%.0f%%", float64(folder.Bytes)/float64(max(s.Bytes, 1))*100)).
						Width(44).
						FontSize(typeCaption).
						TextColor(t.TextMuted).
						TextAlign(ui.End)
				})
			}
		})
	})
}

// dashboardKinds is what the bytes are: a bar of the kinds, and the
// figures beside it.
func (a *App) dashboardKinds(c *ui.Context, s *store.Stat) {
	t := c.Theme()
	if len(s.ByKind) == 0 {
		return
	}

	box := card(c).FillWidth().Padding(16).Gap(14)
	box.Children(func() {
		ui.Text(c, "Storage by type").FontSize(typeBody).FontWeight(weightMedium).TextColor(t.Text)

		ui.Row(c).FillWidth().Gap(18).AlignItems(ui.Center).Children(func() {
			// The total, as the reference client prints it beside the bar.
			ui.Column(c).Gap(1).Shrink(0).Width(110).Children(func() {
				ui.Text(c, formatBytes(s.Bytes)).FontSize(typeTitle).FontWeight(weightSemi).TextColor(t.Text)
				ui.Text(c, "Total").FontSize(typeCaption).TextColor(t.TextMuted)
			})

			ui.Column(c).Grow(1).Gap(10).Children(func() {
				for _, k := range s.ByKind {
					kind := k
					ui.Row(c.Key(kind.Kind)).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
						led(c, kindTint(t, fileKind(kind.Kind)), 8)
						ui.Text(c, kindName(kind.Kind)).
							Width(80).
							FontSize(typeBody).
							TextColor(t.Text).
							SingleLine()
						ctlMeter(c, float64(kind.Bytes)/float64(max(s.Bytes, 1))).Grow(1)
						ui.Text(c, plural(int(kind.Count), "object", "objects")).
							Width(80).
							FontSize(typeCaption).
							TextColor(t.TextMuted).
							TextAlign(ui.End)
						ui.Text(c, formatBytes(kind.Bytes)).
							Width(80).
							FontSize(typeBody).
							TextColor(t.TextMuted).
							TextAlign(ui.End)
						ui.Text(c, fmt.Sprintf("%.0f%%", float64(kind.Bytes)/float64(max(s.Bytes, 1))*100)).
							Width(42).
							FontSize(typeCaption).
							TextColor(t.TextMuted).
							TextAlign(ui.End)
					})
				}
			})
		})
	})
}

// kindName is what a kind of file is called on the dashboard.
func kindName(kind string) string {
	switch kind {
	case "image":
		return "Images"
	case "video":
		return "Video"
	case "audio":
		return "Audio"
	case "archive":
		return "Archives"
	case "document":
		return "Documents"
	default:
		return "Other"
	}
}

/* -------------------------------------------------------------------------
   Settings

   A section list on the left and the page it shows on the right, inside
   one floating sheet, as the reference client opens it.
   ------------------------------------------------------------------------- */

type settingsPage int

const (
	pagePreferences settingsPage = iota
	pageConnections
	pagePicGo
	pageCLI
	pageAbout
)

func (a *App) settings(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Fill().Padding(24).Children(func() {
		sheet := ui.Column(c).Fill().MaxWidth(820).Radius(14).
			Background(ui.Transparent).Border(0, ui.Transparent)
		sheet.Draw(func(p *ui.Painter, r ui.Rect) {
			paintPlate(p, r, 14, chromeOf(t), chromeOf(t).plate, chromeOf(t).hairline)
		})
		sheet.Children(func() {
			ui.Row(c).Fill().Children(func() {
				a.settingsSidebar(c)
				ui.Box(c).Width(1).FillHeight().Background(chromeOf(t).hairline)
				ui.Column(c).Grow(1).FillHeight().Padding(20, 22).Children(func() {
					ui.Scroll(c).Grow(1).Children(func() {
						switch a.settingsPage {
						case pageConnections:
							a.settingsConnections(c)
						case pagePicGo:
							a.settingsPicGo(c)
						case pageCLI:
							a.settingsCLI(c)
						case pageAbout:
							a.settingsAbout(c)
						default:
							a.settingsPreferences(c)
						}
					})
				})
			})
		})
	})
}

// settingsSidebar is the list of pages, with the chosen one filled.
func (a *App) settingsSidebar(c *ui.Context) {
	t := c.Theme()
	ch := chromeOf(t)

	ui.Column(c).Width(190).FillHeight().Padding(10, 8).Gap(2).Children(func() {
		ui.Text(c, "Settings").FontSize(typeSmall).TextColor(t.TextMuted).Padding(6, 10, 8, 10)

		pages := []struct {
			page  settingsPage
			label string
			glyph *ui.SVG
		}{
			{pagePreferences, "Preferences", iconSliders},
			{pageConnections, "Connections", iconBucket},
			{pagePicGo, "Image hosting", iconImage},
			{pageCLI, "Command line", iconTerminal},
			{pageAbout, "About", iconInfo},
		}
		for _, p := range pages {
			entry := p
			chosen := a.settingsPage == entry.page
			row := ui.ButtonBase(c.Key("page:"+entry.label)).
				FillWidth().Height(34).Padding(0, 10).Gap(9).Radius(8).
				AlignItems(ui.Center).Justify(ui.Start).Cursor(ui.CursorPointer).
				TextColor(t.Text).OnClick(func() {
				if a.settingsPage == entry.page {
					return
				}
				a.settingsPage = entry.page
				// The Command line page reads the link and the installed
				// skills from disk, so it is brought up to date on entry.
				if entry.page == pageCLI {
					a.refreshCLIState()
				}
			})
			row.Label(entry.label)
			hovered := row.Hovered()
			if chosen {
				row.TextColor(ch.selectedOn)
			}
			row.Children(func() {
				color := t.TextMuted
				if chosen {
					color = ch.selectedOn
				}
				// A fixed slot keeps every label on the same left edge,
				// whatever the glyph's own width is.
				slot := ui.Box(c).Width(18).Shrink(0).Center()
				slot.Children(func() {
					ui.Icon(c, entry.glyph).FontSize(15).TextColor(color)
				})
				ui.Text(c, entry.label).FontSize(typeBody).FontWeight(weightMedium).Grow(1).SingleLine().Ellipsis("…")
			})
			row.Background(ui.Transparent).Border(0, ui.Transparent).
				Draw(func(p *ui.Painter, r ui.Rect) {
					switch {
					case chosen:
						p.Fill(r, ch.selected, 8)
					case hovered:
						p.Fill(r, ch.hover, 8)
					}
				})
		}
	})
}

// settingsPageTitle is the name of the settings page that is open, for the
// window's path.
func (a *App) settingsPageTitle() string {
	switch a.settingsPage {
	case pageConnections:
		return "Connections"
	case pageCLI:
		return "Command line"
	case pageAbout:
		return "About"
	}
	return "Preferences"
}

// settingsHeader opens a settings page: its title, and the line that says
// what the page is for. Every page starts with it so the titles share one
// size, one weight and one place.
func settingsHeader(c *ui.Context, title, detail string) {
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(3).Children(func() {
		ui.Text(c, title).FontSize(typeHeadline).FontWeight(weightMedium).TextColor(t.Text)
		if detail != "" {
			ui.Text(c, detail).
				FontSize(typeBody).
				TextColor(t.TextMuted).
				MaxWidth(560)
		}
	})
}

// settingsPreferences is the page of switches and choices.
func (a *App) settingsPreferences(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).FillWidth().Gap(18).Children(func() {
		settingsHeader(c, "Preferences", "Appearance, browsing and sharing.")

		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "Appearance").FontSize(typeBody).TextColor(t.TextMuted)
			group(c, func() {
				a.settingRow(c, "Theme", "Light or dark interface", func() {
					track := ctlSegmentTrack(c)
					track.Children(func() {
						light := !a.prefs.DarkMode
						seg := ctlSegment(c, &light, "Light")
						seg.Width(60)
						seg.Children(func() { ui.Text(c, "Light").FontSize(typeBody) })
						seg.OnClick(func() { a.setDark(false) })

						dark := a.prefs.DarkMode
						seg2 := ctlSegment(c, &dark, "Dark")
						seg2.Width(60)
						seg2.Children(func() { ui.Text(c, "Dark").FontSize(typeBody) })
						seg2.OnClick(func() { a.setDark(true) })
					})
				})
			})
		})

		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "Browsing").FontSize(typeBody).TextColor(t.TextMuted)
			group(c, func() {
				a.settingRow(c, "Show image thumbnails", "Decode and display pictures on grid cards.", func() {
					ctlSwitch(c, "thumbnails", &a.prefs.ShowThumbnails, "Show image thumbnails").OnChange(func() {
						a.persistSettings()
						if a.prefs.ShowThumbnails {
							a.loadThumbnails(a.source().Items)
						}
					})
				})
				settingRule(c)
				a.settingRow(c, "Confirm before deleting", "Ask before permanently deleting objects.", func() {
					ctlSwitch(c, "confirm-delete", &a.prefs.ConfirmDelete, "Confirm before deleting").OnChange(a.persistSettings)
				})
				settingRule(c)
				a.settingRow(c, "Default share link expiration", "How long presigned links stay accessible.", func() {
					ttl := ttlLabel(a.prefs.PresignTTLHours)
					width := float32(120)
					btn := ctlMenuButton(c, "Expiration", ttl, a.ttlOpen, width)
					if btn.Clicked() {
						a.ttlOpen = !a.ttlOpen
					}
					ui.Popover(c, btn, &a.ttlOpen, func() {
						ctlMenuPanel(c, 168).Children(func() {
							ui.Column(c).Gap(1).Children(func() {
								for _, hours := range []int{1, 6, 12, 24, 72, 168} {
									h := hours
									label := ttlLabel(h)
									if menuItem(c, label, "", a.prefs.PresignTTLHours == h, "") {
										a.ttlOpen = false
										a.prefs.PresignTTLHours = h
										a.persistSettings()
									}
								}
							})
						})
					})
				})
			})
		})
	})
}

// settingsConnections is the saved connections, each with its own
// actions, and the way to add another.
func (a *App) settingsConnections(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).FillWidth().Gap(14).Children(func() {
		settingsHeader(c, "Connections", "The storage accounts this app can browse, with their keys kept in the system credential store.")

		if len(a.connections) == 0 {
			ui.Text(c, "No saved connections yet.").FontSize(typeBody).TextColor(t.TextMuted)
		}

		for _, p := range a.connections {
			profile := p
			a.connectionCard(c, profile)
		}

		add := ctlButton(c, "")
		add.Label("Add connection")
		// The plate spans the page, with its mark and label on the same
		// left edge as the cards above it.
		add.FillWidth().Justify(ui.Start)
		add.Children(func() {
			ui.Icon(c, iconPlus).FontSize(14).TextColor(t.Text).Shrink(0)
			ui.Text(c, "Add connection").FontSize(typeBody)
		})
		if add.Clicked() {
			a.openConnection(nil)
		}
	})
}

// connectionCard is one saved connection: its mark, its name, where it
// points, and the two things that can be done to it.
func (a *App) connectionCard(c *ui.Context, profile store.Profile) {
	t := c.Theme()
	active := a.session.Profile != nil && a.session.Profile.ID == profile.ID

	box := card(c).FillWidth().Padding(12, 14).Gap(0)
	box.Children(func() {
		ui.Row(c).FillWidth().Gap(11).AlignItems(ui.Center).Children(func() {
			monogram(c, profile.Name, 34)

			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, profile.Name).FontSize(typeBody).FontWeight(weightMedium).TextColor(t.Text).SingleLine().Ellipsis("…")
					if active {
						ctlBadge(c, "Active", t.Success)
					}
				})
				detail := profile.Host()
				if profile.Bucket != "" {
					detail = profile.Bucket + " · " + detail
				}
				ui.Text(c, detail).FontSize(typeCaption).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
			})

			edit := ctlIconButtonBare(c, "Edit "+profile.Name)
			edit.Children(func() { ui.Icon(c, iconRename).FontSize(14).TextColor(t.TextMuted) })
			if edit.Clicked() {
				a.openConnection(&profile)
			}

			remove := ctlIconButtonBare(c, "Remove "+profile.Name)
			remove.Children(func() { ui.Icon(c, iconTrash).FontSize(14).TextColor(t.TextMuted) })
			if remove.Clicked() {
				a.ask("Remove “"+profile.Name+"”?",
					"This deletes the saved connection and its credentials from the Keychain.",
					"Remove", func() { a.removeConnection(profile) })
			}
		})
	})
}

// settingsAbout is what the app is, and where it keeps things.
func (a *App) settingsAbout(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).FillWidth().Gap(16).Children(func() {
		settingsHeader(c, "About", "")

		ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Center).Children(func() {
			iconPlate(c, iconCloud, t.Accent, 52)
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, a.info.Name).FontSize(typeTitle).FontWeight(weightSemi).TextColor(t.Text)
				ui.Text(c, "A native client for Cloudflare R2 and S3-compatible storage.").
					FontSize(typeBody).
					TextColor(t.TextMuted)
			})
		})

		group(c, func() {
			a.settingRow(c, "Version", "The build running on this machine.", func() {
				ui.Text(c, a.info.Version+" · "+a.info.Platform).FontSize(typeBody).TextColor(t.TextMuted)
			})
			settingRule(c)
			a.settingRow(c, "Credentials", "Access keys live in the system credential store.", func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					if a.info.SecretOK {
						led(c, t.Success, 6)
						ui.Text(c, "Keychain").FontSize(typeBody).TextColor(t.TextMuted)
					} else {
						led(c, t.Warning, 6)
						ui.Text(c, "Unavailable").FontSize(typeBody).TextColor(t.Warning)
					}
				})
			})
			settingRule(c)
			a.settingRow(c, "Data folder", a.info.DataDir, func() {
				if ctlButton(c, "Show").Clicked() {
					a.services.OpenURL("file://" + a.info.DataDir)
				}
			})
		})
	})
}

// group is the recessed plate a settings page's rows sit on.
func group(c *ui.Context, body func()) {
	t := c.Theme()
	e := ui.Column(c).FillWidth().Padding(4, 14).Radius(10).
		Background(ui.Transparent).Border(0, ui.Transparent)
	e.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, chromeOf(t).inset, 10)
	})
	e.Children(body)
}

// settingRow is a titled preference with its control on the right.
func (a *App) settingRow(c *ui.Context, title, detail string, control func()) {
	t := c.Theme()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(16).Padding(10, 2).Children(func() {
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, title).FontSize(typeBody).TextColor(t.Text)
			ui.Text(c, detail).FontSize(typeCaption).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
		})
		control()
	})
}

// settingRule is the hairline between two preferences. It runs the width of
// the group and keeps clear of its rounded ends, and is quiet enough to
// separate rows without drawing attention to itself.
func settingRule(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Children(func() {
		rule(c, 12, chromeOf(t).hairline)
	})
}

func (a *App) persistSettings() {
	settings := a.prefs
	a.settingsWrites <- settings
}

// setDark switches the interface's appearance and remembers the choice.
func (a *App) setDark(dark bool) {
	a.prefs.DarkMode = dark
	a.persistSettings()
}

/* -------------------------------------------------------------------------
   Transfers

   A dock in the bottom right corner of the content, over it rather than
   beside it, as the reference client shows its queue.
   ------------------------------------------------------------------------- */

func (a *App) transfersDock(c *ui.Context) {
	t := c.Theme()
	ch := chromeOf(t)
	transfers := a.transfers()

	winW, winH := c.Size()
	height := float32(300)
	if len(transfers) == 0 {
		height = 168
	}

	ui.Overlay(c, func() {
		dock := ui.Column(c).
			Absolute().
			Left(winW-400).
			Top(winH-height-20).
			Width(380).
			Height(height).
			Radius(12).
			Background(ui.Transparent).
			Border(0, ui.Transparent)
		dock.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Shadow(r, 12, 0, 12, 40, 0, ch.floatShadow)
			p.Fill(r, ch.plate, 12)
			p.Stroke(r, ch.hairline, 12, 1)
		})
		a.dockBody(c, dock, transfers)
	})
}

// dockBody fills the transfers dock: its title bar, then the queue or
// the quiet state that says there is none.
func (a *App) dockBody(c *ui.Context, dock ui.Element, transfers []store.Transfer) {
	t := c.Theme()
	ch := chromeOf(t)

	dock.Children(func() {
		ui.Row(c).FillWidth().Padding(10, 12).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Transfers").FontSize(typeBody).FontWeight(weightMedium).TextColor(t.Text)
			if n := a.activeTransfers(); n > 0 {
				ctlBadge(c, formatCount(n), t.Accent)
			}
			ui.Spacer(c)
			if len(transfers) > 0 {
				clear := ctlIconButtonBare(c, "Clear finished")
				clear.Children(func() { ui.Icon(c, iconTrash).FontSize(13).TextColor(t.TextMuted) })
				if clear.Clicked() {
					a.clearTransfers()
				}
			}
			close := ctlIconButtonBare(c, "Close transfers")
			close.Children(func() {
				ui.Icon(c, iconClose).FontSize(12).TextColor(t.TextMuted)
			})
			if close.Clicked() {
				a.transfersOpen = false
			}
		})

		if len(transfers) == 0 {
			ui.Column(c).FillWidth().Padding(10, 20, 24, 20).Center().Gap(6).Children(func() {
				mark := ui.Box(c).Size(38, 38).Radius(19).Center().Shrink(0).
					Background(ui.Transparent).Border(0, ui.Transparent)
				mark.Draw(func(p *ui.Painter, r ui.Rect) {
					p.Fill(r, ch.inset, 19)
				})
				mark.Children(func() {
					ui.Icon(c, iconTransfer).FontSize(18).TextColor(t.TextMuted)
				})
				ui.Text(c, "No transfers yet").FontSize(typeBody).FontWeight(weightMedium).TextColor(t.Text)
				ui.Text(c, "Uploads and downloads will show up here.").
					FontSize(typeCaption).
					TextColor(t.TextMuted).
					TextAlign(ui.Center)
			})
			return
		}

		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Padding(4, 8, 8, 8).Gap(2).Children(func() {
				for _, tr := range transfers {
					a.transferRow(c, tr)
				}
			})
		})
	})
}

// transferRow is one transfer: what it is, how far along, and the one
// thing that can still be done to it.
func (a *App) transferRow(c *ui.Context, x store.Transfer) {
	t := c.Theme()

	glyph, tint := iconDownload, t.Accent
	if x.Kind == store.TransferKind("upload") {
		glyph, tint = iconUpload, t.Success
	}

	ui.Row(c.Key(x.ID)).FillWidth().Padding(6, 6).Gap(9).AlignItems(ui.Center).
		Radius(7).Children(func() {
		ui.Icon(c, glyph).FontSize(14).TextColor(tint).Shrink(0)

		ui.Column(c).Grow(1).Gap(3).Children(func() {
			ui.Text(c, x.Name).FontSize(typeBody).SingleLine().Ellipsis("…")
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, transferState(x)).FontSize(typeCaption).TextColor(t.TextMuted).SingleLine()
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
			cancel := ctlIconButtonBare(c, "Cancel "+x.Name)
			cancel.Children(func() { ui.Icon(c, iconClose).FontSize(12).TextColor(t.TextMuted) })
			if cancel.Clicked() {
				a.cancelTransfer(x.ID)
			}
		} else {
			ui.Icon(c, transferGlyph(x)).FontSize(12).TextColor(transferTint(x, t))
		}
	})
}

// transferState names where a transfer stands, in words.
func transferState(x store.Transfer) string {
	switch x.State {
	case store.StateQueued:
		return "Waiting"
	case store.StateRunning:
		return "Transferring"
	case store.StateDone:
		return "Done"
	case store.StateFailed:
		return "Failed"
	case store.StateCanceled:
		return "Canceled"
	}
	return string(x.State)
}

func transferGlyph(x store.Transfer) *ui.SVG {
	switch x.State {
	case store.StateDone:
		return iconCheck
	case store.StateFailed, store.StateCanceled:
		return iconAlert
	}
	return iconClock
}

func transferTint(x store.Transfer, t *ui.Theme) ui.Color {
	switch x.State {
	case store.StateDone:
		return t.Success
	case store.StateFailed, store.StateCanceled:
		return t.Danger
	}
	return t.TextMuted
}
