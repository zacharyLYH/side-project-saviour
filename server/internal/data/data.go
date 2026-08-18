// Package data owns the on-disk layout of $DATA_DIR. Bootstrap creates the
// tree once on boot; every other package writes only into its own
// subdirectory.
package data

import (
	"fmt"
	"os"
	"path/filepath"
)

// Bootstrap creates $DATA_DIR and its subdirectories with owner-only
// permissions. Idempotent: safe to call on every boot.
func Bootstrap(dataDir string) error {
	paths := []string{
		dataDir,
		filepath.Join(dataDir, "projects"),
		filepath.Join(dataDir, "harnesses"),
		filepath.Join(dataDir, "ssh"),
	}
	for _, d := range paths {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", d, err)
		}
	}
	return nil
}
