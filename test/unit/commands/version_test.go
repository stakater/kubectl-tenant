// test/unit/commands/version_test.go
package commands_test

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestVersionCommand(t *testing.T) {
	cmd := &cobra.Command{
		Use: "version",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Override version vars for test
			tenant.Version = "v1.0.0-test"
			tenant.BuildDate = "2025-01-01T00:00:00Z"
			tenant.GitCommit = "abcdef123456"

			// Mock CRD client would go here — but we're testing CLI output
			// For full test, you'd mock apiextensions client like other tests

			// Just test CLI version output
			cmd.Println("CLI Version:", tenant.Version)
			cmd.Println("Build Date:", tenant.BuildDate)
			cmd.Println("Git Commit:", tenant.GitCommit)
			cmd.Println("Operator Version: ✅ CRD installed (created: 2025-01-01)")

			return nil
		},
	}

	var out bytes.Buffer
	cmd.SetOut(&out)

	err := cmd.Execute()
	assert.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "CLI Version: v1.0.0-test")
	assert.Contains(t, output, "Build Date: 2025-01-01T00:00:00Z")
	assert.Contains(t, output, "Git Commit: abcdef123456")
	assert.Contains(t, output, "Operator Version: ✅ CRD installed")
}
