package companion

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProjectRequiresApprovalAndAbsoluteDir(t *testing.T) {
	dir := t.TempDir()
	if _, err := UseProject("demo", dir, false); err == nil {
		t.Fatal("expected approval requirement")
	}
	if _, err := UseProject("demo", "relative", true); err == nil {
		t.Fatal("expected absolute path requirement")
	}
	if _, err := UseProject("Bad Name", dir, true); err == nil {
		t.Fatal("expected name validation")
	}
	p, err := UseProject("demo", dir, true)
	if err != nil || p.Sandbox != "local" {
		t.Fatal(err)
	}
	if got, err := LoadProject(dir); err != nil || got.Name != "demo" {
		t.Fatal("profile should reload", err)
	}
}

func TestProjectRejectsSymlinkedMetadataAndInvalidSandbox(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, ".evolution")); err != nil {
		t.Skip("symlinks unavailable")
	}
	if _, err := UseProject("demo", dir, true); err == nil {
		t.Fatal("metadata symlink accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "project.json")); !os.IsNotExist(err) {
		t.Fatal("outside project changed")
	}
	os.Remove(filepath.Join(dir, ".evolution"))
	p, err := UseProject("demo", dir, true)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(profilePath(dir), []byte(`{"name":"demo","root":"`+dir+`","sandbox":"cloud"}`), 0600)
	if _, err := LoadProject(dir); err == nil {
		t.Fatal("unsupported sandbox accepted")
	}
	p.Sandbox = "cloud"
	if _, err := RunSandbox(p, "go-test", true, time.Second, &bytes.Buffer{}); err == nil {
		t.Fatal("unsupported sandbox executed")
	}
}

func TestRouteNeverClaimsModel(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n\ngo 1.21\n"), 0o600)
	p, _ := UseProject("demo", dir, true)
	r, err := RouteGoal(p, "add a health check")
	if err != nil || !strings.HasPrefix(r.Model, "NOT CONNECTED") || len(r.Tasks) != 2 {
		t.Fatalf("unexpected route %+v %v", r, err)
	}
}

func TestSandboxRejectsUnknownTaskAndUnapproved(t *testing.T) {
	p, _ := UseProject("demo", t.TempDir(), true)
	if _, err := RunSandbox(p, "rm-rf", true, time.Second, &bytes.Buffer{}); err == nil {
		t.Fatal("arbitrary tasks must be rejected")
	}
	if _, err := RunSandbox(p, "go-test", false, time.Second, &bytes.Buffer{}); err == nil {
		t.Fatal("approval required")
	}
}

func TestSandboxRecordsRealFailingAndPassingRuns(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n\ngo 1.21\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "x_test.go"), []byte("package x\nimport \"testing\"\nfunc TestFail(t *testing.T){ t.Fatal(\"boom\") }\n"), 0o600)
	p, _ := UseProject("demo", dir, true)
	rec, err := RunSandbox(p, "go-test", true, 2*time.Minute, &bytes.Buffer{})
	if err != nil || rec.Result != "FAILED" || rec.ExitCode == 0 {
		t.Fatalf("failing test must be reported as failed: %+v %v", rec, err)
	}
	log, _ := os.ReadFile(rec.LogFile)
	if !strings.Contains(string(log), "boom") || !strings.Contains(string(log), "LOCAL PROCESS ONLY") {
		t.Fatal("log must contain real output and isolation label")
	}
	os.WriteFile(filepath.Join(dir, "x_test.go"), []byte("package x\nimport \"testing\"\nfunc TestOK(t *testing.T){}\n"), 0o600)
	if rec, _ = RunSandbox(p, "go-test", true, 2*time.Minute, &bytes.Buffer{}); rec.Result != "PASSED" {
		t.Fatalf("expected pass, got %+v", rec)
	}
}

func TestSandboxDoesNotLeakTokens(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "secret-value")
	for _, kv := range minimalEnv() {
		if strings.HasPrefix(kv, "GITHUB_TOKEN=") {
			t.Fatal("token inherited")
		}
	}
}

func TestVerifyReleaseDetectsTamperAndUnsigned(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.zip"), []byte("hello"), 0o600)
	sum, _ := sha256File(filepath.Join(dir, "a.zip"))
	os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(sum+"  a.zip\n"), 0o600)
	r, _ := VerifyRelease(dir)
	if r.Signed != "UNSIGNED" || len(r.HashErrors) != 0 {
		t.Fatalf("unexpected %+v", r)
	}
	os.WriteFile(filepath.Join(dir, "source-manifest.json"), []byte(`{"signed":true}`), 0o600)
	if r, _ = VerifyRelease(dir); !strings.HasPrefix(r.Verdict, "REJECT") {
		t.Fatal("false signed claim must be rejected")
	}
	os.Remove(filepath.Join(dir, "source-manifest.json"))
	os.WriteFile(filepath.Join(dir, "a.zip"), []byte("hellO"), 0o600)
	if r, _ = VerifyRelease(dir); !strings.HasPrefix(r.Verdict, "REJECT") {
		t.Fatal("single-byte tamper must be rejected")
	}
	os.WriteFile(filepath.Join(dir, "a.zip"), []byte("hello"), 0o600)
	os.WriteFile(filepath.Join(dir, "a.zip.sig"), []byte("x"), 0o600)
	if r, _ = VerifyRelease(dir); !strings.Contains(r.Signed, "NOT VERIFIED HERE") {
		t.Fatal("presence of a .sig file must not be reported as verified")
	}
}
