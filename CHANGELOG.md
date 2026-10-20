# Changelog

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
- Connections live in the system credential store, and the app starts on the
  Cloudflare R2 bucket it ships configured for.
- A `bkt` command line for the same connections, and a skill that teaches a
  coding agent to use it. Both install from Settings → Command line.
