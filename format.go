package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// formatBytes formats a byte count the way a file manager does, in
// decimal units: "412 MB", "1.1 GB".
func formatBytes(bytes int64) string {
	if bytes < 0 {
		return "—"
	}
	if bytes < 1000 {
		return fmt.Sprintf("%d B", bytes)
	}
	units := []string{"kB", "MB", "GB", "TB", "PB"}
	value := float64(bytes) / 1000
	unit := 0
	for value >= 1000 && unit < len(units)-1 {
		value /= 1000
		unit++
	}
	digits := 1
	if value >= 100 {
		digits = 0
	}
	return fmt.Sprintf("%.*f %s", digits, value, units[unit])
}

// formatCount formats a number with thousands separators.
func formatCount(n int) string {
	s := strconv.Itoa(n)
	negative := strings.HasPrefix(s, "-")
	if negative {
		s = s[1:]
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if negative {
		return "-" + b.String()
	}
	return b.String()
}

// formatDate formats a timestamp as a short date and time. The zero time,
// which is what an object without a timestamp has, reads as a dash.
func formatDate(t time.Time) string {
	if t.IsZero() || t.Year() < 1970 {
		return "—"
	}
	return t.Local().Format("2 Jan 2006 at 15:04")
}

// formatRelative says how long ago a timestamp was: "3 hours ago".
func formatRelative(t time.Time, now time.Time) string {
	if t.IsZero() || t.Year() < 1970 {
		return ""
	}
	seconds := now.Sub(t).Seconds()
	steps := []struct {
		unit string
		size float64
	}{
		{"second", 60},
		{"minute", 60},
		{"hour", 24},
		{"day", 30},
		{"month", 12},
		{"year", 0},
	}
	value := seconds
	for _, step := range steps {
		if step.size == 0 || value < step.size {
			return relativePhrase(value, step.unit)
		}
		value /= step.size
	}
	return relativePhrase(value, "year")
}

// relativePhrase writes "in 3 minutes" or "3 minutes ago", matching the
// rounding the reference client used.
func relativePhrase(value float64, unit string) string {
	n := int64(value + 0.5)
	past := true
	if value < 0 {
		n, past = -n, false
	}
	var text string
	switch n {
	case 0:
		text = "just now"
	case 1:
		text = "1 " + unit + " ago"
	default:
		text = fmt.Sprintf("%d %ss ago", n, unit)
	}
	if !past {
		if n == 1 {
			return "in 1 " + unit
		}
		return fmt.Sprintf("in %d %ss", n, unit)
	}
	return text
}

// plural writes a count with its noun: "1 folder", "5 folders".
func plural(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// summarizeListing describes a listing in one line the way the toolbar
// does: "5 folders · 6 files · 1.6 GB".
func summarizeListing(folders, files int, bytes int64) string {
	var parts []string
	if folders > 0 {
		parts = append(parts, plural(folders, "folder", "folders"))
	}
	if files > 0 {
		parts = append(parts, plural(files, "file", "files"))
	}
	if bytes > 0 {
		parts = append(parts, formatBytes(bytes))
	}
	if len(parts) == 0 {
		return "Empty"
	}
	return strings.Join(parts, " · ")
}

/* -------------------------------------------------------------------------
   Kinds of file
   ------------------------------------------------------------------------- */

// extensionOf returns a key's lower-case extension, with the dot.
func extensionOf(key string) string {
	base := key
	if i := strings.LastIndex(key, "/"); i >= 0 {
		base = key[i+1:]
	}
	dot := strings.LastIndex(base, ".")
	if dot <= 0 {
		return ""
	}
	return strings.ToLower(base[dot:])
}

// kindOf classifies a key by its extension, so the interface can give it
// an icon and a color.
func kindOf(key string, folder bool) fileKind {
	if folder || strings.HasSuffix(key, "/") {
		return kindFolder
	}
	ext := extensionOf(key)
	switch {
	case imageExtensions[ext]:
		return kindImage
	case videoExtensions[ext]:
		return kindVideo
	case audioExtensions[ext]:
		return kindAudio
	case archiveExtensions[ext]:
		return kindArchive
	case documentExtensions[ext], textExtensions[ext]:
		return kindDocument
	default:
		return kindOther
	}
}

// kindLabel names a kind of file for the list's Kind column: "PNG image".
func kindLabel(k fileKind, key string) string {
	if k == kindFolder {
		return "Folder"
	}
	ext := strings.TrimPrefix(extensionOf(key), ".")
	if ext == "" {
		return "File"
	}
	switch k {
	case kindImage:
		return strings.ToUpper(ext) + " image"
	case kindVideo:
		return strings.ToUpper(ext) + " video"
	case kindAudio:
		return strings.ToUpper(ext) + " audio"
	case kindArchive:
		return strings.ToUpper(ext) + " archive"
	case kindDocument:
		return strings.ToUpper(ext) + " document"
	default:
		return strings.ToUpper(ext) + " file"
	}
}

// textExtensions is the set of extensions the app previews as text.
var textExtensions = map[string]bool{
	".txt": true, ".md": true, ".markdown": true, ".csv": true, ".tsv": true,
	".log": true, ".json": true, ".yaml": true, ".yml": true, ".toml": true,
	".ini": true, ".conf": true, ".env": true, ".srt": true, ".vtt": true,
	".js": true, ".mjs": true, ".cjs": true, ".ts": true, ".tsx": true, ".jsx": true,
	".go": true, ".rs": true, ".py": true, ".rb": true, ".java": true, ".kt": true,
	".c": true, ".h": true, ".cc": true, ".cpp": true, ".hpp": true, ".cs": true,
	".swift": true, ".php": true, ".sh": true, ".bash": true, ".zsh": true, ".sql": true,
	".css": true, ".scss": true, ".html": true, ".htm": true, ".xml": true,
	".vue": true, ".svelte": true, ".lua": true, ".dart": true,
}

var (
	imageExtensions = map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
		".avif": true, ".bmp": true, ".ico": true, ".tiff": true,
	}
	videoExtensions = map[string]bool{
		".mp4": true, ".mov": true, ".webm": true, ".mkv": true,
		".avi": true, ".m4v": true, ".ogv": true,
	}
	audioExtensions = map[string]bool{
		".mp3": true, ".wav": true, ".m4a": true, ".flac": true,
		".ogg": true, ".oga": true, ".aac": true, ".opus": true,
	}
	archiveExtensions = map[string]bool{
		".zip": true, ".gz": true, ".tar": true, ".tgz": true,
		".bz2": true, ".xz": true, ".7z": true, ".rar": true, ".zst": true,
	}
	documentExtensions = map[string]bool{
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
		".ppt": true, ".pptx": true, ".rtf": true, ".epub": true,
	}
)

// isImageKey reports whether a key holds an image the app can decode.
func isImageKey(key string) bool { return imageExtensions[extensionOf(key)] }

// isTextKey reports whether a key holds text the app can show.
func isTextKey(key string) bool { return textExtensions[extensionOf(key)] }

// isPreviewable reports whether the app has anything useful to show for a
// key without downloading it.
func isPreviewable(key string) bool { return isImageKey(key) || isTextKey(key) }
