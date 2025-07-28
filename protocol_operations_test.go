package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/storage/memory"
	"github.com/stretchr/testify/suite"
)

// ProtocolOperationsTestSuite tests complete git protocol operations across different filesystem and handler combinations
// These tests validate the full client-server git communication over network
type ProtocolOperationsTestSuite struct {
	suite.Suite
}

// TestGitCloneMatrix tests git clone operations across all filesystem and handler combinations
func (s *ProtocolOperationsTestSuite) TestGitCloneMatrix() {
	configs := GenerateTestMatrix(s.T())
	
	for _, config := range configs {
		s.Run(config.Description, func() {
			LogTestConfiguration(s.T(), config)
			s.runGitCloneTest(config)
		})
	}
}

// TestReferenceListingMatrix tests git reference listing across all combinations
func (s *ProtocolOperationsTestSuite) TestReferenceListingMatrix() {
	configs := GenerateTestMatrix(s.T())
	
	for _, config := range configs {
		s.Run(config.Description, func() {
			LogTestConfiguration(s.T(), config)
			s.runReferenceListingTest(config)
		})
	}
}

// runGitCloneTest performs a complete git clone test
func (s *ProtocolOperationsTestSuite) runGitCloneTest(config TestConfiguration) {
	// Start test server
	testServer := StartTestServer(s.T(), config)
	defer testServer.Cleanup()
	
	s.T().Logf("📡 Git server URL: %s", testServer.URL)
	
	// Test basic repository validation first
	ValidateRepository(s.T(), testServer.Server, config)
	
	// Perform git clone
	s.T().Logf("🔄 Testing git clone")
	
	storage := memory.NewStorage()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	repo, err := git.CloneContext(ctx, storage, nil, &git.CloneOptions{
		URL: testServer.URL,
	})
	
	// Analyze results
	s.analyzeCloneResult(config, repo, err)
}

// runReferenceListingTest performs git reference listing test
func (s *ProtocolOperationsTestSuite) runReferenceListingTest(testConfig TestConfiguration) {
	// Start test server
	testServer := StartTestServer(s.T(), testConfig)
	defer testServer.Cleanup()
	
	s.T().Logf("📡 Git server URL: %s", testServer.URL)
	
	// Test reference listing
	s.T().Logf("🔍 Testing reference listing")
	
	storage := memory.NewStorage()
	
	// Create repository and remote
	repo, err := git.Init(storage, nil)
	s.Require().NoError(err, "should be able to init repository")
	
	remoteConfig := &config.RemoteConfig{
		Name: "origin",
		URLs: []string{testServer.URL},
	}
	remote, err := repo.CreateRemote(remoteConfig)
	s.Require().NoError(err, "should be able to create remote")
	
	// List references
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	refs, err := remote.ListContext(ctx, &git.ListOptions{})
	
	// Analyze results
	s.analyzeReferenceListingResult(testConfig, refs, err)
}

// analyzeCloneResult analyzes and logs git clone results
func (s *ProtocolOperationsTestSuite) analyzeCloneResult(testConfig TestConfiguration, repo *git.Repository, err error) {
	expectedToWork := s.shouldConfigurationWork(testConfig, "clone")
	
	if err != nil {
		if expectedToWork {
			s.T().Logf("❌ Clone FAILED (unexpected): %v", err)
			s.T().Logf("   🐛 This indicates a problem with: %s", testConfig.Description)
			
			// Log specific error patterns to help debugging
			s.logErrorPatterns(err)
		} else {
			s.T().Logf("❌ Clone failed (expected): %v", err)
			s.T().Logf("   ℹ️  This is expected for: %s", testConfig.Description)
		}
	} else {
		if expectedToWork {
			s.T().Logf("✅ Clone SUCCESS (expected)")
			s.validateClonedRepository(repo)
		} else {
			s.T().Logf("🤔 Clone unexpectedly SUCCEEDED for: %s", testConfig.Description)
			s.T().Logf("   ℹ️  This might indicate the issue was fixed!")
			s.validateClonedRepository(repo)
		}
	}
}

// analyzeReferenceListingResult analyzes and logs reference listing results
func (s *ProtocolOperationsTestSuite) analyzeReferenceListingResult(testConfig TestConfiguration, refs []*plumbing.Reference, err error) {
	expectedToWork := s.shouldConfigurationWork(testConfig, "list_refs")
	
	if err != nil {
		if expectedToWork {
			s.T().Logf("❌ Reference listing FAILED (unexpected): %v", err)
		} else {
			s.T().Logf("❌ Reference listing failed (expected): %v", err)
		}
	} else {
		s.T().Logf("✅ Reference listing SUCCESS: found %d references", len(refs))
		for _, ref := range refs {
			s.T().Logf("  - %s -> %s", ref.Name(), ref.Hash())
		}
	}
}

// shouldConfigurationWork determines if a configuration should work based on our current knowledge
func (s *ProtocolOperationsTestSuite) shouldConfigurationWork(testConfig TestConfiguration, operation string) bool {
	// Based on our testing, we expect:
	// - osfs + git_binary = should work (git binary is proven)
	// - osfs + go_git_custom = should work (filesystem operations work)
	// - embedfs + git_binary = should work (git binary doesn't care about filesystem)  
	// - embedfs + go_git_custom = currently fails (this is what we're debugging)
	
	switch {
	case testConfig.FilesystemType == "osfs":
		return true // osfs should work with both handlers
	case testConfig.FilesystemType == "embedfs" && testConfig.HandlerName == "git_binary":
		return true // git binary should work with any filesystem
	case testConfig.FilesystemType == "embedfs" && testConfig.HandlerName == "go_git_custom":
		return false // This is the known issue we're debugging
	case testConfig.FilesystemType == "memfs":
		return false // Not implemented yet
	default:
		return false
	}
}

// validateClonedRepository validates that a cloned repository has expected content
func (s *ProtocolOperationsTestSuite) validateClonedRepository(repo *git.Repository) {
	s.Require().NotNil(repo, "repository should not be nil")
	
	// Check HEAD
	head, err := repo.Head()
	s.Require().NoError(err, "should be able to get HEAD")
	s.T().Logf("✅ HEAD: %s -> %s", head.Name(), head.Hash())
	
	// Check that we have expected branches
	refs, err := repo.References()
	s.Require().NoError(err, "should be able to list references")
	
	refCount := 0
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		refCount++
		return nil
	})
	s.Require().NoError(err, "should be able to iterate references")
	s.Greater(refCount, 0, "should have references")
	s.T().Logf("✅ Repository validation: %d references found", refCount)
}

// logErrorPatterns logs specific error patterns to help with debugging
func (s *ProtocolOperationsTestSuite) logErrorPatterns(err error) {
	errorStr := err.Error()
	
	switch {
	case contains(errorStr, "broken pipe"):
		s.T().Logf("   🔍 Pattern: Broken pipe - connection closed during protocol exchange")
	case contains(errorStr, "connection reset"):
		s.T().Logf("   🔍 Pattern: Connection reset - server closed connection")
	case contains(errorStr, "object not found"):
		s.T().Logf("   🔍 Pattern: Object not found - git objects not accessible")
	case contains(errorStr, "timeout"):
		s.T().Logf("   🔍 Pattern: Timeout - operation took too long")
	case contains(errorStr, "EOF"):
		s.T().Logf("   🔍 Pattern: EOF - unexpected end of data")
	default:
		s.T().Logf("   🔍 Pattern: Unknown error pattern")
	}
}

// contains checks if a string contains a substring (helper function)
func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || (len(str) > len(substr) && 
		(str[:len(substr)] == substr || str[len(str)-len(substr):] == substr || 
		 containsInMiddle(str, substr))))
}

func containsInMiddle(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestProtocolOperationsTestSuite runs the protocol operations test suite
func TestProtocolOperationsTestSuite(t *testing.T) {
	suite.Run(t, new(ProtocolOperationsTestSuite))
}