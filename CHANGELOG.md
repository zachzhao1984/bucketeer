# Changelog

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
