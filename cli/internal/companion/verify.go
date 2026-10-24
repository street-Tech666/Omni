package companion

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func validDigest(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == sha256.Size
}

// Do not follow artifact symlinks outside the explicitly selected staging root.
func releaseArtifact(dir, file string) (string, error) {
	if file == "" || file == "." || file == ".." || strings.ContainsAny(file, `/\`) {
		return "", errors.New("invalid artifact filename")
	}
	p := filepath.Join(dir, file)
	info, err := os.Lstat(p)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("artifact must be a regular file, not a symlink or directory")
	}
	return p, nil
}

// ReleaseReport is the result of checking a staged release directory.
// Hash checks are real. Signature status is only "VERIFIED" when a platform
// verifier has been run and recorded by CI; this code never asserts it.
type ReleaseReport struct {
	Files      int      `json:"files_checked"`
	HashErrors []string `json:"hash_errors"`
	Signed     string   `json:"signature_status"`
	Missing    []string `json:"missing_signature_files"`
	Verdict    string   `json:"verdict"`
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func VerifyRelease(dir string) (ReleaseReport, error) {
	r := ReleaseReport{HashErrors: []string{}, Missing: []string{}}
	if !filepath.IsAbs(dir) {
		return r, errors.New("--dir must be an absolute path")
	}
	sums, err := os.Open(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		return r, errors.New("SHA256SUMS not found")
	}
	defer sums.Close()
	checksums := map[string]string{}
	scan := bufio.NewScanner(sums)
	for scan.Scan() {
		parts := strings.Fields(scan.Text())
		if len(parts) != 2 || !validDigest(parts[0]) || strings.ContainsAny(parts[1], `/\`) || parts[1] == ".." || parts[1] == "." {
			r.HashErrors = append(r.HashErrors, "malformed line")
			continue
		}
		if _, exists := checksums[parts[1]]; exists {
			r.HashErrors = append(r.HashErrors, parts[1]+": duplicate checksum entry")
			continue
		}
		checksums[parts[1]] = parts[0]
		r.Files++
		p, pathErr := releaseArtifact(dir, parts[1])
		got, err := "", pathErr
		if pathErr == nil {
			got, err = sha256File(p)
		}
		if err != nil {
			r.HashErrors = append(r.HashErrors, parts[1]+": missing")
		} else if got != parts[0] {
			r.HashErrors = append(r.HashErrors, parts[1]+": checksum mismatch")
		}
		// Expected detached signature produced by the release pipeline (cosign/minisign).
		if _, err := os.Stat(filepath.Join(dir, parts[1]+".sig")); err != nil {
			r.Missing = append(r.Missing, parts[1]+".sig")
		}
	}
	if scan.Err() != nil {
		r.HashErrors = append(r.HashErrors, "could not read complete SHA256SUMS")
	}
	if r.Files == 0 {
		r.HashErrors = append(r.HashErrors, "no artifacts listed")
	}
	var manifest struct {
		Signed bool `json:"signed"`
	}
	if data, err := os.ReadFile(filepath.Join(dir, "source-manifest.json")); err == nil {
		if json.Unmarshal(data, &manifest) != nil {
			r.HashErrors = append(r.HashErrors, "malformed source manifest")
		} else {
			entries, err := ReleaseManifest(dir)
			if err != nil {
				r.HashErrors = append(r.HashErrors, "invalid source manifest")
			} else {
				if len(entries) != len(checksums) {
					r.HashErrors = append(r.HashErrors, "manifest/checksum coverage mismatch")
				}
				for _, entry := range entries {
					if checksums[filepath.Base(entry.Artifact)] != entry.SHA256 {
						r.HashErrors = append(r.HashErrors, "manifest/checksum digest mismatch")
					}
				}
			}
		}
	} else if !os.IsNotExist(err) {
		r.HashErrors = append(r.HashErrors, "source manifest unreadable")
	}
	switch {
	case len(r.Missing) == 0 && r.Files > 0:
		r.Signed = "SIGNATURE FILES PRESENT — NOT VERIFIED HERE; run platform verifiers (cosign verify-blob, codesign/spctl, signtool verify)"
	case manifest.Signed:
		r.Signed = "INVALID CLAIM: manifest says signed but signature files are missing"
	default:
		r.Signed = "UNSIGNED"
	}
	if len(r.HashErrors) > 0 || strings.HasPrefix(r.Signed, "INVALID") {
		r.Verdict = "REJECT: do not distribute"
	} else {
		r.Verdict = fmt.Sprintf("HASHES OK for %d files; not a trusted release until signatures are verified", r.Files)
	}
	return r, nil
}
