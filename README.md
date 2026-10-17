# Bucketeer

A cross-platform desktop client for S3-compatible object storage, built with
[MyGo](https://github.com/egoist/mygo) using 100% **Native UI** (`github.com/egoist/mygo/ui`).
No web view, no TypeScript, no Node/Bun runtime dependencies.

Browse buckets like folders, upload and download with live progress, preview
files, measure storage, and share them with expiring links.

Works with Cloudflare R2, Amazon S3, MinIO, Backblaze B2, Wasabi,
DigitalOcean Spaces, and any other endpoint that speaks the S3 API.

## Building and Running

The project is pure Go. Only Go 1.24+ and the MyGo CLI are required.

```sh
# Run locally in development
go run .

# Package native macOS app and DMG
go run github.com/egoist/mygo/cmd/mygo@v0.5.5 build
```

This builds a standalone binary of about 12.3 MB (and a 3.9 MB DMG) in `build/darwin-arm64/`.

## What it does

**Browse.** A bucket is a folder: grid and list views, breadcrumbs, search,
sorting by name, size or date, filtering by kind, and multi-select with the
usual click, shift-click and command-click conventions. A folder is an
object prefix, so folders held as zero-byte marker objects show up too.

**Transfer.** Uploads and downloads run a few at a time with per-file
progress, and large files go through multipart uploads with a part size that
scales with the file. The queue is FIFO, so work starts in the order it was
asked for, and each transfer can be canceled. Files can be dropped directly on
the window to upload them.

**Preview.** Images decode directly via native bitmaps (`ui.Bitmap`), and text /
source files preview inside the app with syntax-appropriate scroll areas.
Anything the app cannot draw directly opens in the system browser via temporary
presigned URLs.

**Organize.** Create folders, rename, move, and delete. Deleting a folder
removes everything under it.

**Share.** Generate a presigned link that expires (1h, 6h, 12h, 1d, 3d, 7d),
and open it or copy it.

**Measure.** The dashboard sums a bucket's size and object count, breaks the
space down by top-level folder and storage class, and lists the largest
objects.

## Where credentials live

Access keys go to the system credential store — the login Keychain on
macOS, the Secret Service on Linux, the Credential Manager on Windows.
`connections.json` holds only non-sensitive profile metadata, and the
settings file holds preferences. Neither contains a secret access key, and
the files are written with owner-only permissions.

The app reports when a system has no credential store rather than silently
falling back to a file.

## The window's title bar

The window hides its title bar (`TitleBarHiddenInset`), so macOS draws the
traffic lights over the view. MyGo reports the room they take in
`c.TitleBar()` (`Height`, `Left`, `Right`), and the native layout is built from
those metrics: the sidebar reserves the title bar's strip as a window drag
area (`DragWindow()`), and the toolbar clears the window controls and handles
fullscreen switches cleanly.

## Layout

```
main.go                     native window setup, application menus, lifecycle
app.go                      central application state and background worker synchronization
service.go                  storage client integration and credential handling
actions.go                  upload/download, CRUD, sharing and transfer operations
theme.go                    light and dark palette definitions for MyGo ui.Theme
icons.go                    reusable embedded SVG vector icons
format.go                   byte sizes, relative dates, file classifications
preview.go                  image decoding and dimension safety guards
view.go                     main view layout and sidebar
view_toolbar.go             toolbar, breadcrumbs, search, drop target, and empty states
view_files.go               grid view, table view, and bucket list
view_panels.go              dashboard storage analytics, settings, and transfers panel
dialogs.go                  modals for connections, new folders, renaming, moving, and sharing
internal/store/
  profiles.go               connection profiles and secret store abstraction
  s3.go                     S3 and MinIO operations
  transfers.go              transfer queue manager and worker pool
  files.go                  local file helper utilities
```

## Tests

```sh
# Offline tests (profiles, transfers, formatters, and native UI tester)
go test -v ./...

# Generate native screenshot previews
PAIL_SHOTS=preview/shots go test -run TestGenerateShots -v .

# Integration tests with a local S3 endpoint:
go install github.com/johannesboyne/gofakes3/cmd/gofakes3@latest
gofakes3 -backend=bolt -bolt.db=/tmp/fakes3.db -host=:9111 &

S3_ENDPOINT=http://127.0.0.1:9111 S3_KEY=testkey S3_SECRET=testsecret \
  go test ./... -race
```

## Keyboard Shortcuts

| | |
|---|---|
| `⌘O` | Upload files |
| `⌘⇧N` | New folder |
| `⌘R` | Refresh |
| `⌘T` | Transfers panel |
| `⌘A` | Select all |
| `Enter` | Open folder / detail |
| `Esc` | Clear selection / close modal |

`⌘` maps to `Ctrl` on Linux and Windows.
