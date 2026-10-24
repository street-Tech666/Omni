package companion

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDiscoverRequiresExplicitApproval(t *testing.T) {
	for _, args := range [][]string{{"discover"}, {"discover", "--json"}, {"discover", "--approve", "--approve"}, {"discover", "--approve", "--upload"}} {
		var out, err bytes.Buffer
		if Run(args, "evolution", &out, &err) != 2 || out.Len() != 0 {
			t.Fatal(args)
		}
	}
}

func TestDiscoverUsesPathNamesWithoutExecutingOrExposingPaths(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "executed")
	script := "#!/bin/sh\ntouch " + marker + "\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	var out, errOut bytes.Buffer
	if Run([]string{"discover", "--approve", "--json"}, "evolution", &out, &errOut) != 0 {
		t.Fatal(errOut.String())
	}
	var report DiscoveryReport
	if json.Unmarshal(out.Bytes(), &report) != nil || report.Transmission != "NONE; local output only" {
		t.Fatal(out.String())
	}
	if !report.Results[0].Detected || report.Results[0].Matches != 1 || report.Results[1].Detected {
		t.Fatal(report.Results)
	}
	if strings.Contains(out.String(), dir) {
		t.Fatal("machine path exposed")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("discovered tool executed")
	}
}

func TestDiscoverIgnoresNonExecutableUnixFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix mode rule")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte("inert"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, r := range Discover(dir, runtime.GOOS).Results {
		if r.Tool.ID == "docker" && r.Detected {
			t.Fatal("non-executable detected")
		}
	}
}

func TestDiscoverEmptyPathCannotScanWorkingDirectory(t *testing.T) {
	for _, result := range Discover("", runtime.GOOS).Results {
		if result.Detected || result.Matches != 0 {
			t.Fatal("empty PATH inspected a directory")
		}
	}
}

func TestDiscoverWindowsNameRules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "NODE.EXE"), []byte("inert"), 0600); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, result := range Discover(dir, "windows").Results {
		if result.Tool.ID == "node" {
			found = result.Detected && result.Matches == 1
		}
	}
	if !found {
		t.Fatal("Windows case/extension rule not applied")
	}
}

func TestDiscoverRejectsIndirectOrRemotePathEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte("inert"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".", "relative", "//server/share", `\\server\share`} {
		if discoveryMatches("git", path, runtime.GOOS) != 0 {
			t.Fatal("unsafe entry accepted")
		}
	}
	if runtime.GOOS == "windows" {
		return
	}
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if discoveryMatches("git", link, runtime.GOOS) != 0 {
		t.Fatal("symlink directory followed")
	}
	if err := os.Symlink(filepath.Join(dir, "git"), filepath.Join(dir, "node")); err != nil {
		t.Fatal(err)
	}
	if discoveryMatches("node", dir, runtime.GOOS) != 0 {
		t.Fatal("symlink executable followed")
	}
}
