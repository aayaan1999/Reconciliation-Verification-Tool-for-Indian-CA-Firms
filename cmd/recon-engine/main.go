// Command recon-engine reads uploaded files, runs the reconciliation rules and
// produces findings. It must never import a network package (see internal/archtest).
//
// Built in SPEC-011 (CLI) and SPEC-023 (daemon mode). This is the SPEC-001 placeholder.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "recon-engine: not built yet (see specs/SPEC-011)")
	os.Exit(1)
}
