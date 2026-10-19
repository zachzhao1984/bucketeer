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

**Browse.** The window follows the reference client: a sidebar of places
(Dashboard, All files, Recent files, the folders of the bucket being
browsed), a toolbar of the actions that apply where you are, and the files
themselves — as a grid of large kind glyphs and decoded image thumbnails,
or as a table with a name, a type, a size, a date, and a row menu.

**Navigate.** A bucket is a folder. Breadcrumbs walk back up it, the
sidebar lists the folders one level below where you are, and Recent files
keeps what you have opened, newest first, beside the newest objects the
bucket itself reports. Selection follows the platform: click, shift-click,
command-click, select-all, and the keyboard's arrows.

**Filter and sort.** The type filter and the sort field sit beside the
filter box, and both are menus rather than controls of their own: all
types, folders, images, video, audio, documents, archives, other; name,
size, or date modified, ascending or descending.

**Transfer.** Uploads and downloads run a few at a time with per-file
progress, and large files go through multipart uploads with a part size
that scales with the file. The queue floats in the corner it belongs in
and can be dismissed; each transfer can be canceled. Files can be dropped
on the window to upload them, and a whole folder can be uploaded under its
own name.

**Preview.** Opening a file shows one sheet: its name and its actions on
a bar, the preview on a recessed plate, and its facts in the footer.
Images decode through native bitmaps, text previews in place, and
anything the app cannot draw says so plainly and opens in the browser
through a temporary presigned URL.

**Organize.** Create folders and buckets, rename, move, and delete.
Deleting a folder removes everything under it, so the app asks first.

**Share.** Generate a presigned link that expires (1h, 6h, 12h, 1d, 3d,
7d), copy it, or open it.

**Measure.** The dashboard sums the bucket's size and object count, breaks
the space down by folder and by kind of file, and reports what changed in
the last week.

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

## The look

The window is two planes, as the reference client draws them. The sidebar
is a light gray (`#F7F7F7`) with the navigation in it, and the content
plane beside it is white, divided by one hairline. Controls standing on the
content are white plates with a hairline and a shadow faint enough to read
as depth; controls inside a toolbar row drop to a soft gray well. The one
accent — a strong blue (`#3163F4`) — fills the chosen place in the sidebar,
the one primary action on a screen, and the mark of the kind of file a card
shows.

Nothing in the app is a bevel. A surface is a fill and, where it needs
telling apart from what is next to it, a hairline; a real shadow is spent
only where something genuinely floats above the window — menus, popovers,
the transfers dock, and dialogs.

`theme.go` holds the tokens — surfaces, hairlines, washes, and two palettes,
one per appearance. `chrome.go` paints them and builds the controls from
them. Type is one scale: `typeCaption` for counts and hints, `typeBody` for
everything a person reads, `typeHeadline` for dialog and section titles,
`typeTitle` for the place being browsed, and `typeDisplay` for a figure on
the dashboard.

The window hides its title bar (`TitleBarHiddenInset`), so macOS draws the
traffic lights over the sidebar. MyGo reports the room they take in
`c.TitleBar()` (`Height`, `Left`, `Right`), and the sidebar reserves that
strip as a drag region, which is why the app's own name is not printed over
it.
