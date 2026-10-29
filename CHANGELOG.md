# Changelog

## Unreleased

- The Settings entry is in the connection menu, alongside connection actions.

- Settings opens as a centered modal over the main window, preserving the
  browser's folder and selection. Close it with Escape, the close button, or
  the backdrop. Preferences includes a persistent macOS menu bar icon switch
  that takes effect immediately.

- The menu bar icon derives from the app's bucket-and-cards artwork, rendered
  as a monochrome template that follows the menu bar appearance.

- Menu bar uploads also work with MyGo's default CGO-free dev and release
  builds; the native bridge uses purego/objc instead of a CGO-only implementation.
- macOS menu bar uploads: drop files, folders, or PNG/TIFF images onto the
  icon to upload to a saved default connection and bucket. Set the destination
  with Use Current Bucket as Default; uploads keep running when the window
  closes and appear in the existing transfer list.

## 1.4.0

- MyGo 0.7.2, with the toolchain it needs (Go 1.27.3). 0.7 names the
  Windows installer `<slug>-setup-<version>-<arch>.exe` and disk images
  after the app's slug, so the release workflow's globs follow.
- The first-run page is a board of the services the app speaks: the app's
  own icon, one tile per service with its brand mark, and a tile click
  opens the form already set to that service. The three feature lines are
  gone; a custom endpoint is a line under the board.
- Brand marks keep their own colors again in the connection list and the
  sidebar; they had been drawn as single-color silhouettes.

## 1.3.1

- Refined app icon with a transparent background, shown at the top of the
  README.

## 1.3.0

- Remove system credential-store access and migration from the app and CLI.
  Profile keys and the PicGo upload token use only the local credential file.
- Remove bundled test-account loading and automatic R2 connection seeding.
  New installations start without connections.
- Restrict packaged resources to public app icons; private local test
  configuration stays outside the resources directory.
- Includes the new blue bucket app icon and signed in-app updates for
  macOS, Windows, and Linux. Check for updates from the app menu or
  Settings → About.

## 1.2.0

- New app icon: a blue bucket of files with the Cloudflare cloud mark.
- Check for updates from the app menu or Settings → About. Updates use
  signed downloads, show release notes, and offer to restart when ready.
- Future releases can download only the changes from an installed version,
  with a full-download fallback. This is the first release with the updater;
  install it manually once to receive future updates in the app.

## 1.1.1

- Connections show each service's own mark — Cloudflare, AWS S3, Backblaze,
  Wasabi, MinIO, Alibaba Cloud, Tencent Cloud, DigitalOcean Spaces — drawn
  from theSVG (https://thesvg.org); an unknown service keeps the drive
  plate with its initials.

## 1.1.0

- **Linux builds**: a `tar.gz` with `install.sh` (installs to `~/.local`,
  no root) and a Debian package, for amd64 and arm64.
- Credentials moved out of the system keychain into `secrets.json` in the
  app's data folder (mode 0600). An ad-hoc signed build cannot make the
  keychain's per-item authorization stick, so every launch asked twice;
  the file asks nothing. Credentials an older build kept in the keychain
  are imported on first launch — one prompt per key that is there — and
  removed from it.
- The sidebar's Folders list stays on the bucket's own folders while the
  right side moves through them; the root folder that holds the browsed
  prefix stays highlighted.
- Folders in the grid draw as the file card's anatomy — a plate with a
  large line glyph and the name below — selected the same way files are.
- The Midnight appearance in both light and dark, with the transfers
  light-rail along the window's bottom edge.
- The app menu carries Settings… (⌘,), the appearance choice is
  Auto/Light/Dark, ⌘F focuses the filter, Space previews.
- Fixed: the delete confirmation's Cancel did nothing; the startup
  listing could hang on the loading skeletons forever.

## 0.1.0

The first release.

- Browse S3-compatible buckets as a grid of cards or a table: Cloudflare R2,
  Amazon S3, Backblaze B2, Wasabi, MinIO, Alibaba Cloud OSS, Tencent Cloud
  COS, DigitalOcean Spaces, and any other endpoint that speaks the S3 API.
- Upload files and whole folders, download objects and prefix trees, with
  live progress in a transfers dock.
- A dashboard of what a bucket holds, by kind, class and top-level folder.
- Preview pictures and text in place; share any object with a presigned
  link that expires.
- Connections keep their access keys on disk (owner-only), and the app starts
  on the Cloudflare R2 bucket it ships configured for.
- A `bkt` command line for the same connections, and a skill that teaches a
  coding agent to use it. Both install from Settings → Command line.
