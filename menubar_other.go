//go:build !darwin

package main

// Menu-bar drag and drop is provided by AppKit on macOS.
func startMenuBar(a *App) func() { return func() {} }
