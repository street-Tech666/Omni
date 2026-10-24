package companion

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseManifest(t *testing.T) {
	dir := t.TempDir()
	data := []byte("artifact")
	sum := sha256.Sum256(data)
	h := hex.EncodeToString(sum[:])
	os.WriteFile(filepath.Join(dir, "a.zip"), data, 0600)
	os.WriteFile(filepath.Join(dir, "b.zip"), []byte("tampered"), 0600)
	os.WriteFile(filepath.Join(dir, "a.zip.sig"), []byte("sig"), 0600)
	os.WriteFile(filepath.Join(dir, "source-manifest.json"), []byte(`{"artifacts":[{"target":"linux/amd64","file":"a.zip","sha256":"`+h+`"},{"target":"darwin/arm64","file":"b.zip","sha256":"`+h+`"}]}`), 0600)
	e, err := ReleaseManifest(dir)
	if err != nil || len(e) != 2 {
		t.Fatal(err)
	}
	if e[0].Platform != "linux" || e[0].Arch != "amd64" || e[0].Fingerprint != "MATCHES" || !strings.Contains(e[0].SigningStatus, "NOT VERIFIED") {
		t.Fatal(e[0])
	}
	if !strings.HasPrefix(e[1].Fingerprint, "MISMATCH") || e[1].SigningStatus != "UNSIGNED" {
		t.Fatal(e[1])
	}
	var out, errb bytes.Buffer
	if Run([]string{"release", "manifest", "--dir", dir}, "evolution", &out, &errb) != 3 {
		t.Fatal("mismatch must fail")
	}
	if _, err := ReleaseManifest("relative"); err == nil {
		t.Fatal("relative dir accepted")
	}
}

func TestManifestJSONExitParity(t *testing.T) {
	for _, state := range []string{"matching", "tampered", "missing"} {
		t.Run(state, func(t *testing.T) {
			dir := t.TempDir()
			sum := sha256.Sum256([]byte("original"))
			h := hex.EncodeToString(sum[:])
			if state != "missing" {
				data := []byte("original")
				if state == "tampered" {
					data = []byte("changed")
				}
				if err := os.WriteFile(filepath.Join(dir, "a.zip"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "source-manifest.json"), []byte(`{"artifacts":[{"target":"linux/amd64","file":"a.zip","sha256":"`+h+`"}]}`), 0600); err != nil {
				t.Fatal(err)
			}
			want := 3
			if state == "matching" {
				want = 0
			}
			for _, asJSON := range []bool{false, true} {
				args := []string{"release", "manifest", "--dir", dir}
				if asJSON {
					args = append(args, "--json")
				}
				var out, errOut bytes.Buffer
				if got := Run(args, "evolution", &out, &errOut); got != want {
					t.Fatalf("json=%v got %d want %d", asJSON, got, want)
				}
				if asJSON {
					var entries []ManifestEntry
					if err := json.Unmarshal(out.Bytes(), &entries); err != nil || len(entries) != 1 {
						t.Fatalf("invalid JSON: %v", err)
					}
				}
			}
		})
	}
}
