package daemon

import (
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/storage/filesystem"
	"github.com/stretchr/testify/suite"
)

// FilesystemOperationsTestSuite tests basic filesystem operations across different billy.Filesystem implementations
// These tests validate that filesystems can provide the basic file operations needed by go-git, without network protocol
type FilesystemOperationsTestSuite struct {
	suite.Suite
}

// TestRepositoryValidation tests that servers can validate git repositories across all filesystem types
func (s *FilesystemOperationsTestSuite) TestRepositoryValidation() {
	configs := GenerateTestMatrix(s.T())
	
	for _, config := range configs {
		s.Run(config.Description, func() {
			LogTestConfiguration(s.T(), config)
			
			// Create server (no network setup needed for validation)
			server := &Server{
				Filesystem: config.Filesystem,
				ExportAll:  true,
			}
			
			ValidateRepository(s.T(), server, config)
		})
	}
}

// TestFilesystemAccess tests basic file system operations (stat, readdir) across all filesystem types
func (s *FilesystemOperationsTestSuite) TestFilesystemAccess() {
	configs := GenerateTestMatrix(s.T())
	
	for _, config := range configs {
		s.Run(config.Description, func() {
			LogTestConfiguration(s.T(), config)
			ValidateFilesystemAccess(s.T(), config)
		})
	}
}

// TestChrootFunctionality tests chroot operations across all filesystem types
func (s *FilesystemOperationsTestSuite) TestChrootFunctionality() {
	configs := GenerateTestMatrix(s.T())
	
	for _, config := range configs {
		s.Run(config.Description, func() {
			LogTestConfiguration(s.T(), config)
			ValidateChroot(s.T(), config)
		})
	}
}

// TestGoGitRepositoryAccess tests that go-git can open repositories from different filesystems
func (s *FilesystemOperationsTestSuite) TestGoGitRepositoryAccess() {
	configs := GenerateTestMatrix(s.T())
	
	// Only test custom go-git handlers for this test
	for _, config := range configs {
		if config.HandlerName != "go_git_custom" {
			continue
		}
		
		s.Run(config.Description, func() {
			LogTestConfiguration(s.T(), config)
			ValidateFilesystemAccess(s.T(), config)
			ValidateChroot(s.T(), config)
			s.validateGoGitAccess(config)
		})
	}
}

// validateGoGitAccess tests that go-git can open and read from the repository
func (s *FilesystemOperationsTestSuite) validateGoGitAccess(config TestConfiguration) {
	s.T().Logf("🔍 Testing go-git repository access")
	
	// Create chrooted filesystem 
	chrootedFS, err := config.Filesystem.Chroot(config.RepoPath)
	s.Require().NoError(err, "should be able to chroot to repository")
	
	// Create go-git storage
	storage := filesystem.NewStorage(chrootedFS, nil)
	s.T().Logf("✅ Storage created")
	
	// Open repository with go-git
	repo, err := git.Open(storage, nil)
	s.Require().NoError(err, "go-git should be able to open repository")
	s.T().Logf("✅ Repository opened with go-git")
	
	// Test HEAD reference
	head, err := repo.Head()
	s.Require().NoError(err, "should be able to get HEAD reference")
	s.T().Logf("✅ HEAD reference: %s -> %s", head.Name(), head.Hash())
	
	// Test reference listing
	refs, err := repo.References()
	s.Require().NoError(err, "should be able to list references")
	
	refCount := 0
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		s.T().Logf("  - %s -> %s", ref.Name(), ref.Hash())
		refCount++
		return nil
	})
	s.Require().NoError(err, "should be able to iterate references")
	s.Greater(refCount, 0, "should have at least one reference")
	s.T().Logf("✅ References: found %d references", refCount)
}