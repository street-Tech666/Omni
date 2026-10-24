package companion

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func changeProject(t *testing.T) (ProjectProfile, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n\ngo 1.21\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := UseProject("change-test", dir, true)
	if err != nil {
		t.Fatal(err)
	}
	return p, dir
}

func TestChangeNewRequiresApprovalAndValidSummary(t *testing.T) {
	p, _ := changeProject(t)
	if _, err := ChangeNew(p, "a real summary", false); err == nil {
		t.Fatal("approval required")
	}
	if _, err := ChangeNew(p, "hi", true); err == nil {
		t.Fatal("short summary must be rejected")
	}
	if _, err := ChangeNew(p, strings.Repeat("x", 201), true); err == nil {
		t.Fatal("long summary must be rejected")
	}
}

func TestChangeSnapshotSkipsMetadataAndSymlinks(t *testing.T) {
	p, dir := changeProject(t)
	os.MkdirAll(filepath.Join(dir, "node_modules", "dep"), 0o700)
	os.WriteFile(filepath.Join(dir, "node_modules", "dep", "x.js"), []byte("x"), 0o600)
	os.MkdirAll(filepath.Join(dir, ".git"), 0o700)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("x"), 0o600)
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("token"), 0o600)
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(dir, "linked.txt")); err == nil {
		defer os.Remove(filepath.Join(dir, "linked.txt"))
	}
	rec, err := ChangeNew(p, "baseline snapshot", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := rec.Files["go.mod"]; !ok {
		t.Fatal("go.mod missing from snapshot")
	}
	for path := range rec.Files {
		if strings.HasPrefix(path, "node_modules/") || strings.HasPrefix(path, ".git/") || strings.HasPrefix(path, ".evolution/") || path == "linked.txt" {
			t.Fatalf("snapshot must skip %s", path)
		}
	}
	if rec.Note == "" || !strings.Contains(rec.Note, "not a line diff") {
		t.Fatal("honest review-manifest note required")
	}
}

func TestChangeDiffDetectsAddedModifiedRemoved(t *testing.T) {
	p, dir := changeProject(t)
	rec, err := ChangeNew(p, "baseline snapshot", true)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() { println(\"changed\") }\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "added.go"), []byte("package main\n"), 0o600)
	os.Remove(filepath.Join(dir, "go.mod"))
	diff, err := DiffChange(p, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Added != 1 || diff.Modified != 1 || diff.Removed != 1 {
		t.Fatalf("wrong counts: %+v", diff)
	}
	status := map[string]string{}
	for _, e := range diff.Entries {
		status[e.Path] = e.Status
		if e.Status == "modified" && (e.OldSHA256 == "" || e.NewSHA256 == "" || e.OldSHA256 == e.NewSHA256) {
			t.Fatal("modified entry must carry distinct old/new hashes")
		}
	}
	if status["added.go"] != "added" || status["main.go"] != "modified" || status["go.mod"] != "removed" {
		t.Fatalf("wrong statuses: %v", status)
	}
	again, err := DiffChange(p, rec.ID)
	if err != nil || again.Added != diff.Added || again.Modified != diff.Modified || again.Removed != diff.Removed {
		t.Fatal("diff must be reproducible")
	}
}

func TestLoadChangeRejectsBadIDAndForeignProject(t *testing.T) {
	p, dir := changeProject(t)
	rec, _ := ChangeNew(p, "baseline snapshot", true)
	if _, err := LoadChange(p, "../../etc/passwd"); err == nil {
		t.Fatal("path-like id must be rejected")
	}
	if _, err := LoadChange(p, "20060102T150405Z-ffffffffffff"); err == nil {
		t.Fatal("unknown id must be rejected")
	}
	other := p
	other.Name = "other-project"
	if _, err := LoadChange(other, rec.ID); err == nil {
		t.Fatal("foreign project binding must be rejected")
	}
	path := filepath.Join(dir, ".evolution", "changes", rec.ID+".json")
	data, _ := os.ReadFile(path)
	var tampered ChangeRecord
	if json.Unmarshal(data, &tampered) != nil {
		t.Fatal("record must parse")
	}
	tampered.Project = "other-project"
	forged, _ := json.Marshal(tampered)
	os.WriteFile(path, forged, 0o600)
	if _, err := LoadChange(p, rec.ID); err == nil {
		t.Fatal("tampered project binding must be rejected")
	}
}

func TestChangeBoundSandboxRunRecordsManifest(t *testing.T) {
	p, dir := changeProject(t)
	rec, _ := ChangeNew(p, "baseline snapshot", true)
	if _, err := RunSandboxWithChange(p, "go-test", "20060102T150405Z-ffffffffffff", true, time.Minute, &bytes.Buffer{}); err == nil {
		t.Fatal("unknown change must block the run")
	}
	if _, err := RunSandboxWithChange(p, "go-test", rec.ID, false, time.Minute, &bytes.Buffer{}); err == nil {
		t.Fatal("approval still required with a change binding")
	}
	run, err := RunSandboxWithChange(p, "go-build", rec.ID, true, 2*time.Minute, &bytes.Buffer{})
	if err != nil {
		t.Skipf("go toolchain unavailable: %v", err)
	}
	if run.Change != rec.ID {
		t.Fatalf("run record must name the reviewed change: %+v", run)
	}
	saved, err := os.ReadFile(filepath.Join(dir, ".evolution", "runs", run.ID+".json"))
	if err != nil {
		t.Fatal("run record must be saved")
	}
	var parsed RunRecord
	if json.Unmarshal(saved, &parsed) != nil || parsed.Change != rec.ID {
		t.Fatal("saved run record must carry the change binding")
	}
	log, _ := os.ReadFile(run.LogFile)
	if !strings.Contains(string(log), "change="+rec.ID) {
		t.Fatal("saved log must name the reviewed change")
	}
}

func TestSupportMatrixNeverClaimsInstallerOrSignatures(t *testing.T) {
	entries := SupportMatrix()
	if len(entries) != 6 {
		t.Fatalf("six declared targets expected, got %d", len(entries))
	}
	for _, e := range entries {
		joined := strings.Join([]string{e.SourceBuild, e.NativeExecution, e.Signing, e.Install, e.Update, e.Rollback, e.Uninstall}, " ")
		if strings.Contains(joined, "SIGNATURE VERIFIED") || strings.Contains(joined, "installer available") {
			t.Fatalf("matrix must not claim signing/installer support: %s", e.Target)
		}
		if !strings.Contains(e.Signing, "UNSIGNED") || !strings.Contains(e.Install, "NOT AVAILABLE") {
			t.Fatalf("matrix must stay honest for %s", e.Target)
		}
	}
}

func RunS(a []string, o, e *bytes.Buffer) int { return Run(a, "evolution", o, e) }

func TestCLIChangeAndMatrixCommands(t *testing.T) {
	p, dir := changeProject(t)
	var out, errOut bytes.Buffer
	if code := RunS([]string{"change", "new", "baseline snapshot", "--project", dir}, &out, &errOut); code != 1 {
		t.Fatalf("unapproved change must fail, got %d", code)
	}
	out.Reset()
	if code := RunS([]string{"change", "new", "baseline snapshot", "--project", dir, "--approve"}, &out, &errOut); code != 0 {
		t.Fatalf("change new failed: %d %s", code, errOut.String())
	}
	recs, err := ListChanges(p)
	if err != nil || len(recs) != 1 {
		t.Fatalf("proposal must be listed: %v %v", recs, err)
	}
	out.Reset()
	if code := RunS([]string{"change", "diff", recs[0].ID, "--project", dir}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "unchanged") {
		t.Fatalf("diff failed: %d %s %s", code, out.String(), errOut.String())
	}
	out.Reset()
	if code := RunS([]string{"release", "matrix"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "UNSIGNED") || !strings.Contains(out.String(), "NOT AVAILABLE") {
		t.Fatalf("matrix must be honest: %d %s", code, out.String())
	}
	out.Reset()
	if code := RunS([]string{"release", "matrix", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("matrix json failed: %d", code)
	}
	var parsed []SupportMatrixEntry
	if json.Unmarshal(out.Bytes(), &parsed) != nil || len(parsed) != 6 {
		t.Fatal("matrix JSON must parse with six targets")
	}
}
