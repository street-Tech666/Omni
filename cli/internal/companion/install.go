package companion

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

const receiptName = "evolution-source-install.json"

type receipt struct {
	Format int               `json:"format"`
	Kind   string            `json:"kind"`
	Files  map[string]string `json:"files"`
}

func digest(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func binaryName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
func allowedFile(name string) bool {
	return name == binaryName("evolution") || name == binaryName("omni")
}

// Require an explicit absolute dedicated directory beneath existing, non-symlink
// parents. No PATH/shell edits, recursive deletion or elevated permissions.
func safeDirectory(dir string) error {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir || dir == filepath.VolumeName(dir)+string(os.PathSeparator) {
		return errors.New("choose a clean absolute new dedicated directory, not a root")
	}
	for p := dir; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
			return errors.New("directory path contains a symlink or non-directory")
		}
		if err != nil && !os.IsNotExist(err) {
			return errors.New("directory could not be checked")
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
	}
	parent, err := os.Stat(filepath.Dir(dir))
	if err != nil || !parent.IsDir() {
		return errors.New("parent directory must already exist")
	}
	return nil
}

func Install(dir string, approve, withOmni bool) error {
	exe, err := os.Executable()
	if err != nil {
		return errors.New("source executable unavailable")
	}
	return installFrom(exe, dir, approve, withOmni, os.Getenv("PATH"))
}
func installFrom(exe, dir string, approve, withOmni bool, path string) error {
	if !approve {
		return errors.New("explicit --approve required; no changes made")
	}
	if err := safeDirectory(dir); err != nil {
		return err
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		return errors.New("destination already exists; nothing overwritten")
	}
	if len(FindCommands("evolution", path, runtime.GOOS)) > 0 {
		return errors.New("evolution already resolves in PATH; use the source by its explicit path, do not overwrite it")
	}
	if withOmni && len(FindCommands("omni", path, runtime.GOOS)) > 0 {
		return errors.New("omni already resolves in PATH; omit --with-omni and use evolution omni")
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		return errors.New("source executable could not be read")
	}
	// Mkdir is exclusive; refuse a concurrent pre-existing destination.
	if err = os.Mkdir(dir, 0700); err != nil {
		return errors.New("could not create dedicated directory")
	}
	created := []string{}
	rollback := func() {
		for _, name := range created {
			_ = os.Remove(filepath.Join(dir, name))
		}
		_ = os.Remove(dir)
	}
	r := receipt{1, "UNSIGNED_SOURCE_ONLY", map[string]string{}}
	names := []string{binaryName("evolution")}
	if withOmni {
		names = append(names, binaryName("omni"))
	}
	for _, name := range names {
		if err = writeExclusive(filepath.Join(dir, name), data, 0700); err != nil {
			rollback()
			return errors.New("copy failed; attempted rollback of created files only")
		}
		created = append(created, name)
		r.Files[name] = digest(data)
	}
	metadata, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		rollback()
		return err
	}
	if err = writeExclusive(filepath.Join(dir, receiptName), metadata, 0600); err != nil {
		rollback()
		return errors.New("receipt failed; attempted rollback of created files only")
	}
	return nil
}
func writeExclusive(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return errors.New("could not write source file")
	}
	return nil
}

func Uninstall(dir string, approve bool) error {
	if !approve {
		return errors.New("explicit --approve required; no changes made")
	}
	if err := safeDirectory(dir); err != nil {
		return err
	}
	metadataPath := filepath.Join(dir, receiptName)
	info, err := os.Lstat(metadataPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return errors.New("missing or invalid local source receipt")
	}
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return errors.New("receipt unreadable")
	}
	r := receipt{}
	if json.Unmarshal(data, &r) != nil || r.Format != 1 || r.Kind != "UNSIGNED_SOURCE_ONLY" || len(r.Files) < 1 || len(r.Files) > 2 || r.Files[binaryName("evolution")] == "" {
		return errors.New("unrecognized source receipt")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errors.New("directory unreadable")
	}
	if len(entries) != len(r.Files)+1 {
		return errors.New("extra user files present; no files removed")
	}
	// Validate every file before removing any. Local receipt hashes are checks,
	// not publisher signatures; this never authenticates a distributed artifact.
	for name, expected := range r.Files {
		if !allowedFile(name) {
			return errors.New("unrecognized filename; no files removed")
		}
		p := filepath.Join(dir, name)
		info, err := os.Lstat(p)
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("missing, symlinked or modified files; no files removed")
		}
		h, err := hashFile(p)
		if err != nil || h != expected {
			return errors.New("source copy changed; no files removed")
		}
	}
	for name := range r.Files {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("uninstall incomplete; stop and inspect dedicated directory: %w", err)
		}
	}
	if err = os.Remove(metadataPath); err != nil {
		return errors.New("uninstall incomplete; receipt could not be removed")
	}
	return os.Remove(dir)
}
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
