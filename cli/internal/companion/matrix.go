package companion

// SupportMatrixEntry states, honestly, what is verified for each declared
// target. Nothing here may claim a signed installer, native verification or
// update/rollback support that has not actually been performed.
type SupportMatrixEntry struct {
	Target          string `json:"target"`
	SourceBuild     string `json:"source_build"`
	NativeExecution string `json:"native_execution"`
	Signing         string `json:"signing"`
	Install         string `json:"install"`
	Update          string `json:"update"`
	Rollback        string `json:"rollback"`
	Uninstall       string `json:"uninstall"`
}

const (
	matrixUnsigned  = "UNSIGNED — no platform signature verifier has run"
	matrixNoInstall = "NOT AVAILABLE — no signed installer or package exists"
)

// SupportMatrix is the declared target support matrix. It is static and
// conservative on purpose: only work actually performed may be listed as
// verified, and native execution is claimed only where tests really ran.
func SupportMatrix() []SupportMatrixEntry {
	targets := []struct{ target, native string }{
		{"linux/amd64", "Source build and Go test suite executed on a Linux host"},
		{"linux/arm64", "Cross-compiled only; native execution NOT VERIFIED"},
		{"darwin/amd64", "Cross-compiled only; native execution NOT VERIFIED"},
		{"darwin/arm64", "Cross-compiled only; native execution NOT VERIFIED"},
		{"windows/amd64", "Cross-compiled only; native execution NOT VERIFIED"},
		{"windows/arm64", "Cross-compiled only; native execution NOT VERIFIED"},
	}
	out := make([]SupportMatrixEntry, 0, len(targets))
	for _, t := range targets {
		out = append(out, SupportMatrixEntry{
			Target:          t.target,
			SourceBuild:     "Unsigned source cross-compile only; not a released package",
			NativeExecution: t.native,
			Signing:         matrixUnsigned,
			Install:         matrixNoInstall,
			Update:          matrixNoInstall,
			Rollback:        matrixNoInstall,
			Uninstall:       "Manual source-tree removal only; no installer to uninstall",
		})
	}
	return out
}
