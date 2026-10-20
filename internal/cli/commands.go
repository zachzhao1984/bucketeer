package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"bucketeer/internal/store"
)

/* ---- ls ---- */

// lsEntry is one line of a listing, in the JSON document too.
type lsEntry struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Size         int64  `json:"size"`
	LastModified string `json:"lastModified,omitempty"`
}

// lsDoc is what `ls --json` prints.
type lsDoc struct {
	Prefix    string    `json:"prefix"`
	Recursive bool      `json:"recursive"`
	Bucket    string    `json:"bucket"`
	Count     int       `json:"count"`
	Truncated bool      `json:"truncated"`
	Entries   []lsEntry `json:"entries"`
}

func runLS(args []string, cfg Config, opts globalOpts, session *store.Session) error {
	fs := flag.NewFlagSet("ls", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	recursive := fs.Bool("r", false, "list every descendant")
	limit := fs.Int("limit", 0, "stop after this many entries")
	if err := parseFlagSet(fs, args); err != nil {
		return err
	}
	bucket, err := bucketOf(session)
	if err != nil {
		return err
	}
	prefix := ""
	if fs.NArg() > 0 {
		prefix = fs.Arg(0)
	}

	o := newOut(cfg, opts)
	defer o.flush()

	doc := lsDoc{Prefix: prefix, Recursive: *recursive, Bucket: bucket, Entries: []lsEntry{}}
	if *recursive {
		items, err := listPrefix(context.Background(), session, bucket, prefix)
		if err != nil {
			return err
		}
		for _, item := range items {
			if *limit > 0 && len(doc.Entries) >= *limit {
				doc.Truncated = true
				break
			}
			doc.Entries = append(doc.Entries, lsEntry{
				Key: item.Key, Name: item.Name, Type: "object",
				Size: item.Size, LastModified: item.LastModified.Format("2006-01-02T15:04:05Z07:00"),
			})
		}
	} else {
		listing, err := session.List(context.Background(), bucket, prefix)
		if err != nil {
			return err
		}
		for _, item := range listing.Items {
			if *limit > 0 && len(doc.Entries) >= *limit {
				doc.Truncated = true
				break
			}
			entry := lsEntry{Key: item.Key, Name: item.Name, Type: "object", Size: item.Size}
			if item.Folder {
				entry.Type, entry.Size = "folder", 0
			} else {
				entry.LastModified = item.LastModified.Format("2006-01-02T15:04:05Z07:00")
			}
			doc.Entries = append(doc.Entries, entry)
		}
	}
	doc.Count = len(doc.Entries)

	if err := o.doc(doc); err != nil {
		return err
	}
	for _, entry := range doc.Entries {
		switch entry.Type {
		case "folder":
			o.text("%s/", path.Join(prefix, entry.Name))
		default:
			o.text("%s  %s", sizeLabel(entry.Size), entry.Name)
		}
	}
	if !o.json && doc.Count == 0 {
		o.text("(empty)")
	}
	return nil
}

/* ---- find ---- */

// findDoc is what `find --json` prints.
type findDoc struct {
	Bucket    string    `json:"bucket"`
	Prefix    string    `json:"prefix"`
	Query     string    `json:"query"`
	Count     int       `json:"count"`
	Truncated bool      `json:"truncated"`
	Matches   []lsEntry `json:"matches"`
}

func runFind(args []string, cfg Config, opts globalOpts, session *store.Session) error {
	fs := flag.NewFlagSet("find", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	glob := fs.String("glob", "", "match the whole key, case-sensitively")
	minSize := fs.String("min-size", "", "smallest size, as 100MB or 512KiB")
	maxSize := fs.String("max-size", "", "largest size")
	sortBy := fs.String("sort", "name", "size, date or name")
	limit := fs.Int("limit", 100, "most matches to return")
	older := fs.String("older-than", "", "only objects older than this, as 90d")
	newer := fs.String("newer-than", "", "only objects newer than this, as 2h")
	if err := parseFlagSet(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 2 {
		return usagef("find takes a query and an optional prefix")
	}
	bucket, err := bucketOf(session)
	if err != nil {
		return err
	}
	query, prefix := "", ""
	if fs.NArg() > 0 {
		query = fs.Arg(0)
	}
	if fs.NArg() > 1 {
		prefix = fs.Arg(1)
	}

	var minBytes, maxBytes int64 = -1, -1
	if *minSize != "" {
		if minBytes, err = parseSize(*minSize); err != nil {
			return usagef("--min-size: %v", err)
		}
	}
	if *maxSize != "" {
		if maxBytes, err = parseSize(*maxSize); err != nil {
			return usagef("--max-size: %v", err)
		}
	}
	olderThan, err := parseAge(*older)
	if err != nil {
		return usagef("--older-than: %v", err)
	}
	newerThan, err := parseAge(*newer)
	if err != nil {
		return usagef("--newer-than: %v", err)
	}
	switch *sortBy {
	case "size", "date", "name":
	default:
		return usagef("--sort takes size, date or name")
	}

	items, err := listPrefix(context.Background(), session, bucket, prefix)
	if err != nil {
		return err
	}

	// A query matches the object's name; a glob matches the whole key.
	needle := strings.ToLower(strings.TrimSpace(query))
	now := nowFunc()
	matches := make([]lsEntry, 0, len(items))
	for _, item := range items {
		if needle != "" && !strings.Contains(strings.ToLower(item.Name), needle) {
			continue
		}
		if *glob != "" && !matchGlob(*glob, item.Key) {
			continue
		}
		if minBytes >= 0 && item.Size < minBytes {
			continue
		}
		if maxBytes >= 0 && item.Size > maxBytes {
			continue
		}
		age := now.Sub(item.LastModified)
		if olderThan > 0 && age < olderThan {
			continue
		}
		if newerThan > 0 && age > newerThan {
			continue
		}
		matches = append(matches, lsEntry{
			Key: item.Key, Name: item.Name, Type: "object",
			Size: item.Size, LastModified: item.LastModified.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	sortEntryList(matches, *sortBy)

	doc := findDoc{Bucket: bucket, Prefix: prefix, Query: query, Matches: matches}
	if *limit > 0 && len(matches) > *limit {
		doc.Matches = matches[:*limit]
		doc.Truncated = true
	}
	doc.Count = len(doc.Matches)

	o := newOut(cfg, opts)
	defer o.flush()
	if err := o.doc(doc); err != nil {
		return err
	}
	for _, m := range doc.Matches {
		o.text("%s  %s  %s", sizeLabel(m.Size), m.Key, dateLabel(parseTime(m.LastModified)))
	}
	if !o.json && doc.Count == 0 {
		o.text("No matches.")
	}
	return nil
}

// sortEntryList orders matches the way --sort asks.
func sortEntryList(entries []lsEntry, by string) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		switch by {
		case "size":
			if a.Size != b.Size {
				return a.Size > b.Size
			}
		case "date":
			if a.LastModified != b.LastModified {
				return a.LastModified > b.LastModified
			}
		}
		return strings.ToLower(a.Key) < strings.ToLower(b.Key)
	})
}

// matchGlob matches a key against a pattern that supports *, ? and **.
func matchGlob(pattern, key string) bool {
	return matchGlobAt(pattern, key)
}

// matchGlobAt walks a pattern and a key, where ** spans "/" and * does not.
func matchGlobAt(pattern, key string) bool {
	for len(pattern) > 0 {
		switch {
		case strings.HasPrefix(pattern, "**"):
			rest := strings.TrimPrefix(pattern, "**")
			rest = strings.TrimPrefix(rest, "/")
			if rest == "" {
				return true
			}
			for i := 0; i <= len(key); i++ {
				if matchGlobAt(rest, key[i:]) {
					return true
				}
			}
			return false
		case pattern[0] == '*':
			rest := pattern[1:]
			for i := 0; i <= len(key); i++ {
				if matchGlobAt(rest, key[i:]) {
					return true
				}
				if i < len(key) && key[i] == '/' {
					return false
				}
			}
			return false
		case pattern[0] == '?':
			if key == "" || key[0] == '/' {
				return false
			}
			pattern, key = pattern[1:], key[1:]
		default:
			if key == "" || key[0] != pattern[0] {
				return false
			}
			pattern, key = pattern[1:], key[1:]
		}
	}
	return key == ""
}

/* ---- stat ---- */

// statDocs is what `stat --json` prints.
type statDocs struct {
	Bucket       string            `json:"bucket"`
	Key          string            `json:"key"`
	Name         string            `json:"name"`
	Size         int64             `json:"size"`
	ContentType  string            `json:"contentType"`
	ETag         string            `json:"etag"`
	LastModified string            `json:"lastModified"`
	StorageClass string            `json:"storageClass,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

func runStat(args []string, cfg Config, opts globalOpts, session *store.Session) error {
	fs := flag.NewFlagSet("stat", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := parseFlagSet(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return usagef("stat takes one object key")
	}
	bucket, err := bucketOf(session)
	if err != nil {
		return err
	}
	key := exactKey(fs.Arg(0))
	info, err := session.StatObject(context.Background(), bucket, key)
	if err != nil {
		return err
	}
	o := newOut(cfg, opts)
	defer o.flush()
	if err := o.doc(statDocs{
		Bucket: bucket, Key: info.Key, Name: info.Name, Size: info.Size,
		ContentType: info.ContentType, ETag: info.ETag,
		LastModified: info.LastModified.Format("2006-01-02T15:04:05Z07:00"),
		StorageClass: info.StorageClass, Metadata: info.Metadata,
	}); err != nil {
		return err
	}
	o.text("%s", info.Key)
	o.text("  size          %s (%d bytes)", sizeLabel(info.Size), info.Size)
	o.text("  type          %s", orDash(info.ContentType))
	o.text("  modified      %s", dateLabel(info.LastModified))
	o.text("  etag          %s", orDash(info.ETag))
	o.text("  storage class %s", orDash(info.StorageClass))
	return nil
}

/* ---- usage ---- */

// usageDoc is what `usage --json` prints.
type usageDoc struct {
	Bucket    string           `json:"bucket"`
	Prefix    string           `json:"prefix"`
	Objects   int64            `json:"totalObjects"`
	Bytes     int64            `json:"totalBytes"`
	ByKind    map[string]int64 `json:"bytesByKind"`
	ByFolder  map[string]int64 `json:"bytesByFolder"`
	Largest   []lsEntry        `json:"largest"`
	Truncated bool             `json:"truncated"`
	Counts    map[string]int64 `json:"countByKind"`
}

func runUsage(args []string, cfg Config, opts globalOpts, session *store.Session) error {
	fs := flag.NewFlagSet("usage", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := parseFlagSet(fs, args); err != nil {
		return err
	}
	bucket, err := bucketOf(session)
	if err != nil {
		return err
	}
	prefix := ""
	if fs.NArg() > 0 {
		prefix = fs.Arg(0)
	}
	items, err := listPrefix(context.Background(), session, bucket, prefix)
	if err != nil {
		return err
	}

	doc := usageDoc{
		Bucket: bucket, Prefix: prefix, Objects: int64(len(items)),
		ByKind: map[string]int64{}, ByFolder: map[string]int64{},
		Counts: map[string]int64{}, Largest: []lsEntry{},
	}
	for _, item := range items {
		doc.Bytes += item.Size
		kind := store.KindOf(item.Key)
		doc.ByKind[kind] += item.Size
		doc.Counts[kind]++
		folder := "root"
		if i := strings.IndexByte(item.Key, '/'); i > 0 {
			folder = item.Key[:i]
		}
		doc.ByFolder[folder] += item.Size
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Size > items[j].Size })
	for i, item := range items {
		if i >= 10 {
			break
		}
		doc.Largest = append(doc.Largest, lsEntry{Key: item.Key, Name: item.Name, Size: item.Size, Type: "object"})
	}

	o := newOut(cfg, opts)
	defer o.flush()
	if err := o.doc(doc); err != nil {
		return err
	}
	o.text("%s in %s, %d objects", sizeLabel(doc.Bytes), orRoot(prefix), doc.Objects)
	for _, kind := range sortedKeys(doc.ByKind) {
		o.text("  %-10s %8s  %d objects", kind, sizeLabel(doc.ByKind[kind]), doc.Counts[kind])
	}
	return nil
}

/* ---- cat ---- */

func runCat(args []string, cfg Config, opts globalOpts, session *store.Session) error {
	fs := flag.NewFlagSet("cat", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	force := fs.Bool("force", false, "read an object over 64 MiB")
	if err := parseFlagSet(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return usagef("cat takes one object key")
	}
	bucket, err := bucketOf(session)
	if err != nil {
		return err
	}
	key := exactKey(fs.Arg(0))
	if !*force {
		info, err := session.StatObject(context.Background(), bucket, key)
		if err != nil {
			return err
		}
		if info.Size > 64<<20 {
			return usagef("%s is %s: pass --force to print something that large", key, sizeLabel(info.Size))
		}
	}
	body, _, err := session.OpenObject(context.Background(), bucket, key, -1, -1)
	if err != nil {
		return err
	}
	defer body.Close()
	_, err = io.Copy(cfg.Stdout, body)
	return err
}

/* ---- get ---- */

func runGet(args []string, cfg Config, opts globalOpts, session *store.Session) error {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	recursive := fs.Bool("r", false, "download a whole prefix")
	noClobber := fs.Bool("no-clobber", false, "keep files that already exist")
	dryRun := fs.Bool("dry-run", false, "print the plan and write nothing")
	if err := parseFlagSet(fs, args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return usagef("get takes one or more keys, then a local path")
	}
	bucket, err := bucketOf(session)
	if err != nil {
		return err
	}
	sources := fs.Args()[:fs.NArg()-1]
	dest := localPath(fs.Arg(fs.NArg() - 1))

	// A destination that exists as a folder, ends in a separator, or comes
	// with several sources receives the objects under their own names.
	destIsDir, destExists := modeOf(dest)
	intoFolder := destIsDir || strings.HasSuffix(dest, "/") || len(sources) > 1
	o := newOut(cfg, opts)
	defer o.flush()

	type plan struct{ key, dest string }
	var plans []plan
	for _, src := range sources {
		key := exactKey(src)
		// -r means "take a prefix" only when the source is not an object:
		// `get -r notes.txt out.txt` must download that one file.
		prefix := strings.HasSuffix(src, "/")
		if *recursive && !prefix {
			if _, err := session.StatObject(context.Background(), bucket, key); err != nil {
				prefix = true
			}
		}
		if prefix && !*recursive {
			return usagef("%s is a prefix: pass -r to download everything under it", src)
		}
		if prefix {
			items, err := listPrefix(context.Background(), session, bucket, key)
			if err != nil {
				return err
			}
			if len(items) == 0 {
				return fmt.Errorf("no objects under %q", src)
			}
			// The prefix's own name is kept when the caller wrote a
			// trailing slash, which is what makes `get -r media/ ./out`
			// write ./out/media/... rather than flattening it.
			base := ""
			if strings.HasSuffix(src, "/") {
				base = keyBase(key)
			}
			for _, item := range items {
				rel := strings.TrimPrefix(item.Key, withSlash(key))
				plans = append(plans, plan{key: item.Key, dest: filepath.Join(dest, base, filepath.FromSlash(rel))})
			}
			continue
		}
		target := dest
		if intoFolder {
			target = filepath.Join(dest, filepath.Base(key))
		}
		// Naming one file explicitly overwrites it; a destination that
		// already holds a file is only a mistake when several sources are
		// being written into it.
		if destExists && !destIsDir && len(sources) > 1 {
			return usagef("%s is a file: give a folder when downloading several objects", dest)
		}
		plans = append(plans, plan{key: key, dest: target})
	}

	written := 0
	for _, p := range plans {
		if *noClobber {
			if _, exists := modeOf(p.dest); exists {
				o.text("skip %s: %s exists", p.key, p.dest)
				continue
			}
		}
		if *dryRun {
			o.text("would write %s → %s", p.key, p.dest)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p.dest), 0o755); err != nil {
			return err
		}
		n, err := session.Download(context.Background(), store.DownloadOptions{
			Bucket: bucket, Key: p.key, Dest: p.dest,
		}, nil)
		if err != nil {
			return err
		}
		written++
		o.text("%s  →  %s", sizeLabel(n), p.dest)
	}
	if !o.json && written == 0 && !*dryRun {
		o.text("Nothing was written.")
	}
	return nil
}

/* ---- put ---- */

func runPut(args []string, cfg Config, opts globalOpts, session *store.Session) error {
	fs := flag.NewFlagSet("put", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	recursive := fs.Bool("r", false, "upload a whole directory")
	noClobber := fs.Bool("no-clobber", false, "keep objects that already exist")
	dryRun := fs.Bool("dry-run", false, "print the plan and write nothing")
	contentType := fs.String("content-type", "", "set the content type of what is uploaded")
	if err := parseFlagSet(fs, args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return usagef("put takes one or more paths, then a destination prefix or key")
	}
	bucket, err := bucketOf(session)
	if err != nil {
		return err
	}
	sources := fs.Args()[:fs.NArg()-1]
	dest := exactKey(fs.Args()[fs.NArg()-1])

	o := newOut(cfg, opts)
	defer o.flush()

	type plan struct{ src, key string }
	var plans []plan
	for _, src := range sources {
		if src == "-" {
			if len(sources) > 1 || dest == "" {
				return usagef("stdin needs one source and a full destination key")
			}
			plans = append(plans, plan{src: "-", key: dest})
			continue
		}
		local := localPath(src)
		isDir, exists := modeOf(local)
		if !exists {
			return fmt.Errorf("%s does not exist", src)
		}
		if isDir {
			if !*recursive {
				return usagef("%s is a directory: pass -r to upload it", src)
			}
			root := filepath.Clean(local)
			err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
					return nil
				}
				rel, err := filepath.Rel(root, p)
				if err != nil {
					return err
				}
				prefix := dest
				if prefix != "" {
					prefix += "/"
				}
				plans = append(plans, plan{src: p, key: prefix + filepath.ToSlash(filepath.Join(filepath.Base(root), rel))})
				return nil
			})
			if err != nil {
				return err
			}
			continue
		}
		key := dest
		// A trailing slash, or a destination with several sources, names a
		// folder to put the file in.
		if strings.HasSuffix(fs.Args()[fs.NArg()-1], "/") || len(sources) > 1 {
			if key != "" {
				key += "/"
			}
			key += filepath.Base(local)
		}
		plans = append(plans, plan{src: local, key: key})
	}
	sort.SliceStable(plans, func(i, j int) bool { return plans[i].key < plans[j].key })

	uploaded := 0
	for _, p := range plans {
		if *noClobber {
			if _, err := session.StatObject(context.Background(), bucket, p.key); err == nil {
				o.text("skip %s: it exists", p.key)
				continue
			}
		}
		if *dryRun {
			o.text("would upload %s → %s", p.src, p.key)
			continue
		}
		var err error
		if p.src == "-" {
			err = session.UploadWriter(context.Background(), bucket, p.key, cfg.Stdin, -1, *contentType)
		} else {
			// The content type the caller named must apply to the bytes as
			// they are written, not replace them with an empty object
			// afterwards.
			err = session.UploadFile(context.Background(), store.UploadOptions{
				Bucket: bucket, Key: p.key, Src: p.src, Size: -1, ContentType: *contentType,
			}, nil)
		}
		if err != nil {
			return err
		}
		uploaded++
		o.text("→ %s", p.key)
	}
	if !o.json && uploaded == 0 && !*dryRun && len(plans) == 0 {
		o.text("Nothing to upload.")
	}
	return nil
}

/* ---- cp, mv ---- */

func runCopy(args []string, cfg Config, opts globalOpts, session *store.Session) error {
	return runTransfer(args, cfg, opts, session, false)
}

func runMove(args []string, cfg Config, opts globalOpts, session *store.Session) error {
	return runTransfer(args, cfg, opts, session, true)
}

// runTransfer copies or moves one key or a whole prefix.
func runTransfer(args []string, cfg Config, opts globalOpts, session *store.Session, move bool) error {
	name := "cp"
	if move {
		name = "mv"
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	recursive := fs.Bool("r", false, "copy or move a whole prefix")
	noClobber := fs.Bool("no-clobber", false, "keep objects that already exist")
	dryRun := fs.Bool("dry-run", false, "print the plan and change nothing")
	if err := parseFlagSet(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return usagef("%s takes a source and a destination", name)
	}
	bucket, err := bucketOf(session)
	if err != nil {
		return err
	}
	src, dst := fs.Arg(0), fs.Arg(1)

	o := newOut(cfg, opts)
	defer o.flush()

	ctx := context.Background()
	type plan struct{ from, to string }
	var plans []plan

	from := exactKey(src)
	srcPrefix := strings.HasSuffix(src, "/")
	if *recursive || srcPrefix {
		if !*recursive {
			return usagef("%s is a prefix: pass -r to %s everything under it", src, name)
		}
		items, err := listPrefix(ctx, session, bucket, from)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return fmt.Errorf("no objects under %q", src)
		}
		base := withSlash(from)
		target := withSlash(dst)
		if !strings.HasSuffix(dst, "/") && dst != "" {
			target += "/"
		}
		for _, item := range items {
			rel := strings.TrimPrefix(item.Key, base)
			plans = append(plans, plan{from: item.Key, to: target + rel})
		}
	} else {
		to := exactKey(dst)
		if strings.HasSuffix(dst, "/") || dst == "" {
			to = withSlash(dst) + keyBase(from)
		}
		plans = append(plans, plan{from: from, to: to})
	}

	for _, p := range plans {
		if p.from == p.to {
			continue
		}
		if strings.HasPrefix(p.to, withSlash(p.from)) {
			return usagef("cannot %s %s into itself", name, p.from)
		}
		if *noClobber {
			if _, err := session.StatObject(ctx, bucket, p.to); err == nil {
				o.text("skip %s: it exists", p.to)
				continue
			}
		}
		if *dryRun {
			o.text("would %s %s → %s", name, p.from, p.to)
			continue
		}
		if move {
			err = session.Rename(ctx, bucket, p.from, p.to)
		} else {
			err = session.Copy(ctx, bucket, p.from, p.to)
		}
		if err != nil {
			return err
		}
		o.text("%s → %s", p.from, p.to)
	}
	return nil
}

/* ---- rm ---- */

func runRemove(args []string, cfg Config, opts globalOpts, session *store.Session) error {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	recursive := fs.Bool("r", false, "delete every object under a prefix")
	dryRun := fs.Bool("dry-run", false, "print the plan and delete nothing")
	everything := fs.Bool("yes-really-delete-everything", false, "allow deleting the whole bucket")
	if err := parseFlagSet(fs, args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return usagef("rm takes one or more keys")
	}
	bucket, err := bucketOf(session)
	if err != nil {
		return err
	}

	o := newOut(cfg, opts)
	defer o.flush()

	ctx := context.Background()
	keys := make([]string, 0, len(fs.Args()))
	for _, arg := range fs.Args() {
		key := strings.Trim(arg, "/")
		if key == "" {
			if !*everything {
				return usagef("that is the whole bucket: pass --yes-really-delete-everything to delete every object in it")
			}
			items, err := listPrefix(ctx, session, bucket, "")
			if err != nil {
				return err
			}
			for _, item := range items {
				keys = append(keys, item.Key)
			}
			continue
		}
		if strings.HasSuffix(arg, "/") || *recursive {
			items, err := listPrefix(ctx, session, bucket, key)
			if err != nil {
				return err
			}
			if len(items) == 0 {
				o.text("%s holds nothing to delete.", orRoot(key+"/"))
			}
			for _, item := range items {
				keys = append(keys, item.Key)
			}
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)

	if *dryRun {
		for _, key := range keys {
			o.text("would delete %s", key)
		}
		o.text("%d objects would be deleted.", len(keys))
		// The plan is the point of a dry run, so it is written before the
		// command returns.
		o.flush()
		return nil
	}
	deleted, err := session.Remove(ctx, bucket, keys)
	if err != nil {
		return err
	}
	o.text("Deleted %d objects.", deleted)
	return nil
}

/* ---- mkdir ---- */

func runMkdir(args []string, cfg Config, opts globalOpts, session *store.Session) error {
	fs := flag.NewFlagSet("mkdir", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := parseFlagSet(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return usagef("mkdir takes one folder path")
	}
	bucket, err := bucketOf(session)
	if err != nil {
		return err
	}
	folder := strings.Trim(fs.Arg(0), "/")
	if folder == "" {
		return usagef("mkdir needs a folder name")
	}
	parent, name := path.Split(folder)
	key, err := session.MakeFolder(context.Background(), bucket, parent, keyBase(name))
	if err != nil {
		return err
	}
	o := newOut(cfg, opts)
	defer o.flush()
	if err := o.doc(map[string]string{"created": key}); err != nil {
		return err
	}
	o.text("Created %s", key)
	return nil
}

/* ---- share ---- */

func runShare(args []string, cfg Config, opts globalOpts, session *store.Session) error {
	fs := flag.NewFlagSet("share", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	expiry := fs.Int("expiry", 3600, "how long the link works, in seconds")
	if err := parseFlagSet(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return usagef("share takes one object key")
	}
	if *expiry < 1 || *expiry > 604800 {
		return usagef("--expiry takes 1 to 604800 seconds")
	}
	bucket, err := bucketOf(session)
	if err != nil {
		return err
	}
	key := exactKey(fs.Arg(0))
	url, err := session.Presign(context.Background(), bucket, key, timeDurationSeconds(*expiry))
	if err != nil {
		return err
	}
	o := newOut(cfg, opts)
	defer o.flush()
	if err := o.doc(map[string]any{"key": key, "url": url, "expiresIn": *expiry}); err != nil {
		return err
	}
	o.text("%s", url)
	return nil
}

/* ---- profiles ---- */

// tokenDoc is one connection, without its access key.
type tokenDoc struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Endpoint string `json:"endpoint"`
	Region   string `json:"region"`
	Bucket   string `json:"bucket,omitempty"`
	Active   bool   `json:"selected"`
}

// runProfiles lists the saved connections.
func runProfiles(args []string, cfg Config, opts globalOpts) error {
	fs := flag.NewFlagSet("profiles", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "print one JSON document")
	if err := parseFlagSet(fs, args); err != nil {
		return err
	}
	if *asJSON {
		opts.json = true
	}
	profiles, err := cfg.Profiles.List()
	if err != nil {
		return err
	}
	// No saved connection is a normal state, not a failure: the list is
	// simply empty, and --profile cannot name one.
	chosen := store.Profile{}
	if len(profiles) > 0 {
		if p, err := chooseProfile(profiles, opts.profile); err == nil {
			chosen = p
		}
	}
	docs := make([]tokenDoc, 0, len(profiles))
	for _, p := range profiles {
		docs = append(docs, tokenDoc{
			ID: p.ID, Name: p.Name, Provider: string(p.Provider),
			Endpoint: p.Host(), Region: p.Region, Bucket: p.Bucket,
			Active: p.ID == chosen.ID,
		})
	}

	o := newOut(cfg, opts)
	defer o.flush()
	if err := o.doc(docs); err != nil {
		return err
	}
	if len(docs) == 0 {
		o.text("No connection is saved yet: add one in Bucketeer, under Settings → Connections.")
		return nil
	}
	for _, d := range docs {
		mark := " "
		if d.Active {
			mark = "*"
		}
		o.text("%s %s  (%s)", mark, d.Name, orDash(d.Bucket))
		o.text("    %s  %s", d.Provider, d.Endpoint)
	}
	return nil
}

/* ---- The reference the window shows ---- */

// CommandRow is one command as the Settings page lists it: the way it is
// called, what it does, and a sample that is ready to run.
type CommandRow struct {
	Cmd    string
	Does   string
	Sample string
}

// CommandRows is the reference, in the order the page shows it.
func CommandRows() []CommandRow {
	return []CommandRow{
		{"bkt ls", "List a folder, with sizes.", "bkt ls"},
		{"bkt ls -r", "List every object under a prefix.", "bkt ls media/ -r"},
		{"bkt find", "Search names, sizes and ages.", `bkt find "" media/ --min-size 10MB`},
		{"bkt stat", "One object's size, type and metadata.", "bkt stat media/logo.png"},
		{"bkt usage", "Objects and bytes, by kind and folder.", "bkt usage media/"},
		{"bkt cat", "Print a text object's bytes.", "bkt cat media/index.html"},
		{"bkt mkdir", "Create a folder.", "bkt mkdir media/2026/"},
		{"bkt get", "Download objects or a whole folder.", "bkt get -r media/ ./out"},
		{"bkt put", "Upload files, stdin or a folder.", "bkt put -r ./photos media/"},
		{"bkt cp", "Copy on the server, without downloading.", "bkt cp media/logo.png logos/logo.png"},
		{"bkt mv", "Move or rename.", "bkt mv media/old/ media/new/ -r"},
		{"bkt rm", "Delete; -r takes a prefix.", "bkt rm media/tmp/ -r --dry-run"},
		{"bkt share", "A presigned link, up to 7 days.", "bkt share media/logo.png --expiry 3600"},
	}
}

/* ---- parsing helpers ---- */

// parseTime reads one of the timestamps the JSON documents print.
func parseTime(s string) (t time.Time) {
	if s == "" {
		return time.Time{}
	}
	t, _ = timeOrZero(s)
	return t
}

// orDash returns s, or a dash when it is empty.
func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// orRoot names the bucket root in a message.
func orRoot(prefix string) string {
	if prefix == "" || prefix == "/" {
		return "the bucket root"
	}
	return prefix
}

// sortedKeys returns a map's keys, sorted.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// parseSize reads a size as 100MB, 512KiB, or a plain number of bytes.
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0, errors.New("give a size, as 100MB")
	}
	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "KIB"):
		mult, s = 1<<10, strings.TrimSuffix(s, "KIB")
	case strings.HasSuffix(s, "MIB"):
		mult, s = 1<<20, strings.TrimSuffix(s, "MIB")
	case strings.HasSuffix(s, "GIB"):
		mult, s = 1<<30, strings.TrimSuffix(s, "GIB")
	case strings.HasSuffix(s, "TIB"):
		mult, s = 1<<40, strings.TrimSuffix(s, "TIB")
	case strings.HasSuffix(s, "KB"):
		mult, s = 1000, strings.TrimSuffix(s, "KB")
	case strings.HasSuffix(s, "MB"):
		mult, s = 1000*1000, strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "GB"):
		mult, s = 1000*1000*1000, strings.TrimSuffix(s, "GB")
	case strings.HasSuffix(s, "TB"):
		mult, s = 1000*1000*1000*1000, strings.TrimSuffix(s, "TB")
	case strings.HasSuffix(s, "K"):
		mult, s = 1000, strings.TrimSuffix(s, "K")
	case strings.HasSuffix(s, "M"):
		mult, s = 1000*1000, strings.TrimSuffix(s, "M")
	case strings.HasSuffix(s, "G"):
		mult, s = 1000*1000*1000, strings.TrimSuffix(s, "G")
	case strings.HasSuffix(s, "B"):
		s = strings.TrimSuffix(s, "B")
	}
	s = strings.TrimSpace(s)
	value, err := strconv.ParseFloat(s, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%q is not a size", s)
	}
	return int64(value * float64(mult)), nil
}

// parseAge reads a span as 90d, 12h or 30m.
func parseAge(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	unit := s[len(s)-1]
	value, err := strconv.ParseFloat(s[:len(s)-1], 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%q is not a span", s)
	}
	var scale time.Duration
	switch unit {
	case 'd':
		scale = 24 * time.Hour
	case 'h':
		scale = time.Hour
	case 'm':
		scale = time.Minute
	case 's':
		scale = time.Second
	case 'w':
		scale = 7 * 24 * time.Hour
	default:
		return 0, fmt.Errorf("%q ends in %q: use d, h, m, s or w", s, string(unit))
	}
	return time.Duration(value * float64(scale)), nil
}

// nowFunc is the clock find's age filters read, so a test can hold it.
var nowFunc = time.Now
