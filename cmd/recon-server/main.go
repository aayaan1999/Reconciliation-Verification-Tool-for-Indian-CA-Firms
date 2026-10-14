// Command recon-server serves the browser UI and API on the office network,
// handles logins, uploads and the job queue, and supervises recon-engine.
//
// Built in SPEC-022 onwards. This is the SPEC-001 placeholder. It already links the
// embedded UI (package web) so a release build can be checked for it.
package main

import (
	"fmt"
	"os"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/web"
)

func main() {
	ui := "browser UI included"
	if _, err := web.Files(); err != nil {
		ui = err.Error()
	}
	fmt.Fprintf(os.Stderr, "recon-server: not built yet (see specs/SPEC-022); %s\n", ui)
	os.Exit(1)
}
