# Code signing and release verification — SCAFFOLD, NOT ACTIVE

Nothing in this repository is signed. `evolution release verify` checks SHA-256
hashes for real and reports `UNSIGNED` unless detached `.sig` files exist; even then
it says "NOT VERIFIED HERE" until the platform verifiers below pass in CI.

## Actual platform targets (from scripts/build_source.py)
darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64, windows/arm64.

## Prerequisites the owner must decide/provide (none are purchased or configured)
| Target  | Signing method | Owner account needed | Secrets (Project/CI secret store only) |
|---------|----------------|----------------------|----------------------------------------|
| macOS   | Developer ID Application + notarization (`codesign`, `xcrun notarytool`) | Apple Developer Program membership (paid) | DEVELOPER_ID_P12_BASE64, P12_PASSWORD, APPLE_ID / TEAM_ID / app-specific password or App Store Connect API key |
| Windows | Authenticode (`signtool`) via OV/EV cert or Azure Trusted Signing | Certificate authority or Azure subscription (paid) | Cert/HSM access or Azure client credentials |
| Linux + all archives | Sigstore `cosign sign-blob` keyless (GitHub OIDC) — free | Owner-selected GitHub repository with Actions | none (OIDC); optional minisign key |

## Release gate (all must pass, then owner approves; no auto-publish)
1. `go test`, `go vet`, reproducible `build_source.py` on 3 native runners.
2. Sign → verify with `cosign verify-blob`, `codesign --verify --strict` + `spctl -a`, `signtool verify /pa`.
3. `evolution release verify --dir <staging>` → no hash errors.
4. Owner reviews `source-manifest.json` and approves upload manually.

See `ci/release-signing.yml.example` (dormant; outside .github/workflows).

Verification identity must match the exact owner-approved repository, release
workflow and tag, not any workflow within a repository. Placeholder action pins
and identity must be resolved/reviewed before activation. Hash-only CLI results
never replace native platform verification. Current tests establish source
integrity/error handling, not notarization, Authenticode or Sigstore trust.
