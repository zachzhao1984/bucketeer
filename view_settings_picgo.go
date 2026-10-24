package main

import (
	"strconv"

	"github.com/egoist/mygo/ui"
)

// The Image hosting page: the local upload service that notebooks and
// editors paste pictures through, speaking the protocol PicGo's clients
// already know, so an existing plugin needs only a URL.

// cliStateForPicGo is the page's state: the service as it is running, and
// what the last action did.
type picgoPageState struct {
	state   PicGoState
	profile string // the chosen connection's name, for the row
	err     string
	notice  string
	// publicBase is what the base-URL field shows.
	publicBase string
	// rotate confirms the destructive action.
	confirmRotate bool
}

// refreshPicGoState reads the service and the connection it points at.
func (a *App) refreshPicGoState() {
	state := a.svc.PicGoStatus()
	profile := ""
	if state.Config.ProfileID != "" {
		for _, p := range a.connections {
			if p.ID == state.Config.ProfileID {
				profile = p.Name
				break
			}
		}
	} else if a.session.Profile != nil {
		profile = a.session.Profile.Name + " (connected)"
	}
	// The page keeps what the last action said, until the next one.
	err, notice := a.picgoPage.err, a.picgoPage.notice
	if state.Err != "" {
		// The service's own error wins: it is the fresher one.
		err, notice = state.Err, ""
	}
	page := picgoPageState{
		state:   state,
		profile: profile,
		err:     err,
		notice:  notice,
	}
	if state.Config.PublicBases == nil {
		state.Config.PublicBases = map[string]string{}
	}
	page.publicBase = state.Config.PublicBases[state.Config.ProfileID]
	a.picgoPage = page
}

// setPicGoEnabled turns the service on or off, which starts or stops the
// listener.
func (a *App) setPicGoEnabled(on bool) {
	config := a.picgoPage.state.Config
	config.Enabled = on
	a.applyPicGoConfig(config)
}

// setPicGoPort applies a port from the field.
func (a *App) setPicGoPort(port int) {
	config := a.picgoPage.state.Config
	if config.Port == port {
		return
	}
	config.Port = port
	a.applyPicGoConfig(config)
}

// setPicGoPrefix applies a prefix from the field.
func (a *App) setPicGoPrefix(prefix string) {
	config := a.picgoPage.state.Config
	if config.Prefix == prefix {
		return
	}
	config.Prefix = prefix
	a.applyPicGoConfig(config)
}

// setPicGoProfile applies the chosen connection.
func (a *App) setPicGoProfile(id string) {
	config := a.picgoPage.state.Config
	if config.ProfileID == id {
		return
	}
	config.ProfileID = id
	a.applyPicGoConfig(config)
}

// applyPicGoConfig saves and restarts the service.
func (a *App) applyPicGoConfig(config PicGoConfig) {
	state := a.svc.SavePicGoConfig(config)
	a.picgoPage.state = state
	a.picgoPage.err = state.Err
	if state.Err == "" {
		if state.URL != "" {
			a.picgoPage.notice = "The upload service is running."
		} else {
			a.picgoPage.notice = "The upload service is off."
		}
	}
	a.refreshPicGoState()
}

// applyPublicBase records the connection's public base URL, or clears it.
func (a *App) applyPublicBase(base string) {
	if a.picgoPage.state.Config.ProfileID == "" {
		return
	}
	if _, err := a.svc.SetPublicBase(a.picgoPage.state.Config.ProfileID, base); err != nil {
		a.picgoPage.err = err.Error()
		a.refreshPicGoState()
		return
	}
	a.picgoPage.notice = "The public base URL was saved."
	a.refreshPicGoState()
}

// copyPicGoURL puts the upload URL on the clipboard: the one thing a plugin
// needs.
func (a *App) copyPicGoURL() {
	if url := a.picgoPage.state.URL; url != "" {
		a.services.WriteClipboard(url)
		a.notice = "Upload URL copied"
	}
}

// rotatePicGoToken replaces the secret and restarts the service with it,
// which invalidates every URL copied so far.
func (a *App) rotatePicGoToken() {
	token, err := a.svc.RotatePicGoToken()
	a.picgoPage.confirmRotate = false
	if err != nil {
		a.picgoPage.err = err.Error()
	} else {
		a.picgoPage.notice = "A new secret was made; URLs copied before no longer work."
		a.picgoPage.state.Token = token
	}
	a.refreshPicGoState()
}

/* ---- The page ---- */

// picgoField is a text field of this page, bound to a value and applied
// when the person is done editing it.
func (a *App) picgoField(c *ui.Context, label, hint, placeholder string, value *string, apply func(string)) {
	t := c.Theme()
	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(typeBody).TextColor(t.Text).Grow(1)
			if hint != "" {
				ui.Text(c, hint).FontSize(typeCaption).TextColor(t.TextMuted)
			}
		})
		field := ui.TextInputBase(c.Key(label), value).Placeholder(placeholder).FontSize(typeBody)
		field.OnSubmit(func() {
			apply(*value)
		})
		ctlField(c, field).FillWidth()
		_ = t
	})
}

// settingsPicGo is the Image hosting page.
func (a *App) settingsPicGo(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).FillWidth().Gap(18).Children(func() {
		settingsHeader(c, "Image hosting", "A local upload service for Obsidian, Typora and anything that speaks PicGo's protocol. Pictures pasted in a note land in your bucket, and the note gets the link.")

		if a.picgoPage.err != "" {
			statusBanner(c, t.Danger.Alpha(0.08), iconAlert, a.picgoPage.err, t.Text, t.Danger)
		}
		if a.picgoPage.notice != "" && a.picgoPage.err == "" {
			statusBanner(c, t.Success.Alpha(0.08), iconCheck, a.picgoPage.notice, t.Text, t.Success)
		}

		a.picGoServiceCard(c)
		a.picGoSetupCard(c)
	})
}

// picGoServiceCard is the switch, the port, the prefix, and the URL a
// client pastes.
func (a *App) picGoServiceCard(c *ui.Context) {
	t := c.Theme()
	state := a.picgoPage.state

	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, "Service").FontSize(typeBody).TextColor(t.TextMuted)
		group(c, func() {
			a.settingRow(c, "Enable the upload service", "Listens on this machine only, at 127.0.0.1.", func() {
				ctlSwitch(c, "picgo-enabled", &state.Config.Enabled, "Enable the upload service").OnChange(func() {
					a.setPicGoEnabled(state.Config.Enabled)
				})
			})
			settingRule(c)
			a.settingRow(c, "Port", "The port to listen on; 36677 is what PicGo clients expect.", func() {
				ui.Text(c, itoa(state.Config.Port)).FontSize(typeBody).TextColor(t.TextMuted)
			})
			settingRule(c)
			a.picgoField(c, "Upload into", "a folder in the bucket, empty for its root", "as in notes/pictures",
				&a.picgoPage.state.Config.Prefix, a.setPicGoPrefix)
			settingRule(c)
			a.settingRow(c, "Connection", "The bucket the pictures go to. Follow the connection you are browsing when empty.", func() {
				a.picGoProfileMenu(c)
			})

			if state.Err != "" {
				settingRule(c)
				ui.Row(c).FillWidth().Gap(8).Padding(8, 2).AlignItems(ui.Start).Children(func() {
					ui.Icon(c, iconAlert).FontSize(13).TextColor(t.Danger).Shrink(0).Margin(1, 0)
					ui.Text(c, state.Err).FontSize(typeCaption).TextColor(t.Text).Grow(1)
				})
			}
		})
	})
}

// picGoProfileMenu is the connection the uploads go through.
func (a *App) picGoProfileMenu(c *ui.Context) {
	chosen := a.picgoPage.profile
	if chosen == "" {
		chosen = "Follow the connected one"
	}
	btn := ctlMenuButton(c, "Connection", chosen, a.picgoProfileOpen, 220)
	if btn.Clicked() {
		a.picgoProfileOpen = !a.picgoProfileOpen
	}
	ui.Popover(c, btn, &a.picgoProfileOpen, func() {
		ctlMenuPanel(c, 260).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				if menuItem(c, "Follow the connected one", "", a.picgoPage.state.Config.ProfileID == "", "") {
					a.picgoProfileOpen = false
					a.setPicGoProfile("")
				}
				for _, p := range a.connections {
					profile := p
					if menuItem(c, profile.Name, "", a.picgoPage.state.Config.ProfileID == profile.ID, profile.Bucket) {
						a.picgoProfileOpen = false
						a.setPicGoProfile(profile.ID)
					}
				}
			})
		})
	})
}

// picGoSetupCard is the URL, its secret, and how to point a plugin at it.
func (a *App) picGoSetupCard(c *ui.Context) {
	t := c.Theme()
	state := a.picgoPage.state

	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, "Use it from a notebook").FontSize(typeBody).TextColor(t.TextMuted)
		group(c, func() {
			a.settingRow(c, "Upload URL", "Paste this as the custom uploader's address.", func() {
				if state.URL == "" {
					ui.Text(c, "The service is off").FontSize(typeBody).TextColor(t.TextMuted)
					return
				}
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					copy := ctlIconButtonBare(c, "Copy the upload URL")
					copy.Children(func() { ui.Icon(c, iconCopy).FontSize(13).TextColor(t.TextMuted) })
					if copy.Clicked() {
						a.copyPicGoURL()
					}
					if ctlButton(c, "Copy").Clicked() {
						a.copyPicGoURL()
					}
				})
			})
			if state.URL != "" {
				settingRule(c)
				ui.Column(c).FillWidth().Padding(6, 2).Gap(4).Children(func() {
					ui.Text(c, state.URL).
						FontSize(typeCaption).
						Font("monospace").
						TextColor(t.Text).
						FillWidth()
					ui.Text(c, "The secret in the URL keeps other programs on this machine from uploading. Replace it with a new one when it has been shared somewhere it should not have been.").
						FontSize(typeCaption).
						TextColor(t.TextMuted)
				})
			}
			settingRule(c)
			ui.Row(c).FillWidth().Padding(8, 2).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Gap(1).Children(func() {
					ui.Text(c, "New secret").FontSize(typeBody).TextColor(t.Text)
					ui.Text(c, "Every plugin has to be set up again with the new URL.").FontSize(typeCaption).TextColor(t.TextMuted)
				})
				if a.picgoPage.confirmRotate {
					if ctlDanger(c, "Confirm").Clicked() {
						a.rotatePicGoToken()
					}
					if ctlButton(c, "Keep").Clicked() {
						a.picgoPage.confirmRotate = false
					}
				} else if ctlButton(c, "Replace").Clicked() {
					a.picgoPage.confirmRotate = true
				}
			})
			settingRule(c)
			if a.picgoPage.state.Config.ProfileID == "" {
				a.settingRow(c, "Public base URL", "Choose a connection first; a public base belongs to one.", func() {
					ui.Text(c, "—").FontSize(typeBody).TextColor(t.TextMuted)
				})
			} else {
				a.picgoField(c, "Public base URL", "https://cdn.example.com", "empty for signed links that expire",
					&a.picgoPage.publicBase, a.applyPublicBase)
			}
		})

		// How to point each editor at the service, in the words their
		// plugins use.
		ui.Column(c).FillWidth().Padding(10, 2).Gap(8).Children(func() {
			a.picGoHint(c, "Typora", "Preferences → Image → Upload service: Custom, and paste the URL as the command's argument area accepts it.")
			a.picGoHint(c, "Obsidian", "The Image auto upload plugin: default uploader “PicGo (app)”, and the URL above as the api address.")
			a.picGoHint(c, "Anything else", "POST an image as multipart form-data with the field files, and read {\"success\": true, \"result\": [url]}.")
		})
	})
}

// picGoHint is one editor's setup, named after it.
func (a *App) picGoHint(c *ui.Context, name, detail string) {
	t := c.Theme()
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Start).Children(func() {
		ui.Text(c, name).
			FontSize(typeCaption).
			FontWeight(weightMedium).
			TextColor(t.Text).
			Width(90).
			Shrink(0)
		ui.Text(c, detail).
			FontSize(typeCaption).
			TextColor(t.TextMuted).
			Grow(1)
	})
}

// prefixOrPlaceholder is what the prefix field shows before typing.
func prefixOrPlaceholder(prefix string) string {
	if prefix == "" {
		return "the bucket's root"
	}
	return prefix
}

// itoa is strconv.Itoa, spelled the way this file's neighbors read.
func itoa(n int) string { return strconv.Itoa(n) }
