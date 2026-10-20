package main

import (
	"os"
	"strings"

	"github.com/egoist/mygo/ui"

	"bucketeer/internal/cli"
)

// The Command line page: the `bkt` link the app installs, the skill it
// carries for coding agents, and the commands themselves, so a person can
// read what the tool does without opening a terminal first.

// cliState is what the page shows, read when it opens and after an action.
type cliState struct {
	// LinkExists is true when the `bkt` link is on disk, and LinkCurrent
	// when it points at this build.
	LinkExists  bool
	LinkCurrent bool
	// LinkPath is where the link goes, and LinkDirOnPath whether the
	// shell will find it.
	LinkPath      string
	LinkDirOnPath bool
	// Targets are the agents the skill can be installed for.
	Targets []cli.TargetStatus
	// Err is why the last action failed.
	Err string
	// Notice is what the last action did.
	Notice string
}

// refreshCLIState reads the link and the installed skills, as they are for
// this build.
func (a *App) refreshCLIState() { a.refreshCLIStateAt(exePath()) }

// refreshCLIStateAt is refreshCLIState for a named program: the link and the
// skills count as current when they point at it.
func (a *App) refreshCLIStateAt(program string) {
	state := cliState{
		LinkPath:      cliLinkPath(),
		LinkDirOnPath: onPath(cliLinkDir()),
		Targets:       cli.Statuses(program),
	}
	state.LinkExists, state.LinkCurrent = cliLinkPresentAt(program)
	state.Err, state.Notice = a.cli.Err, a.cli.Notice
	a.cli = state
}

// installCLI makes the `bkt` link and installs the skill wherever an agent
// is set up.
func (a *App) installCLI() { go a.installCLIAt(exePath()) }

// installCLIAt is installCLI for a named program, so a test can install
// without pointing PATH at a temporary build.
func (a *App) installCLIAt(program string) {
	link, err := installLink(program)
	if err != nil {
		a.cli.Err = "Could not create " + cliLinkPath() + ": " + err.Error()
		a.refreshCLIStateAt(program)
		return
	}
	installed := 0
	for _, status := range cli.Statuses(program) {
		if !status.Available || status.Current {
			continue
		}
		if err := cli.Install(status.Target, program); err != nil {
			continue
		}
		installed++
	}
	if !a.cli.LinkDirOnPath {
		a.cli.Notice = "Linked " + link + ". Add " + cliLinkDir() + " to PATH, in " + platformShell() + "."
	} else if installed > 0 {
		a.cli.Notice = "Installed " + plural(installed, "skill", "skills") + "."
	} else {
		a.cli.Notice = "Linked " + link + "."
	}
	a.cli.Err = ""
	a.refreshCLIStateAt(program)
}

// removeCLI deletes the link and the installed skills.
func (a *App) removeCLI() { go a.removeCLIAt(exePath()) }

// removeCLIAt is removeCLI for a named program.
func (a *App) removeCLIAt(program string) {
	if err := removeLink(program); err != nil {
		a.cli.Err = err.Error()
		a.refreshCLIState()
		return
	}
	for _, status := range cli.Statuses(program) {
		if status.Installed {
			_ = cli.Uninstall(status.Target)
		}
	}
	a.cli.Notice = "Removed the command line link and the installed skills."
	a.cli.Err = ""
	a.refreshCLIStateAt(program)
}

// installSkill writes the bundle for one agent.
func (a *App) installSkill(target cli.Target) {
	go func() {
		err := cli.Install(target, exePath())
		a.update(func() {
			if err != nil {
				a.cli.Err = "Could not install for " + target.Name + ": " + err.Error()
			} else {
				a.cli.Notice = "Installed the skill for " + target.Name + "."
			}
			a.refreshCLIState()
		})
	}()
}

// removeSkill deletes the bundle for one agent.
func (a *App) removeSkill(target cli.Target) {
	go func() {
		err := cli.Uninstall(target)
		a.update(func() {
			if err != nil {
				a.cli.Err = err.Error()
			} else {
				a.cli.Notice = "Removed the skill from " + target.Name + "."
			}
			a.refreshCLIState()
		})
	}()
}

// copyCLICommand puts a command on the clipboard, ready to paste into a
// terminal.
func (a *App) copyCLICommand(command, what string) {
	path := exePath()
	if strings.ContainsAny(path, " \t") {
		path = `"` + path + `"`
	}
	a.services.WriteClipboard(path + " " + command)
	a.notice = what + " copied"
}

/* ---- The page ---- */

// settingsCLI is the Command line page.
func (a *App) settingsCLI(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).FillWidth().Gap(18).Children(func() {
		settingsHeader(c, "Command line", "A terminal tool and a skill for coding agents, working on the buckets this app already knows.")

		if a.cli.Err != "" {
			statusBanner(c, t.Danger.Alpha(0.08), iconAlert, a.cli.Err, t.Text, t.Danger)
		}
		if a.cli.Notice != "" && a.cli.Err == "" {
			statusBanner(c, t.Success.Alpha(0.08), iconCheck, a.cli.Notice, t.Text, t.Success)
		}

		a.cliToolCard(c)
		a.cliSkillsCard(c)
		a.cliCommandsCard(c)
	})
}

// cliToolCard is the terminal tool: whether it is installed, where it
// lives, and the one action that changes that.
func (a *App) cliToolCard(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, "Terminal tool").FontSize(typeBody).TextColor(t.TextMuted)
		group(c, func() {
			a.settingRow(c, "bkt command", "A link to this app in "+shortDir(cliLinkDir())+", so a terminal can reach your buckets.", func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					switch {
					case a.cli.LinkCurrent:
						ctlBadge(c, "Installed", t.Success)
					case a.cli.LinkExists:
						ctlBadge(c, "Out of date", t.Warning)
					default:
						ctlBadge(c, "Not installed", t.TextMuted)
					}
					label := "Install"
					if a.cli.LinkExists {
						if a.cli.LinkCurrent {
							if ctlButton(c, "Remove").Clicked() {
								a.ask("Remove the bkt command?",
									"This deletes the link and the skills installed for coding agents. Nothing in your buckets changes.",
									"Remove", a.removeCLI)
							}
							return
						}
						label = "Update"
					}
					if ctlPrimary(c, label).Clicked() {
						a.installCLI()
					}
				})
			})
			settingRule(c)
			a.settingRow(c, "Link", shortDir(a.cli.LinkPath), func() {
				if ctlButton(c, "Copy").Clicked() {
					a.copyText(a.cli.LinkPath, "Path copied")
				}
			})

			if a.cli.LinkExists && !a.cli.LinkDirOnPath {
				settingRule(c)
				ui.Row(c).FillWidth().Gap(8).Padding(8, 2).AlignItems(ui.Start).Children(func() {
					ui.Icon(c, iconInfo).FontSize(13).TextColor(t.Warning).Shrink(0).Margin(1, 0)
					ui.Text(c, "Your terminal does not search "+shortDir(cliLinkDir())+" yet. To add it to PATH, "+pathHint()+".").
						FontSize(typeCaption).
						TextColor(t.TextMuted).
						Grow(1)
				})
			}
		})
	})
}

// cliSkillsCard lists the agents the skill can go to, each with its own
// state and action.
func (a *App) cliSkillsCard(c *ui.Context) {
	t := c.Theme()

	available := 0
	for _, status := range a.cli.Targets {
		if status.Available {
			available++
		}
	}

	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, "Skill for coding agents").FontSize(typeBody).TextColor(t.TextMuted)
		group(c, func() {
			ui.Column(c).FillWidth().Padding(7, 2).Gap(1).Children(func() {
				ui.Text(c, "Teaches an agent to browse, upload, search and clean up your buckets with bkt, using the connections saved here.").
					FontSize(typeCaption).
					TextColor(t.TextMuted)
			})
			for _, s := range a.cli.Targets {
				status := s
				if !status.Available {
					continue
				}
				settingRule(c)
				a.settingRow(c, status.Name, shortDir(status.SkillsDir()), func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						switch {
						case status.Installed && status.Current:
							ctlBadge(c, "Installed", t.Success)
						case status.Installed:
							ctlBadge(c, "Out of date", t.Warning)
						}
						label := "Install"
						if status.Installed {
							label = "Update"
						}
						if ctlButton(c, label).Clicked() {
							a.installSkill(status.Target)
						}
						if status.Installed {
							remove := ctlIconButtonBare(c, "Remove skill for "+status.Name)
							remove.Children(func() { ui.Icon(c, iconTrash).FontSize(13).TextColor(t.TextMuted) })
							if remove.Clicked() {
								a.removeSkill(status.Target)
							}
						}
					})
				})
			}
			if available == 0 {
				settingRule(c)
				ui.Column(c).FillWidth().Padding(7, 2).Gap(2).Children(func() {
					ui.Text(c, "No agent skills folder was found.").FontSize(typeBody).TextColor(t.Text)
					ui.Text(c, "Create "+shortDir(cli.Targets[0].Dir)+" and this list fills in.").
						FontSize(typeCaption).
						TextColor(t.TextMuted)
				})
			}
			settingRule(c)
			ui.Row(c).FillWidth().Gap(8).Padding(8, 2).AlignItems(ui.Center).Children(func() {
				if ctlButton(c, "Copy SKILL.md").Clicked() {
					a.services.WriteClipboard(cli.EntryMarkdown(exePath()))
					a.notice = "SKILL.md copied"
				}
				if ctlButton(c, "Copy install command").Clicked() {
					a.copyCLICommand("skills install", "Install command")
				}
				ui.Spacer(c)
			})
		})
	})
}

// cliCommandsCard is the reference: each command with a sample, ready to
// copy. It is a compact table, because the point is to be read at a glance
// and then pasted into a terminal.
func (a *App) cliCommandsCard(c *ui.Context) {
	t := c.Theme()

	rows := cli.CommandRows()
	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, "Commands").FontSize(typeBody).TextColor(t.TextMuted)
		group(c, func() {
			for i, row := range rows {
				entry := row
				if i > 0 {
					settingRule(c)
				}
				line := ui.Row(c).FillWidth().Padding(5, 2).Gap(12).AlignItems(ui.Center)
				line.Children(func() {
					ui.Text(c, entry.Cmd).
						FontSize(typeCaption).
						Font("monospace").
						TextColor(t.Text).
						Width(96).
						Shrink(0).
						SingleLine()
					ui.Text(c, entry.Does).
						FontSize(typeCaption).
						TextColor(t.TextMuted).
						Grow(1).
						SingleLine().
						Ellipsis("…")
					copy := ctlIconButtonBare(c, "Copy "+entry.Sample)
					copy.Children(func() { ui.Icon(c, iconCopy).FontSize(12).TextColor(t.TextMuted) })
					if copy.Clicked() {
						a.services.WriteClipboard(entry.Sample)
						a.notice = entry.Sample + " copied"
					}
				})
				line.Hovered()
			}
		})
	})
}

// shortDir replaces the home directory with a tilde, so a path fits on a
// settings row.
func shortDir(dir string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rest, ok := strings.CutPrefix(dir, home); ok {
			if rest == "" {
				return "~"
			}
			return "~" + rest
		}
	}
	return dir
}
