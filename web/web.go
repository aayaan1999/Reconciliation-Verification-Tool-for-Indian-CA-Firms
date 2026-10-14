// Package web gives recon-server the browser UI built from this folder.
//
// The UI is embedded only when the program is built with the "ui" tag, after `npm run build`
// has written web/dist:
//
//	cd web && npm ci && npm run build && cd .. && go build -tags ui ./cmd/recon-server
//
// Without the tag the program still compiles (so tests and vet need no Node.js) and Files
// returns ErrNotBuilt. The release build always uses the tag.
package web

import "errors"

// ErrNotBuilt means this program was built without the "ui" tag, so it holds no UI.
var ErrNotBuilt = errors.New("the browser UI is not included in this build (build with -tags ui after npm run build in web/)")
