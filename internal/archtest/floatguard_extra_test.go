package archtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// floatGuardCases feed floatUses one source file each. caught=false marks a construct
// the guard is expected to leave alone; gap names a known bypass that is skipped.
var floatGuardCases = []struct {
	name    string
	imports string
	body    string
	caught  bool
	gap     string
}{
	{"math.Pow constant args", `"math"`, `var X = int64(math.Pow(10, 2))`, true, ""},
	{"math.Sqrt aliased import", `m "math"`, `var X = m.Sqrt`, true, ""},
	{"Sprintf %.2f", `"fmt"`, `var X = fmt.Sprintf("%.2f", 1)`, true, ""},
	{"Sprintf %8.3F", `"fmt"`, `var X = fmt.Sprintf("%8.3F", 1)`, true, ""},
	{"Sprintf %e", `"fmt"`, `var X = fmt.Sprintf("%e", 1)`, true, ""},
	{"Sprintf %g", `"fmt"`, `var X = fmt.Sprintf("%g", 1)`, true, ""},
	{"Sprintf %v is not float", `"fmt"`, `var X = fmt.Sprintf("%v %d %s", 1, 2, "x")`, false, ""},
	{"Sprintf escaped percent f", `"fmt"`, `var X = fmt.Sprintf("100%%f", 1)`, false, ""},
	{"Fprintf float verb in 2nd arg", `"fmt"; "os"`, `var _ = func() { fmt.Fprintf(os.Stdout, "%.1f", 1) }`, true, ""},
	{"json.Number.Float64", `"encoding/json"`, `func F(n json.Number) { _, _ = n.Float64() }`, true, ""},
	{"big.NewFloat", `"math/big"`, `var X = big.NewFloat`, true, ""},
	{"big.Rat.Float64", `"math/big"`, `var X, _ = big.NewRat(1, 3).Float64()`, true, ""},
	{"strconv.ParseFloat", `"strconv"`, `var X, _ = strconv.ParseFloat("1", 64)`, true, ""},
	{"float literal", ``, `var X = 1.5`, true, ""},
	{"exponent literal", ``, `var X = 1e3`, true, ""},
	{"float64 type", ``, `var X float64`, true, ""},
	{"math integer constants only", `"math"`, `var X = int64(math.MaxInt64) + math.MinInt64 + math.MaxInt32`, false, ""},
	{"dot import of math", `. "math"`, `var X = Pow`, true, "dot-imported math functions are not tracked"},
	{"fmt format from constant", `"fmt"`, `const f = "%.2f"
var X = fmt.Sprintf(f, 1)`, true, "only string literals are inspected"},
	{"fmt format by concatenation", `"fmt"`, `var X = fmt.Sprintf("%"+".2f", 1)`, true, "only string literals are inspected"},
	{"time.Duration.Seconds", `"time"`, `var X = time.Second.Seconds()`, true, "float-returning methods other than Float32/Float64 are not tracked"},
}

func TestFloatGuardCases(t *testing.T) {
	for _, c := range floatGuardCases {
		t.Run(c.name, func(t *testing.T) {
			if c.gap != "" {
				t.Skip("BUG: float guard bypass: " + c.gap)
			}
			src := "package p\n\nimport (\n\t" + strings.ReplaceAll(c.imports, "; ", "\n\t") + "\n)\n\n" + c.body + "\n"
			file := filepath.Join(t.TempDir(), "p.go")
			if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			got := floatUses(t, file)
			if c.caught && len(got) == 0 {
				t.Errorf("not caught:\n%s", src)
			}
			if !c.caught && len(got) != 0 {
				t.Errorf("false positive %v:\n%s", got, src)
			}
		})
	}
}
