package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/egoist/mygo"
)

// MenuBarTarget is independent of the currently browsed connection and folder.
type MenuBarTarget struct {
	ProfileID string `json:"profileId"`
	Bucket    string `json:"bucket"`
}

func (s *Service) UploadMenuBarPaths(ctx context.Context, target MenuBarTarget, paths []string) (int, error) {
	if target.ProfileID == "" || target.Bucket == "" {
		return 0, errors.New("open a bucket, then choose Use Current Bucket as Default in the menu bar")
	}
	// Open a separate session; never change the browser's active connection.
	session, err := s.profiles.Open(ctx, target.ProfileID)
	if err != nil {
		return 0, fmt.Errorf("open default upload connection: %w", err)
	}
	return s.uploadPathsTo(session, target.Bucket, "", paths)
}

func (a *App) menuBarAction(action int, paths []string) {
	switch action {
	case 0:
		a.win.Show()
		a.win.Focus()
	case 1:
		if !a.session.Connected || a.session.Profile == nil || a.bucket == "" {
			a.notice = "Open a bucket before choosing the default upload destination."
			a.win.Show()
			return
		}
		settings := a.svc.GetSettings()
		settings.MenuBarUpload = MenuBarTarget{ProfileID: a.session.Profile.ID, Bucket: a.bucket}
		if err := a.svc.SetSettings(settings); err != nil {
			a.fail(err)
			return
		}
		a.notice = "Menu bar uploads go to " + a.bucket + "/"
	case 2:
		mygo.App.Quit()
	case 3:
		target := a.svc.GetSettings().MenuBarUpload
		if target.ProfileID == "" || target.Bucket == "" {
			a.notice = "Choose Use Current Bucket as Default in the menu bar before dropping files."
			a.win.Show()
			return
		}
		paths = append([]string(nil), paths...)
		go func() {
			queued, err := a.svc.UploadMenuBarPaths(a.ctx, target, paths)
			a.update(func() {
				if queued > 0 {
					a.transfersOpen = true
					a.notice = fmt.Sprintf("Queued %d uploads to %s/", queued, target.Bucket)
				}
				if err != nil {
					a.fail(err)
					a.win.Show()
				}
			})
		}()
	}
}
