package companion

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ProjectProfile is the explicit, owner-approved target for OMNI routing and local sandbox tasks.
// It is stored only inside the chosen project directory, never in global config or shell files.
type ProjectProfile struct {
	Name      string `json:"name"`
	Root      string `json:"root"`
	Sandbox   string `json:"sandbox"` // "local" only; "cloud" is BLOCKED until a sandbox service exists
	CreatedAt string `json:"created_at"`
}

const profileDir = ".evolution"

var projectName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// Fixed argv only; no shell, no user-supplied commands.
type sandboxTask struct {
	argv   []string
	marker string
}

var sandboxTasks = map[string]sandboxTask{
	"go-test":  {[]string{"go", "test", "-count=1", "./..."}, "go.mod"},
	"go-vet":   {[]string{"go", "vet", "./..."}, "go.mod"},
	"go-build": {[]string{"go", "build", "-trimpath", "./..."}, "go.mod"},
	"npm-test": {[]string{"npm", "test", "--silent"}, "package.json"},
}

func profilePath(root string) string { return filepath.Join(root, profileDir, "project.json") }

func validateProjectRoot(root string) error {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return errors.New("choose a clean absolute project path")
	}
	for path := root; ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("project path must contain only existing non-symlink directories")
		}
		if filepath.Dir(path) == path {
			break
		}
	}
	for _, path := range []string{filepath.Join(root, profileDir), filepath.Join(root, profileDir, "runs"), filepath.Join(root, profileDir, "changes")} {
		info, err := os.Lstat(path)
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return errors.New("project metadata directory cannot be a symlink or file")
		}
		if err != nil && !os.IsNotExist(err) {
			return errors.New("project metadata directory could not be checked")
		}
	}
	if info, err := os.Lstat(profilePath(root)); err == nil && !info.Mode().IsRegular() {
		return errors.New("project profile cannot be a symlink or directory")
	} else if err != nil && !os.IsNotExist(err) {
		return errors.New("project profile could not be checked")
	}
	return nil
}

func UseProject(name, dir string, approve bool) (ProjectProfile, error) {
	var p ProjectProfile
	if !approve {
		return p, errors.New("add --approve to write .evolution/project.json in that directory")
	}
	if !projectName.MatchString(name) {
		return p, errors.New("project name must be lowercase letters, numbers or hyphens")
	}
	if !filepath.IsAbs(dir) {
		return p, errors.New("--dir must be an absolute path")
	}
	root := filepath.Clean(dir)
	if err := validateProjectRoot(root); err != nil {
		return p, err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return p, errors.New("--dir must be an existing directory")
	}
	if err := os.MkdirAll(filepath.Join(root, profileDir, "runs"), 0o700); err != nil {
		return p, err
	}
	p = ProjectProfile{name, root, "local", time.Now().UTC().Format(time.RFC3339)}
	data, _ := json.MarshalIndent(p, "", "  ")
	return p, os.WriteFile(profilePath(root), append(data, '\n'), 0o600)
}

func LoadProject(dir string) (ProjectProfile, error) {
	var p ProjectProfile
	if !filepath.IsAbs(dir) {
		return p, errors.New("--project must be an absolute path")
	}
	if err := validateProjectRoot(filepath.Clean(dir)); err != nil {
		return p, err
	}
	data, err := os.ReadFile(profilePath(filepath.Clean(dir)))
	if err != nil {
		return p, errors.New("no project profile; run evolution project use first")
	}
	if json.Unmarshal(data, &p) != nil || p.Root != filepath.Clean(dir) || !projectName.MatchString(p.Name) || p.Sandbox != "local" {
		return p, errors.New("project profile is invalid or was moved; re-run evolution project use")
	}
	return p, nil
}

// RouteRequest describes how OMNI would route a goal. It never calls a model.
type RouteRequest struct {
	Project  string   `json:"project"`
	Goal     string   `json:"goal"`
	Roles    []string `json:"roles"`
	Sandbox  string   `json:"sandbox"`
	Tasks    []string `json:"suggested_tasks"`
	Model    string   `json:"model"`
	Approval string   `json:"approval"`
}

func RouteGoal(p ProjectProfile, goal string) (RouteRequest, error) {
	goal = strings.TrimSpace(goal)
	if len(goal) < 5 || len(goal) > 500 {
		return RouteRequest{}, errors.New("goal must be 5-500 characters")
	}
	tasks := []string{}
	for _, id := range []string{"go-test", "go-vet", "npm-test"} {
		if _, err := os.Stat(filepath.Join(p.Root, sandboxTasks[id].marker)); err == nil {
			tasks = append(tasks, id)
		}
	}
	return RouteRequest{p.Name, goal, []string{"OmniForge coder (planned)", "independent reviewer/tester", "Guardian checks"}, "local (cloud sandbox BLOCKED: no service)", tasks,
		"NOT CONNECTED: no model call made", "Run each task yourself with evolution sandbox run --approve"}, nil
}

type RunRecord struct {
	ID        string   `json:"id"`
	Project   string   `json:"project"`
	Task      string   `json:"task"`
	Change    string   `json:"change,omitempty"`
	Argv      []string `json:"argv"`
	Started   string   `json:"started"`
	Seconds   float64  `json:"seconds"`
	ExitCode  int      `json:"exit_code"`
	Result    string   `json:"result"`
	LogFile   string   `json:"log_file"`
	Truncated bool     `json:"truncated"`
}

const maxLog = 1 << 20

type cappedBuffer struct {
	mu sync.Mutex
	buffer bytes.Buffer
	truncated bool
}

func (c *cappedBuffer) Bytes() []byte { c.mu.Lock(); defer c.mu.Unlock(); return append([]byte(nil), c.buffer.Bytes()...) }
func (c *cappedBuffer) Len() int { return c.buffer.Len() }

func (c *cappedBuffer) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	room := maxLog - c.Len()
	if room <= 0 {
		c.truncated = true
		return len(b), nil
	}
	if len(b) > room {
		c.truncated = true
		c.buffer.Write(b[:room])
		return len(b), nil
	}
	return c.buffer.Write(b)
}

// minimalEnv passes only what toolchains need, so tokens in the parent shell are not inherited.
func minimalEnv() []string {
	keep := []string{"PATH", "HOME", "USERPROFILE", "TMPDIR", "TEMP", "TMP", "SYSTEMROOT", "GOPATH", "GOCACHE", "GOMODCACHE", "LANG"}
	env := []string{"CI=1", "EVOLUTION_SANDBOX=local"}
	for _, k := range keep {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

func RunSandbox(p ProjectProfile, task string, approve bool, timeout time.Duration, live io.Writer) (RunRecord, error) {
	return runSandbox(p, task, "", approve, timeout, live)
}

func runSandbox(p ProjectProfile, task, changeID string, approve bool, timeout time.Duration, live io.Writer) (RunRecord, error) {
	var rec RunRecord
	t, ok := sandboxTasks[task]
	if !ok {
		return rec, errors.New("unknown task; allowed: go-test, go-vet, go-build, npm-test")
	}
	if !approve {
		return rec, errors.New("add --approve to run this task in the project directory")
	}
	if err := validateProjectRoot(p.Root); err != nil {
		return rec, err
	}
	if p.Sandbox != "local" || !projectName.MatchString(p.Name) {
		return rec, errors.New("invalid local project profile")
	}
	if _, err := os.Stat(filepath.Join(p.Root, t.marker)); err != nil {
		return rec, fmt.Errorf("task %s needs %s in the project root", task, t.marker)
	}
	bin, err := exec.LookPath(t.argv[0])
	if err != nil {
		return rec, fmt.Errorf("%s is not installed on this machine", t.argv[0])
	}
	idBytes := make([]byte, 6)
	if _, err := rand.Read(idBytes); err != nil {
		return rec, errors.New("run identifier could not be generated")
	}
	// Keep a descriptor to the approved run directory before executing project code.
	// Child renames/symlink substitutions cannot redirect these writes elsewhere.
	runs := filepath.Join(p.Root, profileDir, "runs")
	if err := os.MkdirAll(runs, 0o700); err != nil {
		return rec, err
	}
	if err := validateProjectRoot(p.Root); err != nil {
		return rec, err
	}
	runRoot, err := os.OpenRoot(runs)
	if err != nil {
		return rec, err
	}
	defer runRoot.Close()
	original, err := runRoot.Stat(".")
	if err != nil {
		return rec, err
	}
	rec = RunRecord{ID: time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(idBytes), Project: p.Name, Task: task, Change: changeID, Argv: t.argv, Started: time.Now().UTC().Format(time.RFC3339)}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, t.argv[1:]...)
	// Bound waiting on inherited stdout/stderr pipes; not process-tree isolation.
	cmd.WaitDelay = 500 * time.Millisecond
	cmd.Dir, cmd.Env = p.Root, minimalEnv()
	buf := &cappedBuffer{}
	// One synchronized capped sink avoids stdout/stderr races. Display only
	// after redaction, so terminal capture cannot expose raw child credentials.
	cmd.Stdout, cmd.Stderr = buf, buf
	start := time.Now()
	runErr := cmd.Run()
	rec.Seconds = float64(time.Since(start).Milliseconds()) / 1000
	rec.Truncated = buf.truncated
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		rec.ExitCode, rec.Result = -1, "TIMED OUT"
	case runErr != nil:
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			rec.ExitCode, rec.Result = ee.ExitCode(), "FAILED"
		} else {
			rec.ExitCode, rec.Result = -1, "COULD NOT START"
		}
	default:
		rec.Result = "PASSED"
	}
	if err := validateProjectRoot(p.Root); err != nil {
		return rec, err
	}
	current, err := os.Stat(runs)
	if err != nil || !os.SameFile(original, current) {
		return rec, errors.New("run directory changed during task; output was not saved")
	}
	rec.LogFile = filepath.Join(runs, rec.ID+".log")
	header := fmt.Sprintf("# evolution sandbox run %s\n# project=%s task=%s platform=%s/%s argv=%q\n# isolation: LOCAL PROCESS ONLY (not a container or VM)\n", rec.ID, p.Name, task, runtime.GOOS, runtime.GOARCH, t.argv)
	if rec.Change != "" {
		header += fmt.Sprintf("# change=%s (reviewed file-level manifest bound to this run)\n", rec.Change)
	}
	clean, n := RedactSecrets(buf.Bytes())
	if live != nil {
		if _, err := live.Write(clean); err != nil {
			return rec, err
		}
	}
	if n > 0 {
		header += fmt.Sprintf("# redacted %d secret-like value(s) before saving\n", n)
	}
	if err := writeRunFile(runRoot, rec.ID+".log", append([]byte(header), clean...)); err != nil {
		return rec, err
	}
	meta, _ := json.MarshalIndent(rec, "", "  ")
	return rec, writeRunFile(runRoot, rec.ID+".json", append(meta, '\n'))
}

func writeRunFile(root *os.Root, name string, data []byte) error {
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = root.Remove(name)
		return errors.New("run output could not be saved")
	}
	return nil
}
