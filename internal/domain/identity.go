package domain

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"strconv"
)

// StableID returns a finding's identity (FD-01; technical spec §B7.1):
// base32(sha256(rule_id | engagement_id | period | scope | match_key)), first 16
// characters, using the standard upper-case alphabet A–Z, 2–7. The rule version, the run
// and every locator are deliberately left out, so re-runs and rule edits update the same
// finding instead of creating a new one. A "|" or "\" in the rule ID is escaped as in
// Subject.Key, so different inputs cannot collide by shifting a separator.
func StableID(ruleID string, engagementID int64, subject Subject) string {
	key := escapeField(ruleID) + "|" + strconv.FormatInt(engagementID, 10) + "|" + subject.Key()
	sum := sha256.Sum256([]byte(key))
	return base32.StdEncoding.EncodeToString(sum[:])[:16]
}

// ValueFingerprint returns the lowercase hex SHA-256 of the canonical JSON of the values
// alone, in their given order (technical spec §B7.1). Labels, input names and locators are
// left out: the same figure on a different page of a new draft, or a reworded label, is
// not a change of value, so a finding decided "Not an issue" stays closed (FD-04). It
// returns an error for a value of invalid kind or text that is not valid UTF-8.
func ValueFingerprint(values []LabeledValue) (string, error) {
	list := make([]any, 0, len(values))
	for i, v := range values {
		obj, err := valueObject(v.Value)
		if err != nil {
			return "", fmt.Errorf("domain: fingerprint value %d (%s): %w", i, v.Input, err)
		}
		list = append(list, obj)
	}
	if err := checkText(list); err != nil {
		return "", err
	}
	b, err := marshalCanonical(list)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
