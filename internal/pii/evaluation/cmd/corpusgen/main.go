package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/operator-kit/hs-cli/internal/pii/evaluation"
)

func main() {
	repoRoot := flag.String("repo-root", ".", "repository root containing internal/pii/testdata")
	flag.Parse()
	if err := evaluation.GeneratePrivacyFilterTestdata(*repoRoot); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
