package daemon

import (
	"os"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/suite"
)

// ServerTestSuite tests the Server methods
type ServerTestSuite struct {
	suite.Suite
	tempDir       string
	validRepoPath string
	bareRepoPath  string
	notARepoPath  string
}

// SetupSuite creates test repositories using go-git
func (s *ServerTestSuite) SetupSuite() {
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
func (s *ServerTestSuite) addCommitToRepository(repo *git.Repository, repoPath string) {
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

// TestValidatePath_ValidGitRepository tests validatePath with a valid git repository
func (s *ServerTestSuite) TestValidatePath_ValidGitRepository() {
	server := &Server{}
	
	// Should find the .git directory and return it
	result := server.validatePath(s.validRepoPath)
	s.Equal(s.validRepoPath+"/.git", result, "should return .git directory path")
}

// TestValidatePath_BareRepository tests validatePath with a bare repository
func (s *ServerTestSuite) TestValidatePath_BareRepository() {
	server := &Server{}
	
	// Should recognize bare repo directly
	result := server.validatePath(s.bareRepoPath)
	s.Equal(s.bareRepoPath, result, "should return bare repository path")
}

// TestValidatePath_NonExistentPath tests validatePath with non-existent path
func (s *ServerTestSuite) TestValidatePath_NonExistentPath() {
	server := &Server{}
	
	result := server.validatePath(s.tempDir + "/does-not-exist")
	s.Empty(result, "should return empty string for non-existent path")
}

// TestValidatePath_NotARepository tests validatePath with non-git directory
func (s *ServerTestSuite) TestValidatePath_NotARepository() {
	server := &Server{}
	
	result := server.validatePath(s.notARepoPath)
	s.Empty(result, "should return empty string for non-git directory")
}

// TestValidatePath_StrictPaths tests validatePath with StrictPaths enabled
func (s *ServerTestSuite) TestValidatePath_StrictPaths() {
	server := &Server{StrictPaths: true}
	
	// With strict paths, should be more restrictive about non-existent paths
	result := server.validatePath(s.tempDir + "/does-not-exist")
	s.Empty(result, "should return empty string with strict paths")
}

// TestValidatePath_GitSuffixVariations tests validatePath suffix matching
func (s *ServerTestSuite) TestValidatePath_GitSuffixVariations() {
	server := &Server{}
	
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "direct .git path",
			input:    s.validRepoPath + "/.git",
			expected: s.validRepoPath + "/.git",
		},
		{
			name:     "repo root path (should find .git)",
			input:    s.validRepoPath,
			expected: s.validRepoPath + "/.git",
		},
		{
			name:     "bare repo path",
			input:    s.bareRepoPath,
			expected: s.bareRepoPath,
		},
	}
	
	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := server.validatePath(tt.input)
			s.Equal(tt.expected, result, tt.name)
		})
	}
}

// TestServerTestSuite runs the server test suite
func TestServerTestSuite(t *testing.T) {
	suite.Run(t, new(ServerTestSuite))
}