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

	"github.com/mholt/archives"
	"github.com/urfave/cli/v3"
)

type Config struct {
	Flatten bool
}

func main() {
	app := &cli.Command{
		Name:  "xpld",
		Usage: "compress, extract or inspect archive files",
		Commands: []*cli.Command{
			{
				Name:      "create",
				Aliases:   []string{"c"},
				Usage:     "create an archive from files or directories",
				ArgsUsage: "<source>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Required: true},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					src := c.Args().First()
					out := c.String("output")
					if src == "" || out == "" {
						return errors.New("source and output are required")
					}
					return createArchive(ctx, src, out)
				},
			},
			{
				Name:      "extract",
				Aliases:   []string{"e"},
				Usage:     "extract an archive",
				ArgsUsage: "<archive>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Required: true},
					&cli.BoolFlag{Name: "flatten", Aliases: []string{"f"}},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					archivePath := c.Args().First()
					dst := c.String("output")
					flatten := c.Bool("flatten")
					if archivePath == "" || dst == "" {
						return errors.New("archive path and output directory are required")
					}
					return extractToDirectory(ctx, archivePath, dst, &Config{Flatten: flatten})
				},
			},
			{
				Name:      "inspect",
				Usage:     "inspect archive contents",
				ArgsUsage: "<archive>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "json"},
					&cli.BoolFlag{Name: "txt"},
					&cli.BoolFlag{Name: "tree"},
				},
				Action: func(ctx context.Context, c *cli.Command) error {
					path := c.Args().First()
					if path == "" {
						return errors.New("archive path is required")
					}
					mode := "txt"
					if c.Bool("json") {
						mode = "json"
					} else if c.Bool("tree") {
						mode = "tree"
					}
					return inspectArchive(ctx, path, mode)
				},
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func createArchive(ctx context.Context, src, dst string) error {
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

		// Use the relative path for the name in the archive
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

func extractToDirectory(ctx context.Context, tarball, dst string, config *Config) error {
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

	handler := func(ctx context.Context, f archives.FileInfo) error {
		name := f.NameInArchive
		if config.Flatten {
			name = filepath.Base(name)
		}
		path := filepath.Join(dst, name)
		if f.IsDir() {
			return os.MkdirAll(path, 0755)
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		r, err := f.Open()
		if err != nil {
			return err
		}
		defer r.Close()

		w, err := os.Create(path)
		if err != nil {
			return err
		}
		defer w.Close()

		_, err = io.Copy(w, r)
		return err
	}

	return extractor.Extract(ctx, input, handler)
}

func inspectArchive(ctx context.Context, path, mode string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	format, input, err := archives.Identify(ctx, path, f)
	if err != nil {
		return err
	}

	extractor, ok := format.(archives.Extractor)
	if !ok {
		return fmt.Errorf("unsupported format")
	}

	var files []string
	handler := func(ctx context.Context, f archives.FileInfo) error {
		name := f.NameInArchive
		if f.IsDir() && !strings.HasSuffix(name, "/") {
			name += "/"
		}
		files = append(files, name)
		return nil
	}

	if err := extractor.Extract(ctx, input, handler); err != nil {
		return err
	}

	sort.Strings(files)

	switch mode {
	case "txt":
		for _, f := range files {
			fmt.Println(f)
		}
	case "json":
		b, err := json.MarshalIndent(files, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	case "tree":
		printTree(files)
	default:
		return fmt.Errorf("unknown inspect mode: %s", mode)
	}

	return nil
}

func printTree(files []string) {
	for _, f := range files {
		fmt.Println(f)
	}
}
