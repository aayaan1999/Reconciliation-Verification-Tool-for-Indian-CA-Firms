// Command recon-updater verifies and applies a signed program update from USB,
// and rolls back to the previous version if needed. It has no network access.
//
// Built in SPEC-042. This is the SPEC-001 placeholder.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "recon-updater: not built yet (see specs/SPEC-042)")
	os.Exit(1)
}
