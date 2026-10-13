// Command recon-server serves the browser UI and API on the office network,
// handles logins, uploads and the job queue, and supervises recon-engine.
//
// Built in SPEC-022 onwards. This is the SPEC-001 placeholder.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "recon-server: not built yet (see specs/SPEC-022)")
	os.Exit(1)
}
