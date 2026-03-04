package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/operator-kit/hs-cli/internal/config"
)

func promptConfigFallback(reader *bufio.Reader, cfgFile string, keyringErr error, applyFn func(*config.Config)) error {
	path := config.ResolvedPath(cfgFile)
	fmt.Fprintf(os.Stderr, "\nWarning: OS keyring not available (%v)\n", keyringErr)
	fmt.Fprintf(os.Stderr, "Store credentials in config file (%s) instead? [y/n]: ", path)

	answer, _ := reader.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(answer)) != "y" {
		return fmt.Errorf("credentials not stored")
	}

	c, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	applyFn(c)
	if err := config.Save(cfgFile, c); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Credentials saved to %s\n", path)
	return nil
}
