package companion

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"
)

// flags parses --key value / --flag pairs; returns positional args.
func flags(args []string, valued map[string]bool) (map[string]string, []string, bool) {
	f, pos := map[string]string{}, []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if len(a) > 2 && a[:2] == "--" {
			if _, dup := f[a]; dup {
				return nil, nil, false
			}
			if valued[a] {
				if i+1 >= len(args) {
					return nil, nil, false
				}
				i++
				f[a] = args[i]
			} else if a == "--approve" || a == "--json" {
				f[a] = "1"
			} else {
				return nil, nil, false
			}
		} else {
			pos = append(pos, a)
		}
	}
	return f, pos, true
}

func emit(out io.Writer, asJSON bool, v any, text string) int {
	if asJSON {
		if json.NewEncoder(out).Encode(v) != nil {
			return 1
		}
		return 0
	}
	fmt.Fprintln(out, text)
	return 0
}

func runProject(args []string, out, errOut io.Writer) int {
	f, pos, ok := flags(args, map[string]bool{"--dir": true, "--project": true})
	if !ok || len(pos) < 1 {
		return invalid(errOut)
	}
	switch pos[0] {
	case "use":
		if len(pos) != 2 {
			return invalid(errOut)
		}
		p, err := UseProject(pos[1], f["--dir"], f["--approve"] == "1")
		if err != nil {
			fmt.Fprintln(errOut, "Project not set:", err)
			return 1
		}
		fmt.Fprintf(out, "Project %q targeted at %s (sandbox: local). Profile written to .evolution/project.json only.\n", p.Name, p.Root)
		return 0
	case "show":
		p, err := LoadProject(f["--project"])
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		return emit(out, f["--json"] == "1", p, fmt.Sprintf("Project %s · root %s · sandbox %s", p.Name, p.Root, p.Sandbox))
	}
	return invalid(errOut)
}

func runChange(args []string, out, errOut io.Writer) int {
	f, pos, ok := flags(args, map[string]bool{"--project": true})
	if !ok || len(pos) < 1 {
		return invalid(errOut)
	}
	p, err := LoadProject(f["--project"])
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	switch pos[0] {
	case "new":
		if len(pos) != 2 {
			return invalid(errOut)
		}
		rec, err := ChangeNew(p, pos[1], f["--approve"] == "1")
		if err != nil {
			fmt.Fprintln(errOut, "Change not recorded:", err)
			return 1
		}
		text := fmt.Sprintf("Change proposal %s recorded (%d files fingerprinted, %d oversized skipped).\nReview: evolution change diff %s --project <dir>\nTest:   evolution sandbox run <task> --project <dir> --change %s --approve\n%s", rec.ID, len(rec.Files), rec.Skipped, rec.ID, rec.ID, rec.Note)
		return emit(out, f["--json"] == "1", rec, text)
	case "list":
		if len(pos) != 1 {
			return invalid(errOut)
		}
		recs, err := ListChanges(p)
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		text := "No change proposals recorded."
		if len(recs) > 0 {
			text = ""
			for _, r := range recs {
				text += fmt.Sprintf("%s · %s · %d files\n", r.ID, r.Summary, len(r.Files))
			}
			text += "File-level manifests for human review; not agent-authored changes."
		}
		return emit(out, f["--json"] == "1", recs, text)
	case "diff":
		if len(pos) != 2 {
			return invalid(errOut)
		}
		diff, err := DiffChange(p, pos[1])
		if err != nil {
			fmt.Fprintln(errOut, "Diff blocked:", err)
			return 1
		}
		text := fmt.Sprintf("CHANGE %s · %s\nadded %d · modified %d · removed %d · unchanged %d\n", diff.ChangeID, diff.Summary, diff.Added, diff.Modified, diff.Removed, diff.Unchanged)
		for _, e := range diff.Entries {
			text += fmt.Sprintf("  %-8s %s (%d -> %d bytes)\n", e.Status, e.Path, e.OldSize, e.NewSize)
		}
		text += diff.Note
		return emit(out, f["--json"] == "1", diff, text)
	}
	return invalid(errOut)
}

func runSandboxCmd(args []string, out, errOut io.Writer) int {
	f, pos, ok := flags(args, map[string]bool{"--project": true, "--timeout": true, "--change": true})
	if !ok || len(pos) != 2 || pos[0] != "run" {
		return invalid(errOut)
	}
	timeout := 300
	if v, has := f["--timeout"]; has {
		n, err := strconv.Atoi(v)
		if err != nil || n < 5 || n > 1800 {
			return invalid(errOut)
		}
		timeout = n
	}
	p, err := LoadProject(f["--project"])
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	var rec RunRecord
	if changeID := f["--change"]; changeID != "" {
		rec, err = RunSandboxWithChange(p, pos[1], changeID, f["--approve"] == "1", time.Duration(timeout)*time.Second, out)
	} else {
		rec, err = RunSandbox(p, pos[1], f["--approve"] == "1", time.Duration(timeout)*time.Second, out)
	}
	if err != nil {
		fmt.Fprintln(errOut, "Sandbox run blocked:", err)
		return 1
	}
	fmt.Fprintf(out, "\n[EVOLUTION] %s %s in %.1fs (exit %d) · log %s\n", rec.Task, rec.Result, rec.Seconds, rec.ExitCode, rec.LogFile)
	if rec.Change != "" {
		fmt.Fprintf(out, "[EVOLUTION] bound to reviewed change manifest %s\n", rec.Change)
	}
	if rec.Result != "PASSED" {
		return 3
	}
	return 0
}

func runRoute(args []string, out, errOut io.Writer) int {
	f, pos, ok := flags(args, map[string]bool{"--project": true})
	if !ok || len(pos) != 1 {
		return invalid(errOut)
	}
	p, err := LoadProject(f["--project"])
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	r, err := RouteGoal(p, pos[0])
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	return emit(out, f["--json"] == "1", r, fmt.Sprintf("OMNI ROUTE · project %s\nGoal: %s\nRoles: %v\nSandbox: %s\nSuggested tasks: %v\nModel: %s\nNext: %s", r.Project, r.Goal, r.Roles, r.Sandbox, r.Tasks, r.Model, r.Approval))
}

func runRelease(args []string, out, errOut io.Writer) int {
	f, pos, ok := flags(args, map[string]bool{"--dir": true})
	if !ok || len(pos) != 1 || (pos[0] != "verify" && pos[0] != "manifest" && pos[0] != "matrix") {
		return invalid(errOut)
	}
	if pos[0] == "matrix" {
		entries := SupportMatrix()
		text := "TARGET SUPPORT MATRIX — honest verification state\n"
		for _, e := range entries {
			text += fmt.Sprintf("%s\n  build:     %s\n  native:    %s\n  signing:   %s\n  install:   %s\n  update:    %s\n  rollback:  %s\n  uninstall: %s\n", e.Target, e.SourceBuild, e.NativeExecution, e.Signing, e.Install, e.Update, e.Rollback, e.Uninstall)
		}
		text += "No signed installer, update service or native verification exists yet."
		return emit(out, f["--json"] == "1", entries, text)
	}
	if pos[0] == "manifest" {
		entries, err := ReleaseManifest(f["--dir"])
		if err != nil {
			fmt.Fprintln(errOut, "Manifest blocked:", err)
			return 1
		}
		bad := false
		for _, e := range entries {
			bad = bad || e.Fingerprint != "MATCHES"
			if f["--json"] != "1" {
				fmt.Fprintf(out, "%s/%s\n  artifact: %s\n  sha256:   %s (%s)\n  signing:  %s\n", e.Platform, e.Arch, e.Artifact, e.SHA256, e.Fingerprint, e.SigningStatus)
			}
		}
		if f["--json"] == "1" {
			if code := emit(out, true, entries, ""); code != 0 {
				return code
			}
		} else {
			fmt.Fprintln(out, "No signature is verified by this command; run platform verifiers in CI.")
		}
		if bad {
			return 3
		}
		return 0
	}
	r, err := VerifyRelease(f["--dir"])
	if err != nil {
		fmt.Fprintln(errOut, "Verify blocked:", err)
		return 1
	}
	code := emit(out, f["--json"] == "1", r, fmt.Sprintf("Files: %d · hash errors: %v\nSignatures: %s\nVerdict: %s", r.Files, r.HashErrors, r.Signed, r.Verdict))
	if len(r.HashErrors) > 0 || r.Verdict == "REJECT: do not distribute" {
		return 3
	}
	return code
}
