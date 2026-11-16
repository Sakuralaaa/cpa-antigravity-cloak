//go:build ignore

// package-release zips one shared library at the archive root and writes its
// sha256 line in the checksums.txt format CPA's plugin store expects.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	library := flag.String("library", "", "shared library path")
	archive := flag.String("archive", "", "zip output path")
	checksum := flag.String("checksum", "", "sha256 line output path")
	flag.Parse()
	if *library == "" || *archive == "" || *checksum == "" {
		fmt.Fprintln(os.Stderr, "library, archive and checksum are required")
		os.Exit(2)
	}
	if err := run(*library, *archive, *checksum); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(library, archive, checksum string) error {
	src, err := os.Open(library)
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.Create(archive)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(out)
	hdr := &zip.FileHeader{Name: filepath.Base(library), Method: zip.Deflate}
	hdr.SetMode(0o755)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, src); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	data, err := os.ReadFile(archive)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	line := hex.EncodeToString(sum[:]) + "  " + filepath.Base(archive) + "\n"
	return os.WriteFile(checksum, []byte(line), 0o644)
}
