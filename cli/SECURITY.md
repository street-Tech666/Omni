# Security and release architecture — not certification

## Current trust boundary

Core CLI contains no network client, API credential intake, telemetry,
auto-updates, background agent or elevated permissions. Owner-approved sandbox
runs execute fixed Go/npm argv in the selected local project, never a shell.
Project code/package scripts are executable and may read user-accessible files
or use the network: this is NOT container/VM isolation. The environment allowlist
does not sandbox the filesystem. Timeout bounds the direct process and pipe wait,
not descendant termination on every platform. Unknown operations fail closed.
PATH doctor checks names without executing them.
Opt-in `discover --approve` broadens the fixed name catalog only: PATH metadata,
no content/version/configuration access, no subprocesses or network client, no paths
in output. Discovery skips relative paths, UNC paths and symlink directories/files;
tools reached only through these or outside PATH are intentionally missed. The
operator must provide a local PATH; OS-mounted remote filesystems cannot be certified
local by this source prototype. Name matches are not verified compatibility.
A local filesystem receipt is not a signature and cannot defeat an attacker
with write access to the folder.
Sandbox output is capped and synchronized, then best-effort credential-redacted
before display or saving (not streamed live). Structured tokens, auth/cookie
headers and incomplete private-key blocks are covered; unknown secrets may still
escape. Do not execute untrusted projects or rely on redaction as credential custody.
Use a private dedicated directory, never a shared/writable system binary path.
Run logs use a pre-opened Go directory handle and exclusive writes; a task replacing
the run directory is rejected. This bounds log persistence, not project execution
or bind mounts. Native ACLs, other install paths and interrupted uninstall recovery
require additional review before distribution.

## Future identity / secret storage (disabled)

- Owner-approved OAuth via browser, PKCE/state, exact verified callback/origin
  and short-lived caller identity; secure session refresh/revocation.
- Use macOS Keychain, Windows Credential Manager, Linux Secret Service through
  separately audited adapters; fail closed if secure storage is unavailable.
  No plaintext fallback, command-line keys, token logging or `.env` copying.
- Backend checks real caller/project ownership and scopes on every action.
  Local preferences and unsigned records are not authenticated authorization.
- Explicit source/context selection, budget disclosure and human gates. OMNI
  is an approved model/API coordinator, not a proprietary foundation model.
  Preserve status/denial/cancellation behavior; never fabricate model replies.
- Official opt-in adapters only; each tool retains its own authorization. No
  existing toolchain config, shell startup file, alias or PATH modification.

## Future signed packages (not available)

1. Confirm owner-selected source repo and license; protect branch/release
   environments and review full commit-pinned CI actions, dependencies, SBOM,
   vulnerabilities and source provenance. Offline source archives are NOT
   distribution approval. Do not upload any artifact until owner authorizes.
2. Build/test each native OS/architecture in clean hosts and popular terminals.
   Cover help, offline failures, command/alias collisions, permissions, consent,
   running-file locks, interruptions, install/uninstall, config preservation,
   path/symlink/race abuse, update rollback and negative account/scope tests.
3. macOS: Developer ID executable/package signing, hardened-runtime review,
   notarization and stapling as appropriate; verify `codesign`/Gatekeeper on
   fresh hosts. Windows: Authenticode via approved signing custody and verify
   trust/installer ACLs on fresh hosts. Linux: signed checksums/artifact manifest
   using pinned publisher verification keys and tested distribution format.
4. Record exact source revision, target, toolchain, SBOM, digest, signature,
   signer, expiry/revocation and independently generated provenance. A checksum
   by itself never means publisher trust. Do not use self-created sample keys
   as a production signature or pretend signing credentials are available.
5. Pin signer/trust roots before first install; refuse tampering, invalid or
   expired signatures and unsupported targets. Future self-updates need an
   explicit verified manifest, proposed changes, approval, version pin and
   tested adapter-specific recovery. No automatic modifications to other tools.
6. Owner approves exact repository, packages, channel and release. Register
   verified Homebrew/WinGet identities or an optional tested npm wrapper only
   after signing and native evidence. No fake registry names, download URLs,
   curl-pipe-shell installer or public release now.

Auth, OS key stores, CI distribution, native signed release verification and
OMNI/model connectivity are **not implemented** by this source scaffold.