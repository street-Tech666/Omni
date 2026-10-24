package companion

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestVerifyRejectsIncompleteAndConflictingCoverage(t *testing.T) {
	for _, defect := range []string{"scan", "duplicate", "manifest_missing_entry", "manifest_bad_json", "manifest_digest"} {
		t.Run(defect, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "a.zip"), []byte("a"), 0600); err != nil {
				t.Fatal(err)
			}
			h, err := sha256File(filepath.Join(dir, "a.zip"))
			if err != nil {
				t.Fatal(err)
			}
			sums := h + "  a.zip\n"
			switch defect {
			case "scan":
				sums += strings.Repeat("x", 70000) + "\n"
			case "duplicate":
				sums += h + "  a.zip\n"
			case "manifest_bad_json":
				os.WriteFile(filepath.Join(dir, "source-manifest.json"), []byte("{broken"), 0600)
			default:
				file, digest := "a.zip", strings.Repeat("a", 64)
				if defect == "manifest_missing_entry" {
					file, digest = "b.zip", h
				}
				os.WriteFile(filepath.Join(dir, "source-manifest.json"), []byte(`{"artifacts":[{"target":"linux/amd64","file":"`+file+`","sha256":"`+digest+`"}]}`), 0600)
			}
			os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(sums), 0600)
			for _, jsonFlag := range []bool{false, true} {
				args := []string{"release", "verify", "--dir", dir}
				if jsonFlag {
					args = append(args, "--json")
				}
				var out, errOut bytes.Buffer
				if Run(args, "evolution", &out, &errOut) != 3 {
					t.Fatal("incomplete release accepted")
				}
			}
		})
	}
}

func TestReleaseRejectsSymlinkArtifact(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	path := filepath.Join(outside, "outside.zip")
	os.WriteFile(path, []byte("not in release"), 0600)
	if err := os.Symlink(path, filepath.Join(dir, "a.zip")); err != nil {
		t.Skip("symlinks unavailable")
	}
	h, _ := sha256File(path)
	os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(h+"  a.zip\n"), 0600)
	r, err := VerifyRelease(dir)
	if err != nil || len(r.HashErrors) == 0 {
		t.Fatal("symlink artifact accepted", err)
	}
}

func TestSandboxRedactsSavedAndDisplayedOutput(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/redaction\n\ngo 1.21\n"), 0600)
	os.WriteFile(filepath.Join(dir, "x_test.go"), []byte(`package redaction
import("fmt";"os";"testing")
func TestOutput(t *testing.T){fmt.Print("{\"password\":\"");fmt.Print("synthetic-private-value\"}\n");fmt.Fprintln(os.Stderr,"Authorization: Bearer synthetic-bearer-value");t.Fatal("normal failure remains")}`), 0600)
	p, err := UseProject("redaction", dir, true)
	if err != nil {
		t.Fatal(err)
	}
	var live bytes.Buffer
	r, err := RunSandbox(p, "go-test", true, time.Minute, &live)
	if err != nil {
		t.Fatal(err)
	}
	log, err := os.ReadFile(r.LogFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{string(log), live.String()} {
		if strings.Contains(output, "synthetic-private-value") || strings.Contains(output, "synthetic-bearer-value") {
			t.Fatal("credential leaked")
		}
		if !strings.Contains(output, "normal failure remains") || !strings.Contains(output, "[REDACTED]") {
			t.Fatal("redacted output missing")
		}
	}
	if r.Result != "FAILED" || r.ExitCode == 0 {
		t.Fatal("actual failing command must fail")
	}
}

func TestTruncatedPrivateKeyAndConcurrentBuffer(t *testing.T) {
	b := &cappedBuffer{}
	b.Write([]byte("-----BEGIN PRIVATE KEY-----\n"))
	b.Write(bytes.Repeat([]byte("synthetic-private-body"), maxLog))
	clean, n := RedactSecrets(b.Bytes())
	if !b.truncated || n != 1 || bytes.Contains(clean, []byte("synthetic-private-body")) {
		t.Fatal("truncated private key leaked")
	}
}

func TestTaskCannotRedirectSavedRunDirectory(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "probe")); err != nil {
		t.Skip("symlinks unavailable")
	}
	os.Remove(filepath.Join(dir, "probe"))
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/swapped\n\ngo 1.25\n"), 0600)
	os.WriteFile(filepath.Join(dir, "x_test.go"), []byte(`package swapped
import("os";"testing")
func TestSwap(t *testing.T){ if e:=os.Rename(".evolution/runs",".evolution/original"); e!=nil {t.Fatal(e)}; if e:=os.Symlink(`+fmt.Sprintf("%q", outside)+`,".evolution/runs"); e!=nil {t.Fatal(e)} }`), 0600)
	p, err := UseProject("swapped", dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RunSandbox(p, "go-test", true, time.Minute, &bytes.Buffer{}); err == nil {
		t.Fatal("swapped metadata accepted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatal("output escaped approved directory")
	}
}

func TestQuotedCredentialsAtOutputCapStayRedacted(t *testing.T) {
	for _, quote := range []byte{'"', '\''} {
		b := &cappedBuffer{}
		b.Write(bytes.Repeat([]byte("x"), maxLog-48))
		b.Write([]byte("\npassword=" + string(quote) + "alpha beta gamma delta epsilon zeta eta theta"))
		clean, _ := RedactSecrets(b.Bytes())
		if !b.truncated || bytes.Contains(clean, []byte("alpha")) || bytes.Contains(clean, []byte("beta")) || bytes.Contains(clean, []byte("gamma")) {
			t.Fatal("capped quoted credential leaked")
		}
	}
}

func TestCappedQuotedCredentialsNeverReachSavedOrDisplayedLog(t *testing.T) {
	for _, quote := range []string{`"`, `'`} {
		t.Run(quote, func(t *testing.T) {
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/capped\n\ngo 1.25\n"), 0600)
			// A compiled fixed-task output fixture avoids go-test's own output filtering.
			// Real go-test failure behavior is covered independently above.
			tools := t.TempDir()
			source := filepath.Join(tools, "fixture.go")
			os.WriteFile(source, []byte(`package main
import("fmt";"strings")
func main(){fmt.Print(strings.Repeat("x",1048576-150)); fmt.Print("\npassword="+`+fmt.Sprintf("%q", quote)+`+strings.Repeat("synthetic-secret-fragment ",30))}`), 0600)
			name := "go"
			if runtime.GOOS == "windows" {
				name += ".exe"
			}
			build := exec.Command("go", "build", "-o", filepath.Join(tools, name), source)
			if output, err := build.CombinedOutput(); err != nil {
				t.Fatalf("output fixture compile failed: %s", output)
			}
			t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
			p, err := UseProject("capped", dir, true)
			if err != nil {
				t.Fatal(err)
			}
			var display bytes.Buffer
			rec, err := RunSandbox(p, "go-test", true, time.Minute, &display)
			if err != nil {
				t.Fatal(err)
			}
			log, err := os.ReadFile(rec.LogFile)
			if err != nil {
				t.Fatal(err)
			}
			for _, out := range [][]byte{log, display.Bytes()} {
				if bytes.Contains(out, []byte("synthetic-secret-fragment")) || !bytes.Contains(out, []byte("[REDACTED]")) {
					t.Fatal("capped secret escaped into saved/displayed output")
				}
			}
			if !rec.Truncated {
				t.Fatal("fixture did not reach output cap")
			}
		})
	}
}
