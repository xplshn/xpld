// xpld is a simple CLI interface to mholt/archives
// xpld is hosted at https://github.com/xplshn/xpld
//
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"strconv"
	"time"
	"net/mail"

	"github.com/a8m/tree"
	"github.com/mholt/archives"
	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:  "xpld",
		Authors: []any{
			&mail.Address{Name: "xplshn", Address: "anto@xplshn.com.ar"},
		},
		Version: "v1",
		Usage: "compress, extract, or inspect archive files",
		Commands: []*cli.Command{
			{
				Name:      "create",
				Aliases:   []string{"c"},
				Usage:     "create an archive from files or directories",
				ArgsUsage: "<source>",
				Flags:     commonFlags(&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Required: true}),
				Action: func(ctx context.Context, c *cli.Command) error {
					return createArchive(ctx, c, c.Args().First(), c.String("output"))
				},
			},
			{
				Name:      "extract",
				Aliases:   []string{"e"},
				Usage:     "extract an archive",
				ArgsUsage: "<archive>",
				Flags: append(commonFlags(&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Required: true}),
					&cli.BoolFlag{Name: "flatten", Aliases: []string{"f"}}),
				Action: func(ctx context.Context, c *cli.Command) error {
					return extractToDirectory(ctx, c, c.Args().First(), c.String("output"))
				},
			},
			{
				Name:      "inspect",
				Aliases:   []string{"i"},
				Usage:     "inspect archive contents",
				ArgsUsage: "<archive>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "json", Usage: "print output as JSON"},
					&cli.BoolFlag{Name: "txt", Usage: "force plain text output"},
					&cli.BoolFlag{Name: "tree", Usage: "draw a tree of the archive contents"},
					&cli.BoolFlag{Name: "color", Aliases: []string{"c"}, Usage: "enable ANSI color"},
					&cli.BoolFlag{Name: "sizes", Aliases: []string{"s"}, Usage: "show file sizes in bytes"},
					&cli.StringFlag{Name: "sort", Usage: "sort by: name|extension|version|size|atime|ctime|mtime", Value: "name"},
					&cli.BoolFlag{Name: "reverse", Aliases: []string{"r"}, Usage: "reverse the sort order"},
					&cli.BoolFlag{Name: "dirs-first", Aliases: []string{"d"}, Usage: "list directories before files"},
					&cli.BoolFlag{Name: "all", Aliases: []string{"a"}, Usage: "include hidden files"},
					&cli.BoolFlag{Name: "dirs-only", Usage: "list directories only"},
					&cli.BoolFlag{Name: "full-path", Usage: "print full path for each entry"},
					&cli.BoolFlag{Name: "ignore-case", Usage: "ignore case when matching or sorting"},
					&cli.BoolFlag{Name: "follow-links", Usage: "follow symlinks as directories"},
					&cli.IntFlag{Name: "depth", Usage: "limit directory traversal depth"},
					&cli.StringFlag{Name: "pattern", Usage: "only list files matching a glob pattern"},
					&cli.StringFlag{Name: "ipattern", Usage: "exclude files matching a glob pattern"},
					&cli.BoolFlag{Name: "match-dirs", Usage: "apply patterns to directory names"},
					&cli.BoolFlag{Name: "prune", Usage: "prune empty directories from the output"},
					&cli.BoolFlag{Name: "unit-size", Usage: "print sizes in human-readable units"},
					&cli.BoolFlag{Name: "show-uid", Usage: "display file owner UID"},
					&cli.BoolFlag{Name: "show-gid", Usage: "display file group GID"},
					&cli.BoolFlag{Name: "last-mod", Usage: "display last modification time"},
					&cli.BoolFlag{Name: "quotes", Usage: "quote file names"},
					&cli.BoolFlag{Name: "inodes", Usage: "show inode number"},
					&cli.BoolFlag{Name: "device", Usage: "show device ID"},
					&cli.BoolFlag{Name: "no-indent", Usage: "disable tree indentation"},
				},
				Action: inspectArchive,
			},
		},
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func commonFlags(output *cli.StringFlag) []cli.Flag {
	return []cli.Flag{
		output,
		&cli.BoolFlag{Name: "preserve-ownership", Value: true, Usage: "preserve entry ownership when extracting"},
		&cli.BoolFlag{Name: "preserve-permissions", Value: true, Usage: "preserve entry permissions when extracting"},
		&cli.BoolFlag{Name: "ignore-root-ownership", Usage: "ignore root's ownership of entries"},
		&cli.BoolFlag{Name: "uid-ownership", Value: true, Usage: "preserve only UID"},
	}
}

func createArchive(ctx context.Context, c *cli.Command, src, dst string) error {
	if src == "" || dst == "" {
		return errors.New("source and output are required")
	}
	outFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer outFile.Close()

	format, _, err := archives.Identify(ctx, dst, nil)
	if err != nil {
		return err
	}
	archiver, ok := format.(archives.Archiver)
	if !ok {
		return fmt.Errorf("unsupported archive format")
	}

	var inputs []archives.FileInfo
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		inputs = append(inputs, archives.FileInfo{
			NameInArchive: rel,
			FileInfo:      info,
			Open: func() (fs.File, error) {
				if info.IsDir() {
					return nil, nil
				}
				return os.Open(path)
			},
		})
		return nil
	})
	if err != nil {
		return err
	}
	return archiver.Archive(ctx, outFile, inputs)
}

func extractToDirectory(ctx context.Context, c *cli.Command, tarball, dst string) error {
	if tarball == "" || dst == "" {
		return errors.New("archive path and output directory are required")
	}
	f, err := os.Open(tarball)
	if err != nil {
		return err
	}
	defer f.Close()

	format, input, err := archives.Identify(ctx, tarball, f)
	if err != nil {
		return err
	}
	extractor, ok := format.(archives.Extractor)
	if !ok {
		return fmt.Errorf("unsupported archive format")
	}

	return extractor.Extract(ctx, input, func(ctx context.Context, fi archives.FileInfo) error {
		name := fi.NameInArchive
		if c.Bool("flatten") {
			name = filepath.Base(name)
		}
		path := filepath.Join(dst, name)
		if fi.IsDir() {
			mode := fi.FileInfo.Mode()
			if !c.Bool("preserve-permissions") {
				mode = 0755
			}
			return os.MkdirAll(path, mode)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		r, err := fi.Open()
		if err != nil {
			return err
		}
		defer r.Close()
		w, err := os.Create(path)
		if err != nil {
			return err
		}
		defer w.Close()
		if _, err := io.Copy(w, r); err != nil {
			return err
		}
		if c.Bool("preserve-permissions") {
			if err := os.Chmod(path, fi.FileInfo.Mode()); err != nil {
				return err
			}
		}
		if c.Bool("preserve-ownership") || c.Bool("uid-ownership") {
			if stat, ok := fi.FileInfo.Sys().(interface{ Uid() int; Gid() int }); ok {
				uid := stat.Uid()
				if !c.Bool("uid-ownership") {
					uid = -1
				}
				if err := os.Chown(path, uid, stat.Gid()); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

type fileEntry struct{ name string; info fs.FileInfo }
type treeFS struct{ fsys fs.FS }

func (tfs treeFS) ReadDir(dirname string) ([]string, error) {
	entries, err := fs.ReadDir(tfs.fsys, dirname)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		name := e.Name()
		if e.IsDir() && !strings.HasSuffix(name, "/") {
			name += "/"
		}
		names[i] = name
	}
	return names, nil
}
func (tfs treeFS) Stat(name string) (os.FileInfo, error) {
	f, err := tfs.fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Stat()
}

func inspectArchive(ctx context.Context, c *cli.Command) error {
	f, err := os.Open(c.Args().First())
	if err != nil {
		return err
	}
	defer f.Close()

	_, _, err = archives.Identify(ctx, c.Args().First(), f)
	if err != nil {
		return err
	}
	fsys, err := archives.FileSystem(ctx, c.Args().First(), f)
	if err != nil {
		return err
	}

	var files []fileEntry
	err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if c.Bool("dirs-only") && !d.IsDir() {
			return nil
		}
		if c.String("pattern") != "" {
			if ok, _ := filepath.Match(c.String("pattern"), d.Name()); !ok && (!c.Bool("match-dirs") || !d.IsDir()) {
				return nil
			}
		}
		if c.String("ipattern") != "" {
			if ok, _ := filepath.Match(c.String("ipattern"), d.Name()); ok && (!c.Bool("match-dirs") || !d.IsDir()) {
				return nil
			}
		}
		if c.Int("depth") > 0 && strings.Count(path, "/")+1 > c.Int("depth") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		name := path
		if d.IsDir() && !strings.HasSuffix(name, "/") {
			name += "/"
		}
		if c.Bool("full-path") {
			name = filepath.Join(c.Args().First(), name)
		}
		if c.Bool("quotes") {
			name = fmt.Sprintf("%q", name)
		}
		files = append(files, fileEntry{name, info})
		return nil
	})
	if err != nil {
		return err
	}

	// Sorting
	sortFiles(c, files)

	// Output
	switch {
	case c.Bool("json"):
		return outputJSON(c, files)
	case c.Bool("tree"):
		return outputTree(c, fsys)
	default:
		return outputText(c, files)
	}
}

func sortFiles(c *cli.Command, files []fileEntry) {
	switch c.String("sort") {
	case "size":
		sort.Slice(files, func(i, j int) bool { return files[i].info.Size() < files[j].info.Size() })
	case "mtime":
		sort.Slice(files, func(i, j int) bool { return files[i].info.ModTime().Before(files[j].info.ModTime()) })
	case "ctime":
		sort.Slice(files, func(i, j int) bool {
			if stat, ok := files[i].info.Sys().(interface{ Ctime() time.Time }); ok {
				if stat2, ok := files[j].info.Sys().(interface{ Ctime() time.Time }); ok {
					return stat.Ctime().Before(stat2.Ctime())
				}
			}
			return false
		})
	case "atime":
		sort.Slice(files, func(i, j int) bool {
			if stat, ok := files[i].info.Sys().(interface{ Atime() time.Time }); ok {
				if stat2, ok := files[j].info.Sys().(interface{ Atime() time.Time }); ok {
					return stat.Atime().Before(stat2.Atime())
				}
			}
			return false
		})
	case "extension":
		sort.Slice(files, func(i, j int) bool {
			extI := filepath.Ext(files[i].name)
			extJ := filepath.Ext(files[j].name)
			if extI == extJ {
				return files[i].name < files[j].name
			}
			return extI < extJ
		})
	case "version":
		sort.Slice(files, func(i, j int) bool {
			verI := extractVersion(files[i].name)
			verJ := extractVersion(files[j].name)
			if verI == verJ {
				return files[i].name < files[j].name
			}
			return compareVersions(verI, verJ)
		})
	default: // name
		if c.Bool("ignore-case") {
			sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i].name) < strings.ToLower(files[j].name) })
		} else {
			sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
		}
	}
	if c.Bool("dirs-first") {
		sort.SliceStable(files, func(i, j int) bool { return files[i].info.IsDir() && !files[j].info.IsDir() })
	}
	if c.Bool("reverse") {
		for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
			files[i], files[j] = files[j], files[i]
		}
	}
}

func outputJSON(c *cli.Command, files []fileEntry) error {
	out := make([]map[string]interface{}, len(files))
	for i, f := range files {
		entry := map[string]interface{}{
			"name": f.name,
			"size": f.info.Size(),
			"mode": f.info.Mode().String(),
			"mtime": f.info.ModTime(),
		}
		if c.Bool("unit-size") {
			entry["size"] = formatBytes(f.info.Size())
		}
		if stat, ok := f.info.Sys().(interface{ Uid() int; Gid() int }); ok {
			if c.Bool("show-uid") {
				entry["uid"] = stat.Uid()
			}
			if c.Bool("show-gid") {
				entry["gid"] = stat.Gid()
			}
		}
		if c.Bool("last-mod") {
			entry["mtime"] = f.info.ModTime()
		}
		if c.Bool("inodes") {
			if stat, ok := f.info.Sys().(interface{ Ino() uint64 }); ok {
				entry["inode"] = stat.Ino()
			}
		}
		if c.Bool("device") {
			if stat, ok := f.info.Sys().(interface{ Dev() uint64 }); ok {
				entry["device"] = stat.Dev()
			}
		}
		if c.Bool("ctime") {
			if stat, ok := f.info.Sys().(interface{ Ctime() time.Time }); ok {
				entry["ctime"] = stat.Ctime()
			}
		}
		if c.Bool("atime") {
			if stat, ok := f.info.Sys().(interface{ Atime() time.Time }); ok {
				entry["atime"] = stat.Atime()
			}
		}
		if c.String("sort") == "extension" {
			entry["extension"] = filepath.Ext(f.name)
		}
		if c.String("sort") == "version" {
			if ver := extractVersion(f.name); ver != "" {
				entry["version"] = ver
			}
		}
		out[i] = entry
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func outputTree(c *cli.Command, fsys fs.FS) error {
	opts := &tree.Options{
		Fs:         treeFS{fsys},
		All:        c.Bool("all"),
		DirsOnly:   c.Bool("dirs-only"),
		FullPath:   c.Bool("full-path"),
		IgnoreCase: c.Bool("ignore-case"),
		FollowLink: c.Bool("follow-links"),
		DeepLevel:  c.Int("depth"),
		Pattern:    c.String("pattern"),
		IPattern:   c.String("ipattern"),
		MatchDirs:  c.Bool("match-dirs"),
		Prune:      c.Bool("prune"),
		ByteSize:   c.Bool("sizes"),
		UnitSize:   c.Bool("unit-size"),
		ShowUid:    c.Bool("show-uid"),
		ShowGid:    c.Bool("show-gid"),
		LastMod:    c.Bool("last-mod"),
		Quotes:     c.Bool("quotes"),
		Inodes:     c.Bool("inodes"),
		Device:     c.Bool("device"),
		NoSort:     c.String("sort") == "",
		ModSort:    c.String("sort") == "mtime",
		DirSort:    c.Bool("dirs-first"),
		NameSort:   c.String("sort") == "name",
		SizeSort:   c.String("sort") == "size",
		CTimeSort:  c.String("sort") == "ctime",
		//ATimeSort:  c.String("sort") == "atime",
		//ExtSort:    c.String("sort") == "extension",
		VerSort:    c.String("sort") == "version",
		ReverSort:  c.Bool("reverse"),
		NoIndent:   c.Bool("no-indent"),
		Colorize:   c.Bool("color"),
		OutFile:    os.Stdout,
		Now:        time.Now(),
	}

	if c.String("sort") == "atime" {
		return fmt.Errorf("atime sort is unsupported when using `--tree`")
	}
	if c.String("sort") == "extension" {
		return fmt.Errorf("extension sort is unsupported when using `--tree`")
	}

	n := tree.New(".")
	n.Visit(opts)
	n.Print(opts)
	return nil
}

func outputText(c *cli.Command, files []fileEntry) error {
	for _, f := range files {
		name := f.name
		if c.Bool("color") {
			name = tree.ANSIColor(&tree.Node{FileInfo: f.info}, name)
		}
		var parts []string
		if c.Bool("sizes") {
			if c.Bool("unit-size") {
				parts = append(parts, formatBytes(f.info.Size()))
			} else {
				parts = append(parts, fmt.Sprintf("%10d", f.info.Size()))
			}
		}
		if stat, ok := f.info.Sys().(interface{ Uid() int; Gid() int }); ok {
			if c.Bool("show-uid") {
				parts = append(parts, fmt.Sprintf("uid=%d", stat.Uid()))
			}
			if c.Bool("show-gid") {
				parts = append(parts, fmt.Sprintf("gid=%d", stat.Gid()))
			}
		}
		if c.Bool("last-mod") {
			parts = append(parts, f.info.ModTime().Format(time.RFC3339))
		}
		if c.Bool("inodes") {
			if stat, ok := f.info.Sys().(interface{ Ino() uint64 }); ok {
				parts = append(parts, fmt.Sprintf("ino=%d", stat.Ino()))
			}
		}
		if c.Bool("device") {
			if stat, ok := f.info.Sys().(interface{ Dev() uint64 }); ok {
				parts = append(parts, fmt.Sprintf("dev=%d", stat.Dev()))
			}
		}
		if c.Bool("ctime") {
			if stat, ok := f.info.Sys().(interface{ Ctime() time.Time }); ok {
				parts = append(parts, stat.Ctime().Format(time.RFC3339))
			}
		}
		if c.Bool("atime") {
			if stat, ok := f.info.Sys().(interface{ Atime() time.Time }); ok {
				parts = append(parts, stat.Atime().Format(time.RFC3339))
			}
		}
		if c.String("sort") == "extension" {
			parts = append(parts, fmt.Sprintf("ext=%s", filepath.Ext(f.name)))
		}
		if c.String("sort") == "version" {
			if ver := extractVersion(f.name); ver != "" {
				parts = append(parts, fmt.Sprintf("ver=%s", ver))
			}
		}
		if len(parts) > 0 {
			fmt.Printf("%s %s\n", strings.Join(parts, " "), name)
		} else {
			fmt.Println(name)
		}
	}
	return nil
}

func extractVersion(name string) string {
	parts := strings.Split(filepath.Base(name), "-")
	for _, part := range parts {
		if strings.Contains(part, ".") && !strings.HasPrefix(part, ".") {
			return part
		}
	}
	return ""
}

func compareVersions(v1, v2 string) bool {
	v1Parts := strings.Split(v1, ".")
	v2Parts := strings.Split(v2, ".")
	for i := 0; i < len(v1Parts) && i < len(v2Parts); i++ {
		n1, _ := strconv.Atoi(v1Parts[i])
		n2, _ := strconv.Atoi(v2Parts[i])
		if n1 != n2 {
			return n1 < n2
		}
	}
	return len(v1Parts) < len(v2Parts)
}

func formatBytes(i int64) string {
	var n float64
	sFmt, eFmt := "%.01f", ""
	switch {
	case i >= tree.EB: eFmt, n = "E", float64(i)/float64(tree.EB)
	case i >= tree.PB: eFmt, n = "P", float64(i)/float64(tree.PB)
	case i >= tree.TB: eFmt, n = "T", float64(i)/float64(tree.TB)
	case i >= tree.GB: eFmt, n = "G", float64(i)/float64(tree.GB)
	case i >= tree.MB: eFmt, n = "M", float64(i)/float64(tree.MB)
	case i >= tree.KB: eFmt, n = "K", float64(i)/float64(tree.KB)
	default: sFmt, n = "%.0f", float64(i)
	}
	if eFmt != "" && n >= 10 { sFmt = "%.0f" }
	return strings.Trim(fmt.Sprintf(sFmt+eFmt, n), " ")
}
