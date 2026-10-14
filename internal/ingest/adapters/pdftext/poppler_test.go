package pdftext

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// bundledPdftotext returns the path of third_party/poppler/bin/pdftotext.exe, skipping the
// test off Windows: the bundled build is Windows-only, like the tool the firm receives.
func bundledPdftotext(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("the bundled pdftotext is a Windows program")
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
	exe := filepath.Join(dir, "third_party", "poppler", "bin", "pdftotext.exe")
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("bundled pdftotext missing: %v", err)
	}
	return exe
}

// tablePDF builds a one-page PDF holding a two-column table. Each row's amount is drawn
// before its label, so a reader that follows drawing order puts the columns the wrong way
// round. All figures are illustrative.
func tablePDF() []byte {
	var content strings.Builder
	rows := [][2]string{{"Interest on deposits", "1,12,450"}, {"Bank charges", "2,750"}}
	for i, r := range rows {
		y := 700 - 20*i
		fmt.Fprintf(&content, "BT /F1 10 Tf 300 %d Td (%s) Tj ET\n", y, r[1])
		fmt.Fprintf(&content, "BT /F1 10 Tf 72 %d Td (%s) Tj ET\n", y, r[0])
	}
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", content.Len(), content.String()),
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return b.Bytes()
}

// TestBundledPdftotextKeepsColumns proves the bundled pdftotext runs and that -layout keeps
// each table row on one line with its columns in page order (ADR 0003).
func TestBundledPdftotextKeepsColumns(t *testing.T) {
	exe := bundledPdftotext(t)
	dir := t.TempDir()
	in := filepath.Join(dir, "table.pdf")
	if err := os.WriteFile(in, tablePDF(), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(exe, "-layout", "-enc", "UTF-8", in, "-").CombinedOutput()
	if err != nil {
		t.Fatalf("pdftotext: %v\n%s", err, out)
	}
	text := string(out)
	for _, want := range [][2]string{{"Interest on deposits", "1,12,450"}, {"Bank charges", "2,750"}} {
		found := false
		for _, line := range strings.Split(text, "\n") {
			l, a := strings.Index(line, want[0]), strings.Index(line, want[1])
			if l >= 0 && a > l {
				found = true
			}
		}
		if !found {
			t.Errorf("no line holds %q followed by %q; output:\n%s", want[0], want[1], text)
		}
	}
}
