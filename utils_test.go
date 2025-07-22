package daemon

import (
	"os"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/suite"
)

// UtilsTestSuite tests the utility functions in utils.go
type UtilsTestSuite struct {
	suite.Suite
	tempDir       string
	validRepoPath string
	bareRepoPath  string
	notARepoPath  string
}

// SetupSuite creates test repositories using go-git
func (s *UtilsTestSuite) SetupSuite() {
	// Create temporary directory for test repositories
	s.tempDir = s.T().TempDir()
	s.validRepoPath = s.tempDir + "/valid-repo"
	s.bareRepoPath = s.tempDir + "/bare-repo"
	s.notARepoPath = s.tempDir + "/not-a-repo"
	
	// Create regular repository with .git directory
	repo, err := git.PlainInit(s.validRepoPath, false)
	s.Require().NoError(err, "should initialize regular git repository")
	s.addCommitToRepository(repo, s.validRepoPath)
	
	// Create bare repository
	_, err = git.PlainInit(s.bareRepoPath, true)
	s.Require().NoError(err, "should initialize bare git repository")
	
	// Create non-git directory
	err = os.MkdirAll(s.notARepoPath, 0755)
	s.Require().NoError(err, "should create not-a-repo directory")
}

// addCommitToRepository adds a commit to a non-bare repository
func (s *UtilsTestSuite) addCommitToRepository(repo *git.Repository, repoPath string) {
	worktree, err := repo.Worktree()
	s.Require().NoError(err, "should get worktree")
	
	// Create a test file
	testFile := repoPath + "/README.md"
	file, err := os.Create(testFile)
	s.Require().NoError(err, "should create test file")
	file.WriteString("# Test Repository\n")
	file.Close()
	
	// Add file to git
	_, err = worktree.Add("README.md")
	s.Require().NoError(err, "should add file to git")
	
	// Create commit
	_, err = worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	s.Require().NoError(err, "should create commit")
}

// TestIsGitDir_ValidGitDirectory tests isGitDir with a valid .git directory
func (s *UtilsTestSuite) TestIsGitDir_ValidGitDirectory() {
	result := isGitDir(s.validRepoPath + "/.git")
	s.True(result, "should recognize valid .git directory")
}

// TestIsGitDir_BareRepository tests isGitDir with a bare repository
func (s *UtilsTestSuite) TestIsGitDir_BareRepository() {
	result := isGitDir(s.bareRepoPath)
	s.True(result, "should recognize bare git repository")
}

// TestIsGitDir_NotARepository tests isGitDir with a non-git directory
func (s *UtilsTestSuite) TestIsGitDir_NotARepository() {
	result := isGitDir(s.notARepoPath)
	s.False(result, "should not recognize non-git directory as git repo")
}

// TestIsGitDir_NonExistentDirectory tests isGitDir with non-existent path
func (s *UtilsTestSuite) TestIsGitDir_NonExistentDirectory() {
	result := isGitDir(s.tempDir + "/does-not-exist")
	s.False(result, "should return false for non-existent directory")
}

// TestIsGitDir_WorkingDirectory tests isGitDir with git working directory (not .git)
func (s *UtilsTestSuite) TestIsGitDir_WorkingDirectory() {
	result := isGitDir(s.validRepoPath)
	s.False(result, "should return false for git working directory (not .git)")
}

// TestIsExportOk_WithExportFile tests isExportOk with git-daemon-export-ok file
func (s *UtilsTestSuite) TestIsExportOk_WithExportFile() {
	// Create temporary directory with export-ok file
	tempDir := s.T().TempDir()
	exportFile := tempDir + "/git-daemon-export-ok"
	
	file, err := os.Create(exportFile)
	s.Require().NoError(err, "should create export-ok file")
	file.Close()
	
	result := isExportOk(tempDir)
	s.True(result, "should return true when git-daemon-export-ok file exists")
}

// TestIsExportOk_WithoutExportFile tests isExportOk without git-daemon-export-ok file
func (s *UtilsTestSuite) TestIsExportOk_WithoutExportFile() {
	result := isExportOk(s.validRepoPath + "/.git")
	s.False(result, "should return false when git-daemon-export-ok file does not exist")
}

// TestIsExportOk_NonExistentDirectory tests isExportOk with non-existent directory
func (s *UtilsTestSuite) TestIsExportOk_NonExistentDirectory() {
	result := isExportOk(s.tempDir + "/does-not-exist")
	s.False(result, "should return false for non-existent directory")
}

// TestIsExportOk_ExportOkIsDirectory tests isExportOk when git-daemon-export-ok is a directory
func (s *UtilsTestSuite) TestIsExportOk_ExportOkIsDirectory() {
	// Create temporary directory with export-ok as a directory
	tempDir := s.T().TempDir()
	exportDir := tempDir + "/git-daemon-export-ok"
	
	err := os.Mkdir(exportDir, 0755)
	s.Require().NoError(err, "should create export-ok directory")
	
	result := isExportOk(tempDir)
	s.False(result, "should return false when git-daemon-export-ok is a directory")
}

// TestUtilsTestSuite runs the utils test suite
func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}