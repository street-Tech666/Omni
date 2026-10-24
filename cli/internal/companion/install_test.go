package companion

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func fixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	// macOS may provide temp paths through /var symlink; production accepts
	// only the resolved explicit directory and still refuses symlink parents.
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "source")
	if err := os.WriteFile(exe, []byte("fictional-executable"), 0700); err != nil {
		t.Fatal(err)
	}
	return exe, filepath.Join(root, "owned-bin")
}
func TestInstallUninstallRoundTrip(t *testing.T) {
	exe, dir := fixture(t)
	if err := installFrom(exe, dir, true, true, ""); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{binaryName("evolution"), binaryName("omni"), receiptName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(filepath.Join(dir, receiptName))
		if info.Mode().Perm() != 0600 {
			t.Fatal("receipt permissions")
		}
	}
	if err := Uninstall(dir, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("owned directory remains")
	}
	if _, err := os.Stat(exe); err != nil {
		t.Fatal("source removed")
	}
}
func TestInstallRequiresApproval(t *testing.T) {
	exe, dir := fixture(t)
	if installFrom(exe, dir, false, false, "") == nil {
		t.Fatal("consent bypass")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("wrote without approval")
	}
}
func TestUninstallRequiresApproval(t *testing.T) {
	exe, dir := fixture(t)
	if err := installFrom(exe, dir, true, false, ""); err != nil {
		t.Fatal(err)
	}
	if Uninstall(dir, false) == nil {
		t.Fatal("consent bypass")
	}
	if _, err := os.Stat(filepath.Join(dir, binaryName("evolution"))); err != nil {
		t.Fatal("removed without approval")
	}
}
func TestInstallRefusesExistingDirectory(t *testing.T) {
	exe, dir := fixture(t)
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if installFrom(exe, dir, true, false, "") == nil {
		t.Fatal("overwrote directory")
	}
}
func TestInstallNameCollisions(t *testing.T) {
	for _, name := range []string{"evolution", "omni"} {
		t.Run(name, func(t *testing.T) {
			exe, dir := fixture(t)
			path := filepath.Dir(exe)
			if err := os.WriteFile(filepath.Join(path, binaryName(name)), []byte("existing-tool"), 0700); err != nil {
				t.Fatal(err)
			}
			if installFrom(exe, dir, true, true, path) == nil {
				t.Fatal("collision overwritten")
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatal("collision created files")
			}
		})
	}
}
func TestOmniCollisionAllowsFallbackOnly(t *testing.T) {
	exe, dir := fixture(t)
	path := filepath.Dir(exe)
	if err := os.WriteFile(filepath.Join(path, binaryName("omni")), []byte("existing-tool"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := installFrom(exe, dir, true, false, path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, binaryName("omni"))); !os.IsNotExist(err) {
		t.Fatal("alias created")
	}
}
func TestUninstallPreservesModifiedBinary(t *testing.T) {
	exe, dir := fixture(t)
	if err := installFrom(exe, dir, true, false, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, binaryName("evolution")), []byte("user-change"), 0700); err != nil {
		t.Fatal(err)
	}
	if Uninstall(dir, true) == nil {
		t.Fatal("removed modified file")
	}
	if _, err := os.Stat(filepath.Join(dir, receiptName)); err != nil {
		t.Fatal("partial deletion")
	}
}
func TestUninstallPreservesExtraUserFiles(t *testing.T) {
	exe, dir := fixture(t)
	if err := installFrom(exe, dir, true, false, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("personal"), 0600); err != nil {
		t.Fatal(err)
	}
	if Uninstall(dir, true) == nil {
		t.Fatal("removed extra file")
	}
	if _, err := os.Stat(filepath.Join(dir, binaryName("evolution"))); err != nil {
		t.Fatal("partial deletion")
	}
}
func TestUninstallRefusesSymlink(t *testing.T) {
	exe, dir := fixture(t)
	if err := installFrom(exe, dir, true, false, ""); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, binaryName("evolution"))
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(exe, p); err != nil {
		t.Skip("symlink unavailable")
	}
	if Uninstall(dir, true) == nil {
		t.Fatal("symlink deleted")
	}
}
func TestInstallRefusesSymlinkParent(t *testing.T) {
	exe, dir := fixture(t)
	link := filepath.Join(filepath.Dir(dir), "link")
	if err := os.Symlink(filepath.Dir(dir), link); err != nil {
		t.Skip("symlink unavailable")
	}
	if installFrom(exe, filepath.Join(link, "owned"), true, false, "") == nil {
		t.Fatal("followed symlink")
	}
}
func TestUnsafeDestination(t *testing.T) {
	for _, dir := range []string{"relative", "/", "/tmp/../tmp/unsafe"} {
		if safeDirectory(dir) == nil {
			t.Fatal(dir)
		}
	}
}
func TestFailedSourceReadCreatesNothing(t *testing.T) {
	_, dir := fixture(t)
	if installFrom(filepath.Join(filepath.Dir(dir), "missing"), dir, true, false, "") == nil {
		t.Fatal("missing source")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("partial directory")
	}
}
func TestUninstallRefusesUnknownReceiptNames(t *testing.T) {
	exe, dir := fixture(t)
	if err := installFrom(exe, dir, true, false, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, receiptName), []byte(`{"format":1,"kind":"UNSIGNED_SOURCE_ONLY","files":{"../../outside":"x"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if Uninstall(dir, true) == nil {
		t.Fatal("accepted traversal")
	}
}
