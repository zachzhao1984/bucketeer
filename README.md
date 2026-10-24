# Bucketeer

A cross-platform desktop client for S3-compatible object storage, built with
[MyGo](https://github.com/egoist/mygo) using 100% **Native UI** (`github.com/egoist/mygo/ui`).
No web view, no TypeScript, no Node/Bun runtime dependencies.

Browse buckets like folders, upload and download with live progress, preview
files, measure storage, and share them with expiring links.

Works with Cloudflare R2, Amazon S3, MinIO, Backblaze B2, Wasabi, Alibaba
Cloud OSS, Tencent Cloud COS, DigitalOcean Spaces, and any other endpoint
that speaks the S3 API.

## Download

Releases carry a universal macOS disk image and a Windows installer:

- **macOS 12 or later**, Apple silicon and Intel: `Bucketeer-<version>-macos-universal.dmg`
- **Windows 10 or later**: `Bucketeer-<version>-windows-amd64.exe`, which
  installs for the current user, with no administrator prompt

## Image hosting for notebooks

Settings → Image hosting starts a small upload service on this machine that
speaks [PicGo's server protocol](https://docs.picgo.app/gui/guide/advance), so
Obsidian's and Typora's image-upload plugins work with your bucket without
another tool:

- **Enable the service**, choose the connection and an optional folder, and
  copy the URL it shows.
- In **Typora**: Preferences → Image → Upload service → Custom, with that URL.
- In **Obsidian**: the Image auto upload plugin, default uploader
  “PicGo (app)”, and the URL as the api address.

A picture pasted into a note lands in the bucket under
`<folder>/<year>/<month>/<name>.png`, the note gets the link, and nothing
leaves the machine but the image itself. Every request must carry the secret
in the URL, which the app generates and can replace; a connection with a
public base URL returns plain links, and one without returns signed links
that expire with the share-link setting.

The service listens on 127.0.0.1 only, accepts at most 20 images of up to
32 MiB per request, and keeps the uploads visible in the same transfer list
as the app's own.

## Building and Running

The project is pure Go. Only Go 1.27+ and the MyGo CLI are required.

```sh
# Run locally in development
go run .

# Package the app for this platform: a .app and a .dmg on macOS, an .exe and
# an NSIS installer on Windows
go run github.com/egoist/mygo/cmd/mygo@v0.5.5 build

# Both macOS architectures in one bundle
go run github.com/egoist/mygo/cmd/mygo@v0.5.5 build -platform darwin/universal
```

## Releasing

`.github/workflows/release.yml` builds and publishes the app. Push a tag that
matches the version in `mygo.json`:

```sh
# bump "version" in mygo.json and add its section to CHANGELOG.md, commit, then
git tag v0.1.1 && git push origin v0.1.1
```

The workflow runs the tests, builds a universal macOS disk image on
`macos-14` and a Windows installer on `windows-latest`, smoke-tests each
installer's payload, and attaches the files to a GitHub release of the tag.
A manual run (`workflow_dispatch`) builds the same artifacts without
publishing, so a dry run is free.

Signing is optional, and comes from repository secrets:

| Secret | What it does |
| --- | --- |
| `MACOS_CERTIFICATE`, `MACOS_CERTIFICATE_PASSWORD` | A base64 Developer ID `.p12`, and its password. Without it the app is ad hoc signed, which runs on the machine that built it only. |
| `MACOS_SIGNING_IDENTITY` | The identity to sign with, as `Developer ID Application: Name (TEAMID)`. |
| `MACOS_NOTARY_APPLE_ID`, `MACOS_NOTARY_TEAM_ID`, `MACOS_NOTARY_PASSWORD` | Apple's notary credentials. With them, and an identity, the disk image is notarized and stapled. |
| `WINDOWS_CERTIFICATE`, `WINDOWS_CERTIFICATE_PASSWORD` | A base64 code signing `.pfx`, and its password, so SmartScreen does not warn on the installer. |

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
