// Package auth holds the sign-in rules of the tool: the role matrix (technical spec §B12.1),
// password hashing with argon2id, sessions with their idle and absolute timeouts, the lockout
// after repeated failures, and the user administration that the Admin tab and the user
// commands of recon-engine share. Every change writes its audit entry in the same transaction.
//
// recon-engine runs the user commands, so the package is engine-side in the architecture guard
// (internal/archtest). It imports no network package and it never reads the clock itself: the
// caller passes the time in (SPEC-012).
package auth
