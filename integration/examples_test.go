//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ExampleSuite struct {
	suite.Suite
	repoRoot string
}

func (s *ExampleSuite) SetupSuite() {
	if os.Getenv("LIBFABRIC_TEST_EXAMPLES") == "" {
		s.T().Skip("set LIBFABRIC_TEST_EXAMPLES=1 to run example integration tests")
	}
	root, err := detectRepoRoot()
	require.NoError(s.T(), err, "locate repository root")
	s.repoRoot = root
}

func (s *ExampleSuite) TestClientBasic() {
	s.runExample("examples/client_basic", nil)
}

func (s *ExampleSuite) TestMsgBasic() {
	s.runExample("examples/msg_basic", nil)
}

func (s *ExampleSuite) TestRMABasic() {
	s.runExample("examples/rma_basic", []string{"LIBFABRIC_EXAMPLE_PROVIDER=" + defaultExampleProvider()})
}

func (s *ExampleSuite) runExample(relPath string, extraEnv []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", "./"+relPath)
	env := append(os.Environ(), "FI_SOCKETS_IFACE=lo0")
	if provider := os.Getenv("LIBFABRIC_INTEGRATION_PROVIDER"); provider != "" {
		env = append(env, "LIBFABRIC_EXAMPLE_PROVIDER="+provider)
	}
	if len(extraEnv) > 0 {
		env = append(env, extraEnv...)
	}
	cmd.Env = env
	cmd.Dir = s.repoRoot

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		s.FailNowf("example timeout", "example %s timed out:\n%s", relPath, string(output))
	}
	require.NoErrorf(s.T(), err, "example %s failed:\n%s", relPath, string(output))
}

func detectRepoRoot() (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			return root, nil
		}
		next := filepath.Dir(root)
		if next == root {
			return "", fmt.Errorf("could not locate repository root containing go.mod")
		}
		root = next
	}
}

func defaultExampleProvider() string {
	if provider := os.Getenv("LIBFABRIC_INTEGRATION_PROVIDER"); provider != "" {
		return provider
	}
	return "sockets"
}

func TestExamples(t *testing.T) {
	suite.Run(t, new(ExampleSuite))
}
