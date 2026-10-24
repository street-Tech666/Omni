package companion

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ManifestEntry describes one built target. SigningStatus never claims
// "VERIFIED": this tool does not run platform verifiers.
type ManifestEntry struct {
	Platform      string `json:"platform"`
	Arch          string `json:"arch"`
	Artifact      string `json:"artifact_path"`
	SHA256        string `json:"sha256"`
	Fingerprint   string `json:"fingerprint_check"`
	SigningStatus string `json:"signing_status"`
}

func ReleaseManifest(dir string) ([]ManifestEntry, error) {
	if !filepath.IsAbs(dir) {
		return nil, errors.New("--dir must be an absolute path")
	}
	data, err := os.ReadFile(filepath.Join(dir, "source-manifest.json"))
	if err != nil {
		return nil, errors.New("source-manifest.json not found")
	}
	var m struct {
		Artifacts []struct {
			Target string `json:"target"`
			File   string `json:"file"`
			SHA256 string `json:"sha256"`
		} `json:"artifacts"`
	}
	if json.Unmarshal(data, &m) != nil || len(m.Artifacts) == 0 {
		return nil, errors.New("source-manifest.json is malformed or lists no artifacts")
	}
	out := []ManifestEntry{}
	seenFiles, seenTargets := map[string]bool{}, map[string]bool{}
	for _, a := range m.Artifacts {
		parts := strings.SplitN(a.Target, "/", 2)
		if len(parts) != 2 || (parts[0] != "linux" && parts[0] != "darwin" && parts[0] != "windows") || (parts[1] != "amd64" && parts[1] != "arm64") || !validDigest(a.SHA256) || a.File == "" || a.File == "." || strings.ContainsAny(a.File, `/\`) || a.File == ".." || seenFiles[a.File] || seenTargets[a.Target] {
			return nil, errors.New("manifest contains an invalid artifact entry")
		}
		seenFiles[a.File], seenTargets[a.Target] = true, true
		p := filepath.Join(dir, a.File)
		e := ManifestEntry{Platform: parts[0], Arch: parts[1], Artifact: p, SHA256: a.SHA256}
		checked, pathErr := releaseArtifact(dir, a.File)
		got, err := "", pathErr
		if pathErr == nil {
			got, err = sha256File(checked)
		}
		switch {
		case err != nil:
			e.Fingerprint = "MISSING FILE"
		case got != a.SHA256:
			e.Fingerprint = "MISMATCH (actual " + got + ")"
		default:
			e.Fingerprint = "MATCHES"
		}
		if _, err := os.Stat(p + ".sig"); err == nil {
			e.SigningStatus = "SIGNATURE FILE PRESENT — NOT VERIFIED HERE"
		} else {
			e.SigningStatus = "UNSIGNED"
		}
		out = append(out, e)
	}
	return out, nil
}
