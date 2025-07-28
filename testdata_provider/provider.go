// Package testdata_provider provides embedded test data for go-git-daemon testing.
// This package is only imported by test code and won't be included in production builds.
package testdata_provider

import (
	"embed"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-billy/v6/embedfs"
	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/osfs"
)

//go:embed all:testdata
var TestData embed.FS

const RepositoryPath = "/testdata/Hello-World.git"

// GetTestData returns the raw embed.FS for tests to wrap with their own embedfs.New().
// This avoids import cycles while providing embedded test data.
func GetTestData() *embed.FS {
	return &TestData
}

// GetTestFilesystem returns a billy.Filesystem for the embedded test data.
// This provides a ready-to-use filesystem for testing.
func GetTestFilesystem() billy.Filesystem {
	return embedfs.New(&TestData)
}

// GetFilesystem returns a filesystem of the requested type that provides access to the test data.
// All filesystem types provide access to the same underlying test repository.
func GetFilesystem(fsType string) (billy.Filesystem, error) {
	switch fsType {
	case "embedfs":
		return embedfs.New(&TestData), nil
		
	case "osfs":
		// Use testdata_provider directory as root so repository is accessible at testdata/Hello-World.git
		workingDir, err := filepath.Abs("testdata_provider")
		if err != nil {
			return nil, err
		}
		return osfs.New(workingDir), nil
		
	case "memfs":
		// Copy repository from embedded data to memfs
		fs := memfs.New()
		err := copyEmbedToMemfs(&TestData, fs, ".")
		if err != nil {
			return nil, err
		}
		return fs, nil
		
	default:
		return nil, nil
	}
}

// copyEmbedToMemfs recursively copies files from embed.FS to billy memfs
func copyEmbedToMemfs(embedFS *embed.FS, memFS billy.Filesystem, rootPath string) error {
	return fs.WalkDir(embedFS, rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		// Convert embed path to memfs path - keep full path with leading slash
		var relativePath string
		if rootPath == "." {
			relativePath = "/" + path
		} else {
			relativePath = path[len(rootPath):]
			if relativePath == "" {
				relativePath = "/"
			} else if !filepath.IsAbs(relativePath) {
				relativePath = "/" + relativePath
			}
		}
		
		if d.IsDir() {
			// Create directory
			return memFS.MkdirAll(relativePath, 0755)
		} else {
			// Create file
			// First ensure parent directory exists
			parentDir := filepath.Dir(relativePath)
			if parentDir != "/" {
				err := memFS.MkdirAll(parentDir, 0755)
				if err != nil {
					return err
				}
			}
			
			// Open source file
			srcFile, err := embedFS.Open(path)
			if err != nil {
				return err
			}
			defer srcFile.Close()
			
			// Create destination file
			dstFile, err := memFS.Create(relativePath)
			if err != nil {
				return err
			}
			defer dstFile.Close()
			
			// Copy content
			_, err = io.Copy(dstFile, srcFile)
			return err
		}
	})
}

