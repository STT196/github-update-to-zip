package zipper

import (
	"archive/zip"
	"io"
	"os"
)

// ChangedFile is one file that changed between two commits.
type ChangedFile struct {
	// Path is the repo-relative path, e.g. "src/handlers/user.go".
	// This is exactly what gets preserved inside the output zip.
	Path string
	// Status is one of: "added", "modified", "removed", "renamed".
	Status string
	// SourceAbsPath is where to read the file's current bytes from
	// (local git working tree checkout, or a downloaded temp file).
	SourceAbsPath string
}

// BuildZip writes a zip at outZipPath containing every non-removed file
// in files. If rootFolder is non-empty, all paths are prefixed with
// rootFolder/ so the zip extracts to a single parent directory.
func BuildZip(files []ChangedFile, outZipPath string, rootFolder string) error {
	zf, err := os.Create(outZipPath)
	if err != nil {
		return err
	}
	defer zf.Close()

	zw := zip.NewWriter(zf)
	defer zw.Close()

	for _, f := range files {
		if f.Status == "removed" {
			continue
		}

		zipPath := f.Path
		if rootFolder != "" {
			zipPath = rootFolder + "/" + zipPath
		}

		w, err := zw.Create(zipPath) // preserves directory structure
		if err != nil {
			return err
		}

		src, err := os.Open(f.SourceAbsPath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, src); err != nil {
			src.Close()
			return err
		}
		src.Close()
	}

	return nil
}
