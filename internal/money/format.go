package money

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatIndian writes p in rupees with Indian digit grouping: 1,12,450 or, with
// showPaise, 1,12,450.50. Negative amounts get a leading minus sign.
// With showPaise false, an amount that has paise is an error: a report must never
// show a figure different from the one stored.
func FormatIndian(p Paise, showPaise bool) (string, error) {
	negative := p < 0
	mag := uint64(p)
	if negative {
		mag = uint64(-(p + 1)) + 1 // safe for MinPaise
	}
	rupees, paise := mag/PaisePerRupee, mag%PaisePerRupee
	if !showPaise && paise != 0 {
		return "", fmt.Errorf("money: %d paise has paise; it cannot be shown in whole rupees", int64(p))
	}

	var b strings.Builder
	if negative && mag != 0 {
		b.WriteByte('-')
	}
	b.WriteString(groupIndian(strconv.FormatUint(rupees, 10)))
	if showPaise {
		fmt.Fprintf(&b, ".%02d", paise)
	}
	return b.String(), nil
}

// groupIndian inserts commas into a string of digits: the last three digits form one
// group and the rest are grouped in twos (12,34,56,789).
func groupIndian(digits string) string {
	if len(digits) <= 3 {
		return digits
	}
	head, last3 := digits[:len(digits)-3], digits[len(digits)-3:]
	var groups []string
	for len(head) > 2 {
		groups = append([]string{head[len(head)-2:]}, groups...)
		head = head[:len(head)-2]
	}
	groups = append([]string{head}, groups...)
	return strings.Join(groups, ",") + "," + last3
}
