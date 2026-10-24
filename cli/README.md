# EVOLUTION CLI — unreleased offline source

Lightweight, optional companion for **existing** macOS/Linux/Windows terminals.
Not a terminal emulator, daemon, installer service or replacement for Codex,
Claude, GPT, Unity, Blender or other tools. Browser preview requires no download.

## Compatibility-first product policy

Basic access is not contingent on a recurring Evolution subscription, hosted
compute or a replacement IDE. Keep existing investments and authorized tools.
This offline source performs no paid API calls and requires no account; that is
not a promise of free AI. Open-source local utilities are intended where
financially feasible, but the owner's source/distribution license is still
unapproved. Do not infer an open-source license from source visibility. Project
file diagnostics are not implemented. Opt-in PATH-name developer-tool discovery is implemented in offline source; browser
capability discovery is an inert reference catalog, not installed-tool probing.

Future adapters require opt-in official APIs, supported provider capabilities,
secure settings/OS key custody, explicit context and actual usage-cost reporting.
Consumer subscriptions do not confer API entitlement; third-party fees and terms
remain applicable. Hosted compute/team offerings may be optional paid services;
no rate, checkout, billing or guaranteed free inference is available now.

Preserve open JSON exports, original artifacts and reversible owned-copy removal.
No mandatory tool bundles, provider/toolchain reconfiguration or duplicated app
installation. Prefer existing authorized installations and reusable artifacts;
no carbon-neutral assertion or invented energy/time/cost savings. Show measured
source checks separately from user-project builds and self-declared comparisons.
Cyberace remains optional/unconfirmed DEMO; CyberLink Social is entirely outside
this project's enrollment, access, modification and deployment scope.

## Implemented here

Go standard library only; no third-party runtime packages. `help`, `version`
(`--version`), and `doctor [--json]` run offline. Doctor reads platform and PATH
entries for only `evolution` and `omni`; it never executes candidates, scans
project files, reads keys or verifies device safety. Windows name checks cover
case-insensitive defaults, PowerShell `.ps1` and valid effective `PATHEXT`. Shell aliases/functions
are not visible to it: inspect them with your shell's `type -a` or PowerShell
`Get-Command -All` before any future installation.

Explicit source `install` and `uninstall` are restricted to a **new, absolute,
dedicated directory** whose parent exists. No PATH or startup-file changes.
Existing destinations, PATH command collisions and symlink parents are refused.
Uninstall validates the complete local receipt and every binary before removal;
extra files, symlinks, changed binaries or bad receipts prevent removal. It only
removes exact owned file names; it never recursively deletes user directories.
Local receipt hashes are editable integrity checks, **not publisher signatures
or authenticated ownership**. An interrupted operation can require manual
inspection; never promise atomic rollback or protection against a hostile local
process racing filesystem changes. Windows executing-file removal behavior and
native ACLs need Windows-host tests; source modes are not Windows ACL guarantees.

## Developer-only local execution

Prerequisite: reviewed Go 1.25+ toolchain and this source checkout. These are
source-development commands, **not package-manager or released installation
instructions**. Run from `cli/`:

```text
go test ./...
go vet ./...
go build -trimpath -o <absolute-staging-path>/evolution ./cmd/evolution
<absolute-staging-path>/evolution help
<absolute-staging-path>/evolution version
<absolute-staging-path>/evolution doctor --json
```

Windows output is `evolution.exe`. Use a pre-existing staging parent. The optional
`omni`/`omni.exe` copy dispatches to the same currently **blocked** OMNI entry;
`evolution omni` remains the fallback. It does not start a fake AI session.

```text
<source-binary> install --dir <new-absolute-owned-directory> --approve
<source-binary> install --dir <different-new-owned-directory> --approve --with-omni
<source-binary> uninstall --dir <same-owned-directory> --approve
```

Prefer uninstall from the separate source binary, not a running installed copy
(Windows locks running executables). No command has been installed on the user's
machine by Lovable. Test changes are confined to sandbox temporary directories.

## Planned, intentionally blocked

`login`, `connect`, `nexus`, `agents`, `update`, `omni`, `evolution omni`, and
`omni <natural-language request>` return exit **2** without network access,
credential intake, token echo, file writes or success claims. Unknown arguments
also return 2. Errors never repeat supplied credentials. Help/version/doctor
return 0; failed local install/uninstall returns 1.

No OAuth, API model, arbitrary shell execution, download, updater, repository connection,
agent enumeration, media processing, provider billing or automatic integration
is implemented. Do not pass keys via command arguments. The implementation has
no network client or credential store, and does not consume API-key env vars.

### Project targeting and local test records

`project use <name> --dir <absolute-project-dir> --approve` writes only the
selected project's `.evolution/project.json`. `project show --project <dir>`
reads it; `omni route "<goal>" --project <dir>` returns a plan with no model call.
`sandbox run <go-test|go-vet|go-build|npm-test> --project <dir> --approve`
runs one fixed local argv. It is not isolated from local files or network.
Stdout/stderr are synchronized, capped at 1 MiB, redacted before display and
saved with actual exit status in `.evolution/runs`; output appears after execution,
not live. Secrets outside common patterns can still escape. Timeout does not
guarantee termination of every descendant process on every OS.

`release manifest --dir <stage> [--json]` lists all six supported target shapes,
artifact paths, digests and unverified signature-file presence. Missing/mismatched
files exit 3 in both text and JSON. `release verify` rejects incomplete checksum
reads, duplicates, symlink artifacts and manifest/checksum coverage conflicts.
Checksum-only directories remain supported but establish no publisher trust.

## Opt-in private tool discovery

Source-only command: `evolution discover --approve [--json]`. Without explicit
approval it exits 2 and does not scan. It reads configured PATH directory/name
metadata for Git, Node.js, Python 3, Go, Docker, VS Code, Unity, Blender and Solana
CLI. No tool processes, file contents, versions, profile/config files, device
identifiers, accounts or credentials are read. No network/telemetry/upload.
Only name-match counts are output; absolute paths are not printed or saved.
JSON output is optional local stdout, not an automatic file write. Tools outside
PATH and aliases can be missed. File-name presence is not authenticity, version
support or verified compatibility. Suggestions are disabled/planned references,
not installed or connected extensions. No signed distribution is available.

## Builds and unsigned archive staging

Python 3 stdlib build helper invokes a separately installed Go compiler, uses
`CGO_ENABLED=0` and produces source binaries for Darwin/Linux/Windows,
amd64/arm64. It fails if output exists, does not install anything and does not
call GitHub, package managers or signing services:

```text
python3 scripts/build_source.py --out <new-absolute-staging-directory>
```

The helper produces **UNSIGNED_SOURCE_ONLY** ZIPs, exact SHA-256 manifest and
checksums, not trusted release packages. Each ZIP contains only `evolution`,
README and a no-release warning. Optional alias installation requires separate
approval; `omni` is not silently bundled onto PATH. A failed build leaves an
`INCOMPLETE.txt` marker and reports partial staging; it never recursively removes
that directory or presents partial output as a complete archive set.

## Repository and distribution gates

No Evolution repository, URL, package identity, signed download or release feed
is confirmed. The owner must select/create the Git repository through Lovable
**Project settings → Git** first. Do not reuse or overwrite unrelated repos.
Lovable Git sync creates a new private repository; API repository access is a
separate connection. This scaffold uses `evolution.local/companion` as a local Go
module identity, not a GitHub URL or npm package.

`ci/native-source-tests.yml.example` is dormant: it is not in `.github/workflows`.
After repository confirmation and review, pin actions to audited full commit
digests, test source on all native hosts and verify shell install/uninstall.
Only then design protected-environment release jobs. No upload/publish job or
package-manager manifest is activated here.

## Verification in the development sandbox

- 666 existing/new web tests pass; final focused route/reference checks pass.
- Go source tests and `go vet` pass on Linux/amd64; no third-party Go modules.
- Three Python staging tests cover existing-directory preservation, failure
  markers, exact six-target archives, unsigned status and digest validation.
- Six source binaries compile for macOS/Linux/Windows × amd64/arm64. ZIP entries
  and binary/archive SHA-256 values were independently inspected.
- Real Linux/amd64 executable tested: help, version, JSON doctor, blocked future
  integrations; install requires approval, optional alias works, extra/changed
  files refuse uninstall, exact unchanged copies uninstall, evolution conflicts
  block install and omni conflicts retain `evolution omni` without overwriting.
  These run only in temporary sandbox directories, never the user's device.
- Browser checks cover OS choices, declared conflicts, copied references, command
  search/empty result, fallback details, architecture, browser OMNI and mobile
  navigation/reduced motion. No browser errors or provider calls observed.
- Production npm dependency scan reports no known advisories; native CLI uses
  only the Go standard library. This is not a complete security certification.
- Native macOS/Windows/arm64 execution, popular-shell/ACL/race/lock behavior,
  credential stores, authentic OMNI/auth/RLS, signing, update recovery and CI
  distribution are **not verified or enabled**. Repository and owner release
  approval remain required. Nothing published or connected.

See `SECURITY.md` for signing, identity, model consent and native-release gates.