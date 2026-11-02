package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Params are the argon2id settings of one hash. They are written into the hash itself
// (the PHC string format), so hashes made with older settings still verify after the
// defaults are raised.
type Params struct {
	MemoryKiB   uint32 // memory cost in KiB
	Iterations  uint32 // time cost
	Parallelism uint8  // lanes
	SaltLength  uint32 // bytes of random salt
	KeyLength   uint32 // bytes of hash
}

// DefaultParams are the settings for new hashes: the OWASP minimum for argon2id (19 MiB, two
// passes, one lane). They keep a sign-in fast on an office computer.
var DefaultParams = Params{MemoryKiB: 19 * 1024, Iterations: 2, Parallelism: 1, SaltLength: 16, KeyLength: 32}

// Limits on the parameters read back from a stored hash. A hash outside them is refused
// rather than run, so a damaged row cannot make sign-in use all the memory of the computer.
const (
	maxMemoryKiB   = 256 * 1024
	maxIterations  = 10
	maxParallelism = 8
)

// Password length limits, in characters (policy, not a security guarantee on their own).
const (
	MinPasswordLength = 10
	MaxPasswordLength = 200
)

// CheckPasswordPolicy refuses a password that is too short or too long. It does not look at
// the text itself: the firm's staff choose their own passwords on the Partner's direction.
func CheckPasswordPolicy(password string) error {
	n := utf8.RuneCountInString(password)
	switch {
	case n < MinPasswordLength:
		return fmt.Errorf("the password must have at least %d characters", MinPasswordLength)
	case n > MaxPasswordLength:
		return fmt.Errorf("the password must have at most %d characters", MaxPasswordLength)
	}
	return nil
}

// HashPassword returns the argon2id hash of a password in the PHC string format:
//
//	$argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>
//
// The salt is random, so the same password gives a different hash each time.
func HashPassword(password string, p Params) (string, error) {
	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, p.Iterations, p.MemoryKiB, p.Parallelism, p.KeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.MemoryKiB, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword reports whether password matches an encoded argon2id hash. The comparison
// takes the same time whether or not the first bytes match.
func VerifyPassword(password, encoded string) (bool, error) {
	p, salt, key, err := parseHash(encoded)
	if err != nil {
		return false, err
	}
	other := argon2.IDKey([]byte(password), salt, p.Iterations, p.MemoryKiB, p.Parallelism, uint32(len(key)))
	return subtle.ConstantTimeCompare(key, other) == 1, nil
}

// parseHash reads an encoded hash, checking its parameters are within the limits above.
func parseHash(encoded string) (Params, []byte, []byte, error) {
	bad := fmt.Errorf("auth: the stored password hash is not a valid argon2id hash")
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return Params{}, nil, nil, bad
	}
	var p Params
	var mem, iters, par uint64
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &iters, &par); err != nil {
		return Params{}, nil, nil, bad
	}
	if mem == 0 || mem > maxMemoryKiB || iters == 0 || iters > maxIterations || par == 0 || par > maxParallelism {
		return Params{}, nil, nil, bad
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return Params{}, nil, nil, bad
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) < 16 || len(key) > 64 {
		return Params{}, nil, nil, bad
	}
	p = Params{MemoryKiB: uint32(mem), Iterations: uint32(iters), Parallelism: uint8(par)}
	return p, salt, key, nil
}

// IsArgon2id reports whether a stored value has the argon2id prefix. The database also checks
// this (migration 0008); the test for AC-1 reads the column with it.
func IsArgon2id(stored string) bool {
	return strings.HasPrefix(stored, "$argon2id$")
}
