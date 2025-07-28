package daemon

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/aymanbagabas/go-git-daemon/testdata_provider"
	"github.com/go-git/go-billy/v6"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/storage/filesystem"
	"github.com/stretchr/testify/require"
)

// TestConfiguration represents a specific test setup
type TestConfiguration struct {
	FilesystemName string
	FilesystemType string // "osfs", "embedfs", "memfs"
	HandlerName    string // "git_binary", "go_git_custom"
	Filesystem     billy.Filesystem
	Handler        ServiceHandler
	RepoPath       string
	Description    string
}

// TestServer represents a running test server
type TestServer struct {
	Server   *Server
	URL      string
	Listener net.Listener
	Done     chan error
	Cleanup  func()
}

// GetAllFilesystems returns all filesystem implementations to test
func GetAllFilesystems(t *testing.T) []struct {
	Name       string
	Type       string
	Filesystem billy.Filesystem
	RepoPath   string
} {
	// All filesystems access the same repository at the same relative path
	repoPath := testdata_provider.RepositoryPath
	
	// Get filesystems from testdata_provider
	embedfsFS, err := testdata_provider.GetFilesystem("embedfs")
	require.NoError(t, err, "should be able to get embedfs")
	
	osfsFS, err := testdata_provider.GetFilesystem("osfs")
	require.NoError(t, err, "should be able to get osfs")
	
	memfsFS, err := testdata_provider.GetFilesystem("memfs")
	require.NoError(t, err, "should be able to get memfs")
	
	return []struct {
		Name       string
		Type       string  
		Filesystem billy.Filesystem
		RepoPath   string
	}{
		{
			Name:       "osfs",
			Type:       "osfs",
			Filesystem: osfsFS,
			RepoPath:   repoPath,
		},
		{
			Name:       "embedfs", 
			Type:       "embedfs",
			Filesystem: embedfsFS,
			RepoPath:   repoPath,
		},
		{
			Name:       "memfs",
			Type:       "memfs", 
			Filesystem: memfsFS,
			RepoPath:   repoPath,
		},
	}
}

// GetAllHandlers returns all handler implementations to test
func GetAllHandlers(t *testing.T) []struct {
	Name        string
	Type        string
	HandlerFunc func(billy.Filesystem) ServiceHandler
} {
	return []struct {
		Name        string
		Type        string
		HandlerFunc func(billy.Filesystem) ServiceHandler
	}{
		{
			Name: "git_binary",
			Type: "git_binary",
			HandlerFunc: func(fs billy.Filesystem) ServiceHandler {
				return DefaultUploadPackHandler
			},
		},
		{
			Name: "go_git_custom",
			Type: "go_git_custom", 
			HandlerFunc: func(fs billy.Filesystem) ServiceHandler {
				return CreateCustomGoGitHandler(t, fs)
			},
		},
	}
}

// GenerateTestMatrix creates all filesystem × handler combinations
func GenerateTestMatrix(t *testing.T) []TestConfiguration {
	var configs []TestConfiguration
	
	filesystems := GetAllFilesystems(t)
	handlers := GetAllHandlers(t)
	
	for _, fs := range filesystems {
		for _, handler := range handlers {
			configs = append(configs, TestConfiguration{
				FilesystemName: fs.Name,
				FilesystemType: fs.Type,
				HandlerName:    handler.Name,
				Filesystem:     fs.Filesystem,
				Handler:        handler.HandlerFunc(fs.Filesystem),
				RepoPath:       fs.RepoPath,
				Description:    fmt.Sprintf("%s + %s", fs.Name, handler.Name),
			})
		}
	}
	
	return configs
}

// CreateCustomGoGitHandler creates our custom go-git service handler
func CreateCustomGoGitHandler(t *testing.T, fs billy.Filesystem) ServiceHandler {
	return func(ctx context.Context, path string, conn net.Conn, cmdFunc func(*exec.Cmd)) error {
		t.Logf("🔧 Custom go-git handler called for path: %s", path)
		
		// Create storage from filesystem
		repoFS, err := fs.Chroot(path)
		if err != nil {
			return fmt.Errorf("failed to chroot to repository: %w", err)
		}
		
		storage := filesystem.NewStorage(repoFS, nil)
		
		// Use go-git v6's built-in UploadPack function
		opts := &transport.UploadPackOptions{
			AdvertiseRefs: false, // git-daemon handles this
			StatelessRPC:  false, // stateful git:// connection
		}
		
		return transport.UploadPack(ctx, storage, conn, conn, opts)
	}
}

// StartTestServer starts a git server with the given configuration
func StartTestServer(t *testing.T, config TestConfiguration) *TestServer {
	// Create server
	server := &Server{
		Filesystem:        config.Filesystem,
		ExportAll:         true,
		Logger:           log.New(os.Stderr, fmt.Sprintf("GIT-DAEMON-%s-%s: ", config.FilesystemName, config.HandlerName), log.LstdFlags),
		Verbose:          true,
		UploadPackHandler: config.Handler,
	}
	
	// Create listener on random port
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err, "should create listener")
	
	// Get server address
	addr := listener.Addr().String()
	host, port, err := net.SplitHostPort(addr)
	require.NoError(t, err, "should parse address")
	
	if host == "::" {
		host = "localhost"
	}
	
	gitURL := fmt.Sprintf("git://%s:%s%s", host, port, config.RepoPath)
	
	// Start server in background
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Serve(listener)
	}()
	
	// Give server time to start
	time.Sleep(50 * time.Millisecond)
	
	// Create cleanup function
	cleanup := func() {
		server.Close()
		listener.Close()
		<-serverDone
	}
	
	return &TestServer{
		Server:   server,
		URL:      gitURL,
		Listener: listener,
		Done:     serverDone,
		Cleanup:  cleanup,
	}
}

// ValidateRepository tests basic repository validation
func ValidateRepository(t *testing.T, server *Server, config TestConfiguration) {
	t.Logf("🔍 Validating repository: %s", config.RepoPath)
	
	// Test server path validation
	validPath := server.validatePath(config.RepoPath)
	require.NotEmpty(t, validPath, "should validate repository path")
	t.Logf("✅ Repository validation: %s -> %s", config.RepoPath, validPath)
}

// ValidateFilesystemAccess tests basic filesystem operations
func ValidateFilesystemAccess(t *testing.T, config TestConfiguration) {
	if config.Filesystem == nil {
		t.Log("⚠️  No filesystem provided, skipping filesystem validation")
		return
	}
	
	t.Logf("🔍 Testing filesystem access")
	
	// Test stat
	info, err := config.Filesystem.Stat(config.RepoPath)
	require.NoError(t, err, "should be able to stat repository")
	require.True(t, info.IsDir(), "repository should be directory")
	t.Logf("✅ Filesystem stat: repository exists and is directory")
	
	// Test readdir
	entries, err := config.Filesystem.ReadDir(config.RepoPath)
	require.NoError(t, err, "should be able to read repository directory")
	require.Greater(t, len(entries), 0, "repository should contain files")
	t.Logf("✅ Filesystem readdir: found %d items", len(entries))
	
	// Log repository contents
	foundFiles := make(map[string]bool)
	for _, entry := range entries {
		t.Logf("  - %s", entry.Name())
		foundFiles[entry.Name()] = true
	}
	
	// Check for essential git files
	essentialFiles := []string{"HEAD", "objects", "refs"}
	for _, essential := range essentialFiles {
		require.True(t, foundFiles[essential], "repository should contain %s", essential)
	}
	t.Logf("✅ Essential git files present")
}

// ValidateChroot tests chroot functionality
func ValidateChroot(t *testing.T, config TestConfiguration) {
	if config.Filesystem == nil {
		t.Log("⚠️  No filesystem provided, skipping chroot validation")
		return
	}
	
	t.Logf("🔍 Testing chroot functionality")
	
	// Test chroot
	chrootedFS, err := config.Filesystem.Chroot(config.RepoPath)
	require.NoError(t, err, "should be able to chroot to repository")
	require.NotNil(t, chrootedFS, "chrooted filesystem should not be nil")
	t.Logf("✅ Chroot successful")
	
	// Test access to files in chrooted filesystem
	info, err := chrootedFS.Stat("/HEAD")
	require.NoError(t, err, "should be able to stat HEAD in chrooted filesystem")
	require.False(t, info.IsDir(), "HEAD should be a file")
	t.Logf("✅ Chrooted file access: can read HEAD")
	
	// Test directory listing in chrooted filesystem
	entries, err := chrootedFS.ReadDir("/")
	require.NoError(t, err, "should be able to read root in chrooted filesystem")
	require.Greater(t, len(entries), 0, "chrooted root should contain files")
	t.Logf("✅ Chrooted directory listing: found %d items", len(entries))
}

// LogTestConfiguration logs the test configuration for debugging
func LogTestConfiguration(t *testing.T, config TestConfiguration) {
	t.Logf("🧪 Testing Configuration:")
	t.Logf("   Description: %s", config.Description)
	t.Logf("   Filesystem: %s (%s)", config.FilesystemName, config.FilesystemType)
	t.Logf("   Handler: %s", config.HandlerName)
	t.Logf("   Repository: %s", config.RepoPath)
}