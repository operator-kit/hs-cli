package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/operator-kit/hs-cli/internal/pii/evaluation"
)

func main() {
	repoRoot := flag.String("repo-root", ".", "repository root containing internal/pii/testdata")
	flag.Parse()
	path := filepath.Join(*repoRoot, "internal", "pii", "testdata", "privacy-filter", "v1", "performance", "workloads.json")
	if err := evaluation.WritePerformanceWorkloads(path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
