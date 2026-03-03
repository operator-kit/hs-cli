package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/operator-kit/hs-cli/internal/permission"
)

// skipNames are commands under inbox that do not require permission annotations.
var skipNames = map[string]bool{
	"auth":        true,
	"login":       true,
	"logout":      true,
	"status":      true,
	"config":      true,
	"set":         true, // config set
	"get":         true, // config get (ambiguous but covered below by path)
	"path":        true, // config path
	"permissions": true,
}

// skipPaths are full command paths that should be skipped.
var skipPaths = map[string]bool{
	"inbox auth login":  true,
	"inbox auth logout": true,
	"inbox auth status": true,
	"inbox config set":  true,
	"inbox config get":  true,
	"inbox config path": true,
	"inbox permissions": true,
}

// TestAllLeafCommandsAnnotated walks the inbox command tree and asserts
// every leaf command (RunE != nil) has permission annotations, except
// auth/config/permissions commands.
func TestAllLeafCommandsAnnotated(t *testing.T) {
	inboxCmd := findInboxCmd(rootCmd)
	if inboxCmd == nil {
		t.Fatal("inbox command not found")
	}

	var missing []string
	walkLeaves(inboxCmd, "inbox", func(c *cobra.Command, path string) {
		// Skip auth/config/permissions subtrees
		if isSkippedPath(c, path) {
			return
		}

		_, hasResource := c.Annotations[permission.AnnotationResource]
		_, hasOp := c.Annotations[permission.AnnotationOperation]
		if !hasResource || !hasOp {
			missing = append(missing, path)
		}
	})

	assert.Empty(t, missing, "leaf commands missing permission annotations: %v", missing)
}

func isSkippedPath(c *cobra.Command, path string) bool {
	if skipPaths[path] {
		return true
	}
	// Walk up to check if any ancestor is auth/config/permissions
	for p := c; p != nil; p = p.Parent() {
		name := p.Name()
		if name == "auth" || name == "config" || name == "permissions" {
			return true
		}
	}
	return false
}

func walkLeaves(cmd *cobra.Command, prefix string, fn func(*cobra.Command, string)) {
	for _, sub := range cmd.Commands() {
		path := prefix + " " + sub.Name()
		if sub.RunE != nil || sub.Run != nil {
			fn(sub, path)
		}
		walkLeaves(sub, path, fn)
	}
}
