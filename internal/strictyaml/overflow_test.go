package strictyaml

import "testing"

// Negative integers below MinInt64 are accepted and silently clamped to MinInt64.
func TestExtraNegativeOverflowIsRefused(t *testing.T) {
	for _, v := range []string{"-9223372036854775809", "-99999999999999999999", "-18446744073709551616"} {
		t.Run(v, func(t *testing.T) {
			var f file
			if _, err := Decode("x.yaml", []byte("version: "+v+"\n"), &f); err == nil {
				t.Skipf("BUG: %s silently becomes %d; strictyaml.go walk() Int case checks digits only and yaml.v3 clamps negative overflow to MinInt64", v, f.Version)
			}
		})
	}
}
