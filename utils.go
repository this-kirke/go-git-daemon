package daemon

import (
	"path/filepath"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/osfs"
)

// Returns true if path is a directory containing an `objects` directory and a
// `HEAD` file.
func isGitDir(path string) bool {
	return isGitDirFS(osfs.New("/"), path)
}

// isGitDirFS returns true if path is a directory containing an `objects` directory and a
// `HEAD` file using the provided filesystem interface.
func isGitDirFS(fs billy.Filesystem, path string) bool {
	// Check objects directory exists and is a directory
	objectsPath := filepath.Join(path, "objects")
	stat, err := fs.Stat(objectsPath)
	if err != nil {
		return false
	}
	if !stat.IsDir() {
		return false
	}

	// Check HEAD file exists and is not a directory
	headPath := filepath.Join(path, "HEAD")
	stat, err = fs.Stat(headPath)
	if err != nil {
		return false
	}
	if stat.IsDir() {
		return false
	}

	return true
}

// isExportOk returns true if path contains a `git-daemon-export-ok` file.
func isExportOk(path string) bool {
	return isExportOkFS(osfs.New("/"), path)
}

// isExportOkFS returns true if path contains a `git-daemon-export-ok` file
// using the provided filesystem interface.
func isExportOkFS(fs billy.Filesystem, path string) bool {
	exportOkPath := filepath.Join(path, "git-daemon-export-ok")
	stat, err := fs.Stat(exportOkPath)
	if err != nil {
		return false
	}
	if stat.IsDir() {
		return false
	}

	return true
}
