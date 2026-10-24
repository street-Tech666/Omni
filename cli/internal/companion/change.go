package companion

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Change proposals record a file-level fingerprint snapshot of the targeted
// project so a later working-tree state can be reviewed before tests run.
// Only hashes and sizes are stored — never file contents — so proposals
// cannot leak source or secrets. This is a review manifest, not a line diff
// and not an agent-generated change.

type FileFingerprint struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type ChangeRecord struct {
	ID        string                      `json:"id"`
	Project   string                      `json:"project"`
	Summary   string                      `json:"summary"`
	CreatedAt string                      `json:"created_at"`
	Files     map[string]FileFingerprint  `json:"files"`
	Skipped   int                         `json:"skipped_oversized"`
	Note      string                      `json:"note"`
}

type ChangeDiffEntry struct {
	Path      string `json:"path"`
	Status    string `json:"status"` // added | modified | removed
	OldSHA256 string `json:"old_sha256,omitempty"`
	NewSHA256 string `json:"new_sha256,omitempty"`
	OldSize   int64  `json:"old_size,omitempty"`
	NewSize   int64  `json:"new_size,omitempty"`
}

type ChangeDiff struct {
	ChangeID  string            `json:"change_id"`
	Project   string            `json:"project"`
	Summary   string            `json:"summary"`
	Added     int               `json:"added"`
	Modified  int               `json:"modified"`
	Removed   int               `json:"removed"`
	Unchanged int               `json:"unchanged"`
	Entries   []ChangeDiffEntry `json:"entries"`
	Note      string            `json:"note"`
}

const (
	maxSnapshotFiles   = 5000
	maxSnapshotFile    = 1 << 20  // 1 MiB per file
	maxSnapshotTotal   = 256 << 20 // 256 MiB total
	changeNote         = "File-level fingerprint manifest for human review; not a line diff, not an agent-authored change, not test evidence."
)

var changeIDPattern = regexp.MustCompile(`^\d{8}T\d{6}Z-[0-9a-f]{12}$`)

var snapshotSkipDirs = map[string]bool{
	profileDir: true, ".git": true, "node_modules": true,
	"vendor": true, "dist": true, "build": true, ".next": true,
}

// snapshotProject fingerprints regular files under root. Symlinks and
// special files are never followed or read. Contents are hashed, not stored.
func snapshotProject(root string) (map[string]FileFingerprint, int, error) {
	files := map[string]FileFingerprint{}
	skipped, total := 0, int64(0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil // never follow symlinks
		}
		if d.IsDir() {
			if snapshotSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if len(files) >= maxSnapshotFiles {
			return errors.New("project has too many files to snapshot safely")
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxSnapshotFile || total+info.Size() > maxSnapshotTotal {
			skipped++
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil || closeErr != nil {
			return errors.New("file could not be fingerprinted")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return errors.New("file escaped the project root")
		}
		files[filepath.ToSlash(rel)] = FileFingerprint{hex.EncodeToString(h.Sum(nil)), info.Size()}
		total += info.Size()
		return nil
	})
	return files, skipped, err
}

func changesDir(root string) string { return filepath.Join(root, profileDir, "changes") }

func newChangeID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("change identifier could not be generated")
	}
	return time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(b), nil
}

// ChangeNew records a reviewable baseline snapshot inside the project only.
func ChangeNew(p ProjectProfile, summary string, approve bool) (ChangeRecord, error) {
	var rec ChangeRecord
	if !approve {
		return rec, errors.New("add --approve to record a change proposal in that project")
	}
	summary = strings.Join(strings.Fields(summary), " ")
	if len(summary) < 5 || len(summary) > 200 {
		return rec, errors.New("summary must be 5-200 characters")
	}
	if err := validateProjectRoot(p.Root); err != nil {
		return rec, err
	}
	if p.Sandbox != "local" || !projectName.MatchString(p.Name) {
		return rec, errors.New("invalid local project profile")
	}
	files, skipped, err := snapshotProject(p.Root)
	if err != nil {
		return rec, err
	}
	id, err := newChangeID()
	if err != nil {
		return rec, err
	}
	rec = ChangeRecord{ID: id, Project: p.Name, Summary: summary, CreatedAt: time.Now().UTC().Format(time.RFC3339), Files: files, Skipped: skipped, Note: changeNote}
	if err := os.MkdirAll(changesDir(p.Root), 0o700); err != nil {
		return rec, err
	}
	if err := validateProjectRoot(p.Root); err != nil {
		return rec, err
	}
	dir, err := os.OpenRoot(changesDir(p.Root))
	if err != nil {
		return rec, err
	}
	defer dir.Close()
	data, _ := json.MarshalIndent(rec, "", "  ")
	return rec, writeRunFile(dir, rec.ID+".json", append(data, '\n'))
}

// LoadChange reads a recorded proposal, rejecting symlink substitution and
// proposals that belong to a different project profile.
func LoadChange(p ProjectProfile, id string) (ChangeRecord, error) {
	var rec ChangeRecord
	if !changeIDPattern.MatchString(id) {
		return rec, errors.New("invalid change identifier")
	}
	if err := validateProjectRoot(p.Root); err != nil {
		return rec, err
	}
	dir, err := os.OpenRoot(changesDir(p.Root))
	if err != nil {
		return rec, errors.New("no recorded change proposals for this project")
	}
	defer dir.Close()
	data, err := dir.ReadFile(id + ".json")
	if err != nil {
		return rec, errors.New("change proposal not found in this project")
	}
	if json.Unmarshal(data, &rec) != nil || rec.ID != id || rec.Project != p.Name || rec.Files == nil {
		return rec, errors.New("change proposal is invalid or belongs to another project")
	}
	return rec, nil
}

// ListChanges returns recorded proposals for the project, oldest first.
func ListChanges(p ProjectProfile) ([]ChangeRecord, error) {
	if err := validateProjectRoot(p.Root); err != nil {
		return nil, err
	}
	dir, err := os.OpenRoot(changesDir(p.Root))
	if err != nil {
		return nil, errors.New("no recorded change proposals for this project")
	}
	defer dir.Close()
	entries, err := fs.ReadDir(dir.FS(), ".")
	if err != nil {
		return nil, err
	}
	out := []ChangeRecord{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		rec, err := LoadChange(p, id)
		if err != nil {
			continue // skip foreign or malformed proposals rather than trusting them
		}
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// DiffChange compares the recorded baseline with the current working tree.
func DiffChange(p ProjectProfile, id string) (ChangeDiff, error) {
	rec, err := LoadChange(p, id)
	if err != nil {
		return ChangeDiff{}, err
	}
	now, _, err := snapshotProject(p.Root)
	if err != nil {
		return ChangeDiff{}, err
	}
	diff := ChangeDiff{ChangeID: rec.ID, Project: rec.Project, Summary: rec.Summary, Entries: []ChangeDiffEntry{}, Note: changeNote}
	paths := map[string]bool{}
	for path := range rec.Files {
		paths[path] = true
	}
	for path := range now {
		paths[path] = true
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	for _, path := range ordered {
		before, had := rec.Files[path]
		after, has := now[path]
		switch {
		case had && !has:
			diff.Removed++
			diff.Entries = append(diff.Entries, ChangeDiffEntry{Path: path, Status: "removed", OldSHA256: before.SHA256, OldSize: before.Size})
		case !had && has:
			diff.Added++
			diff.Entries = append(diff.Entries, ChangeDiffEntry{Path: path, Status: "added", NewSHA256: after.SHA256, NewSize: after.Size})
		case before.SHA256 != after.SHA256:
			diff.Modified++
			diff.Entries = append(diff.Entries, ChangeDiffEntry{Path: path, Status: "modified", OldSHA256: before.SHA256, NewSHA256: after.SHA256, OldSize: before.Size, NewSize: after.Size})
		default:
			diff.Unchanged++
		}
	}
	return diff, nil
}

// RunSandboxWithChange binds a real task run to a recorded change proposal,
// so the saved log and run record name the exact reviewed state tested.
func RunSandboxWithChange(p ProjectProfile, task, changeID string, approve bool, timeout time.Duration, live io.Writer) (RunRecord, error) {
	change, err := LoadChange(p, changeID)
	if err != nil {
		return RunRecord{}, err
	}
	return runSandbox(p, task, change.ID, approve, timeout, live)
}
