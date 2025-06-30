xpld
====

xpld is a command-line tool written in Go for compressing, extracting, and inspecting archive files. It supports various archive formats through the github.com/mholt/archives package, providing a simple and efficient way to manage archives.

Features
--------

-   **Create**: Compress files or directories into an archive.

-   **Extract**: Extract archive contents to a specified directory, with an option to flatten the directory structure.

-   **Inspect**: View archive contents in text, JSON, or tree format.

Usage
-----

Run xpld with one of the following commands:

### Create an Archive

Compress files or directories into an archive.

```
xpld create <source> -o <output>
```

-   `<source>`: Path to the file or directory to compress.

-   -o, --output: Output path for the archive (required).

**Example**:

```
xpld create ./my-folder -o output.tar.gz
```

### Extract an Archive

Extract the contents of an archive to a specified directory.

```
xpld extract <archive> -o <output> [--flatten]
```

-   `<archive>`: Path to the archive file.

-   -o, --output: Output directory for extracted files (required).

-   --flatten, -f: Flatten the directory structure during extraction.

**Example**:

```
xpld extract archive.tar.gz -o ./output --flatten
```

### Inspect an Archive

View the contents of an archive in different formats.

```
xpld inspect <archive> [--json | --txt | --tree]
```

-   `<archive>`: Path to the archive file.

-   --json: Output contents in JSON format.

-   --txt: Output contents as plain text (default).

-   --tree: Output contents in a tree-like format.

**Example**:

```
xpld inspect archive.tar.gz --json
```

Supported Formats
-----------------

xpld relies on the github.com/mholt/archives package, thus, we support:

- Supported compression formats
  - brotli (.br)
  - bzip2 (.bz2)
  - flate (.zip)
  - gzip (.gz)
  - lz4 (.lz4)
  - lzip (.lz)
  - minlz (.mz)
  - snappy (.sz) and S2 (.s2)
  - xz (.xz)
  - zlib (.zz)
  - zstandard (.zst)
- Supported archive formats:
  - .zip
  - .tar (including any compressed variants like .tar.gz)
  - .rar (RO)
  - .7z  (RO)

License
-------

This project is dual-licensed under the ISC License (pre-2007, equivalent to MIT) and the RABRMS License. Choose whichever suits you best.

Dependencies
------------

-   github.com/mholt/archives: For archive format handling.

-   github.com/urfave/cli/v3: For command-line interface parsing.
