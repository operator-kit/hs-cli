# PII Redaction — High-Severity Implementation Handoff

Last updated: 2026-07-15

Status: design agreed; implementation not started

## Purpose

This is the implementation handoff for the four remaining high-severity
findings in the original PII review:

5. an empty `HS_INBOX_PII_SECRET` uses unkeyed SHA-256;
6. invalid PII modes silently normalize to `off`;
7. Windows can install a model bundle that its runtime cannot execute;
8. downloaded model hashes are not independent trust anchors.

Issue 9, transactional and bounded model installation, is included with issue
8. Provenance verification is incomplete if untrusted bytes can overwrite the
live cache before verification succeeds.

This document is intended to let another developer implement the work without
having to reconstruct the review. It records the desired behavior, clean
architecture boundaries, regression tests, sequencing, compatibility
requirements, and acceptance criteria.

## Baseline

The four critical findings were fixed in:

- `7eb27a9 test: reproduce critical PII redaction leaks`
- `5a7ea63 fix: enforce PII redaction boundaries`

The detailed review and status register are in:

- `pii-redaction-progress.md`
- `pii-redaction-follow-ups.md`

At this baseline, `go test ./... -count=1` passes. Preserve the critical
regressions while doing this work.

## Scope and delivery order

| Order | Original issue | Delivery outcome |
| --- | --- | --- |
| 1 | #6: invalid mode | Parse once into a strong mode type and fail before Inbox/MCP work begins. |
| 2 | #5: empty secret | Resolve a private key for every enabled engine; remove the unkeyed production fallback. |
| 3 | #7: Windows mismatch | Add an honest capability/status gate and refuse unsupported installation before network access. |
| 4 | #8 + #9: model trust/install | Verify an embedded trusted manifest and install into a bounded staging area before promotion. |
| 5 | Windows support follow-up | Advertise a Windows target only after a real runtime/model smoke test passes on that target. |

The capability gate resolves the high-severity Windows mismatch even if native
Windows inference is delivered later. Do not hold the truthful status/install
behavior behind the larger runtime port.

## Non-negotiable behavior

1. The same explicit, non-blank `HS_INBOX_PII_SECRET` and the same source
   identity must continue to produce the same display identity as before.
2. Do not change `personKey` precedence, fake-name selection, canonicalization,
   or HMAC input/domain strings as part of these fixes.
3. Users who previously relied on the empty-secret SHA-256 fallback will have
   a one-time pseudonym change when a private installation secret is created.
   This is expected and should be called out in release notes.
4. Once a generated installation secret exists, identities must remain stable
   across processes and restarts on that installation.
5. `off`, and an authorized effective `--unredacted` override, must not read or
   write the keyring and must not initialize the model runtime.
6. Invalid privacy configuration must never become `off` implicitly.
7. A model is usable only when the current platform is supported and the exact
   expected bundle has completed trusted validation.
8. When no usable NER model exists, retain the current fail-closed free-text
   behavior. Never fall back to emitting uninspected content.
9. Regression tests must use fake API clients, temporary directories, in-memory
   archives, and local `httptest` servers. They must not call Help Scout or a
   public model host.
10. Secrets, model contents, URLs containing sensitive values, and raw keyring
    errors must not be written to command output or diagnostic logs.

## Target architecture

The command layer should be the composition root. Core PII behavior should not
know about environment variables, keyrings, HTTP, Cobra, or filesystem cache
locations.

```text
Cobra / MCP entry point
    |
    +-- PII engine factory
    |     +-- strict mode parser              (core policy)
    |     +-- secret resolver                 (application service)
    |     |     +-- environment source        (adapter)
    |     |     +-- OS keyring store          (adapter)
    |     |     +-- cryptographic RNG         (adapter)
    |     +-- detector provider
    |           +-- model status service
    |           +-- NER runtime
    |
    +-- pii-model commands
          +-- platform capability
          +-- embedded trusted manifest
          +-- transactional bundle installer
          +-- runtime smoke validator
```

### Recommended package responsibilities

- `internal/pii`
  - `Mode`, `ParseMode`, effective-mode policy, `Secret`, and `Engine`.
  - Pure behavior only; no `os.Getenv`, keyring, HTTP, or cache discovery.
- `internal/pii/setup` (new, or an equivalently named application package)
  - Secret resolution and PII engine/session construction.
  - Interfaces are defined here because this layer consumes the adapters.
- `internal/pii/secretstore` (new)
  - OS-keyring adapter only.
- `internal/pii/ner`
  - Platform capability, trusted manifest, model status, installer, and runtime.
  - HTTP client, cache root, platform, limits, and runtime validator are
    injectable at the service boundary.
- `internal/cmd`
  - Parse command intent, wire concrete dependencies, order preflight before
    API initialization, and present actionable errors/status.

Exact folder names may follow an existing repository convention, but preserve
the dependency direction: adapters depend inward; the PII core does not depend
outward.

### Invocation lifecycle

For any command that can expose Inbox data:

1. Load the file and apply environment overrides.
2. Strictly parse the configured mode.
3. Apply the authorized `--unredacted` policy to obtain the effective mode.
4. If the effective mode is enabled, resolve a non-empty secret before creating
   an API client or making a Help Scout request.
5. Attach NER only when model status is `ready`.
6. Construct an engine whose invariants are already valid.
7. Close any detector/runtime owned by the invocation.
8. Continue to route every output through the mandatory presentation boundary.

The factory may return a small invocation-scoped session containing the engine
and a `Close` function. That gives the NER runtime explicit ownership without
making commands know its details. If this seam is introduced, it can also
resolve deferred observation A2; do not otherwise expand this work into a
general command-lifecycle rewrite.

## Work package 1 — Issue #6: invalid modes fail open

### Current failure

`internal/pii/policy.go` uses `NormalizeMode`. Any unknown value becomes
`off`. `config set --inbox-pii-mode` validates its flag, but manually edited
YAML and `HS_INBOX_PII_MODE` bypass that check. A typo can therefore disable
redaction without an error.

`internal/cmd/config.go` also discards every `config.Load` error during
`config set` and starts from an empty config. That can erase unrelated fields
when a damaged config is being repaired.

### Desired contract

Use a strong mode type and one strict parser:

```go
type Mode string

const (
    ModeOff       Mode = "off"
    ModeCustomers Mode = "customers"
    ModeAll       Mode = "all"
)

func ParseMode(raw string) (Mode, error)
```

Rules:

- empty input means the documented default, `off`;
- surrounding whitespace and case are normalized;
- every other value returns an error containing the invalid value and the
  accepted values;
- downstream policy receives `Mode`, not an arbitrary string;
- no downstream function performs a second permissive normalization.

### Regression tests first

Add pure policy tests:

- `TestParseMode_AcceptsDocumentedValues`
- `TestParseMode_EmptyDefaultsToOff`
- `TestParseMode_NormalizesCaseAndWhitespace`
- `TestParseMode_RejectsUnknownValues`
- `TestEffectiveMode_RejectsInvalidConfiguredMode`

Add configuration tests:

- invalid YAML-file mode is retained as raw input until effective
  configuration has been merged, then rejected;
- a valid environment mode overrides an invalid file mode;
- an invalid environment mode is rejected even when the file value is valid;
- `config set --inbox-pii-mode customers` repairs an invalid stored mode and
  preserves every unrelated file field;
- `config set` does not persist environment-only overrides into YAML;
- malformed YAML and real I/O errors are reported, not replaced with an empty
  config.

Add command-level regression coverage:

- `TestPIIRegression_High06_InvalidFileModeStopsInboxCommand`
- `TestPIIRegression_High06_InvalidEnvModeStopsInboxCommand`
- `TestPIIRegression_High06_InvalidModeStopsMCPStartup`
- assert the fake Inbox API received zero calls;
- assert no fixture payload was emitted;
- assert `config get`, `config path`, and a mode-repairing `config set` remain
  available;
- assert unrelated commands such as `version`, completion, and Docs commands
  are not unnecessarily blocked by an Inbox-only setting.

### Implementation notes

1. Replace `NormalizeMode` with `ParseMode`. Keep normalization private to the
   successful parse path.
2. Change `EffectiveMode`, `ShouldRedactType`, and the engine to consume
   `Mode`.
3. Split configuration loading into two explicit operations:
   - file-only loading for mutation/repair;
   - effective loading that applies environment overrides for runtime use.
4. `config set` must load file-only state, update requested fields, validate
   the resulting PII mode, and save. It must not silently swallow parse or I/O
   errors.
5. Merge file and environment values before strict mode parsing. This lets a
   valid environment override temporarily recover an invalid file.
6. Validate at the Inbox/MCP application boundary before API-client creation.
   Keep a defensive parse in the engine factory so a future caller cannot
   bypass the invariant.
7. Move the background update check after privacy preflight for affected
   commands. An invalid privacy configuration should fail before unrelated
   network work begins.
8. Save the canonical lowercase value when `config set` changes the mode.

### Acceptance criteria

- No unknown mode can construct an enabled or disabled engine.
- Invalid Inbox/MCP configuration returns a clear error before API activity or
  protected output.
- Config repair works without data loss.
- There is no permissive string-normalization path left in production code.

## Work package 2 — Issue #5: empty secret uses unkeyed SHA-256

### Current failure

`Engine.hashBytes` uses plain SHA-256 when `HS_INBOX_PII_SECRET` is empty. The
result is deterministic, but it is identical across installations and permits
offline guessing of low-entropy identifiers.

The explicit-secret HMAC path is good and must remain byte-for-byte compatible.

### Desired secret policy

When the effective PII mode is enabled:

1. An `HS_INBOX_PII_SECRET` containing at least one non-whitespace rune wins
   and is used exactly as supplied. Do not trim or rewrite a valid value,
   because leading/trailing bytes are part of the existing identity contract.
2. An unset, empty, or whitespace-only environment value is treated as absent;
   load a generated 32-byte secret from the OS keyring instead. This matches
   the current fallback threshold without preserving the unkeyed fallback.
3. If no key exists, generate 32 bytes with `crypto/rand`, store it under a
   versioned record name, read it back, and use the persisted value.
4. If the keyring is unavailable or persistence cannot be confirmed, return an
   actionable error instructing a headless user to set
   `HS_INBOX_PII_SECRET`. Do not fall back to unkeyed hashing or YAML storage.
5. When the effective mode is `off`, do not touch the keyring or RNG.

Recommended keyring record semantics:

- stable service name associated with this CLI;
- record/account name such as `pii_identity_secret_v1`;
- random bytes encoded with a fixed base64 encoding only for storage;
- strict decode and exact-length validation when loading;
- never include the value in output, logs, or wrapped errors.

The versioned record name reserves room for issue 13's future rotation policy
without implementing rotation in this change.

### Encapsulation

Define the interfaces at the application-service boundary, not in the engine:

```go
type SecretStore interface {
    Load(context.Context) ([]byte, error)
    Save(context.Context, []byte) error
}

type SecretResolver struct {
    Store     SecretStore
    Random    io.Reader
    LookupEnv func(string) (string, bool)
    // Include an initialization lock if the store has no create-if-absent API.
}
```

Return a copied, validated value or a small `pii.Secret` value whose bytes are
not publicly mutable. The engine should require a non-empty secret whenever
its mode is enabled. Its hash path should then always be HMAC-SHA-256; remove
the production empty-secret branch entirely.

The keyring adapter should distinguish "not found" from "store unavailable".
Only the former permits generation.

### First-use concurrency

Two first-run CLI/MCP processes must converge on one persisted key. A simple
`get -> generate -> set` sequence can let concurrent processes use different
keys. If the selected keyring API has no atomic create-if-absent operation,
serialize initialization with a small cross-process lock, then re-read inside
the lock. A process-local `sync.Once` is not sufficient.

Do not ship automatic generation until the concurrent initialization test is
stable on supported operating systems. Requiring the environment variable is
the safe fallback if robust keyring initialization cannot be delivered.

### Regression tests first

Use an in-memory fake store and injected deterministic random reader. Do not
touch a developer or CI keyring.

- `TestSecretResolver_EnvironmentOverridesStore`
- `TestSecretResolver_ExplicitSecretPreservesBytes`
- `TestSecretResolver_BlankEnvironmentUsesStore`
- `TestSecretResolver_GeneratesAndStoresSecretOnce`
- `TestSecretResolver_ReusesSecretAcrossEngineInstances`
- `TestSecretResolver_OffModeDoesNotTouchStoreOrRandom`
- `TestSecretResolver_NotFoundGeneratesButStoreErrorFails`
- `TestSecretResolver_InvalidStoredValueFailsClosed`
- `TestSecretResolver_ConcurrentInitializersConverge`
- `TestPIIRegression_High05_KeyringFailureEmitsNoOutput`
- assert the fake API receives zero calls when secret preflight fails;
- assert the error explains the environment-variable recovery path without
  exposing key material.

Before changing the constructor, add a golden compatibility fixture that pins
the current output for a known identity and explicit secret. Run the same
fixture against separate engine instances.

Existing unit tests commonly construct enabled engines with an empty secret.
Update them to use a fixed test-only secret helper. Do not retain a production
escape hatch merely to keep tests concise.

### Diagnostic sanitizer

`internal/api/debug_sanitizer.go` currently creates its strongest-mode engine
with an empty secret. It must not use the persisted identity key, because debug
tokens should not become a cross-log identity channel.

Give each diagnostic transport or process an independent cryptographically
random ephemeral secret. If random generation fails, disable/fail debug setup
safely rather than reverting to unkeyed hashing. Keep the sanitizer independent
of user-facing PII mode and `--unredacted`.

### Acceptance criteria

- An enabled production engine cannot exist with an empty key.
- Explicit-secret pseudonyms are unchanged from the baseline fixture.
- Generated-secret pseudonyms are stable across processes on one installation
  and differ across independently initialized stores.
- Keyring/RNG failures occur before protected API work and emit no payload.
- PII secret material is never written to YAML, stdout, stderr, or debug logs.

## Work package 3 — Issue #7: Windows installer/runtime mismatch

### Current failure

The bundle builder publishes Windows archives and cache discovery recognizes
`onnxruntime.dll`, but `runtime_unsupported.go` is selected on Windows. A
`.version` file is enough for `IsModelReady` to report success, so install and
status can imply NER is active when it cannot run.

The current no-detector text behavior is privacy-oriented, but the operator
feedback is untruthful and can create false confidence.

### Stage A: close the mismatch immediately

Introduce an explicit platform capability and a richer model status:

```go
type ModelState string

const (
    ModelUnsupported ModelState = "unsupported"
    ModelAbsent      ModelState = "absent"
    ModelUntrusted   ModelState = "installed-unverified"
    ModelCorrupt     ModelState = "corrupt"
    ModelReady       ModelState = "ready"
)

type ModelStatus struct {
    State  ModelState
    Reason string
}
```

Names may vary, but do not collapse these states into one boolean. Retain
`IsModelReady` only as a thin compatibility wrapper over
`Status().State == ModelReady`, or remove it after callers migrate.

Behavior on an unsupported target:

- `pii-model install` fails before printing download progress and before any
  HTTP request;
- `pii-model status` says the runtime is unsupported and explains that
  free-form content remains fail-closed;
- cache files or a matching `.version` cannot turn the state into `ready`;
- `newPIIEngine` does not attach a detector;
- structured redaction still works and free-form content remains hidden;
- release tooling does not publish or advertise an unsupported platform.

### Regression tests first

Make platform/capability and downloader dependencies injectable so ordinary
tests can simulate Windows while running elsewhere.

- `TestModelStatus_UnsupportedEvenWithVersionMarker`
- `TestInstaller_UnsupportedPlatformDoesNotIssueHTTPRequest`
- `TestModelStatus_DistinguishesAbsentUntrustedCorruptAndReady`
- `TestPIIModelCommand_StatusExplainsUnsupportedRuntime`
- `TestPIIRegression_High07_UnusableRuntimeKeepsFreeTextFailClosed`
- `TestReleaseManifest_ContainsOnlySupportedPlatforms`

The no-request assertion should use a counting fake transport or local server,
not a public endpoint.

### Stage B: add real Windows support separately

The patched `onnxruntime-purego` dependency currently calls Unix-oriented
`purego.Dlopen`. A Windows implementation needs platform-specific loader
files, using the appropriate Windows library load/free operations, plus proof
that function registration and the calling convention work with ONNX Runtime.

Recommended sequence:

1. Implement loader open/close behind build-tagged files in the patched
   dependency.
2. Add Windows to the real NER runtime build tags only after loader tests pass.
3. Start with `windows/amd64` unless `windows/arm64` has its own real CI runner
   or equivalent execution proof.
4. Load the pinned `onnxruntime.dll` and resolve `OrtGetApiBase` in a Windows CI
   smoke test.
5. Construct the tokenizer/model, run a tiny pinned inference fixture, verify
   output shape and at least one expected entity, and close all handles.
6. Exercise `hs pii-model install`, `status`, detector creation, and one
   redaction command end to end on Windows.
7. Only then add that exact target to the supported capability matrix and
   release artifacts.

Cross-compilation proves that code builds; it does not prove DLL loading, ABI,
transitive dependencies, or inference. Do not use it as the support gate.

### Acceptance criteria

- The CLI never reports `ready` on a target whose runtime cannot execute.
- Unsupported installation performs zero network or cache mutation.
- Privacy fallback remains fail-closed.
- Every advertised platform/architecture has an executing runtime/model smoke
  test in CI.

## Work package 4 — Issues #8 and #9: trusted, transactional model install

### Current failure

`downloadAndExtract` writes directly into the live cache and computes
`.sha256` sidecars from the bytes it just downloaded. Those hashes can detect
later accidental changes, but they do not establish provenance. A compromised
download can supply both the content and its newly generated hash.

Installation also lacks a complete transaction, explicit compressed and
expanded size limits, an operation timeout, a strict archive allowlist, and a
runtime smoke gate before the ready marker is written.

### Trust decision

Embed the expected bundle manifest in the CLI. The CLI already pins
`ModelVersion`, so an embedded digest is the simplest trust anchor for the
current release model.

A remote `.sha256` file downloaded beside an archive is not a trust anchor
unless that checksum is independently signed and its public key is pinned in
the CLI. Signed remote manifests can be added later if model updates must be
decoupled from CLI releases.

### Trusted manifest contents

Use a versioned schema, embedded with `go:embed`, containing at least:

- manifest schema version;
- model bundle version;
- immutable model repository revision, not `resolve/main`;
- ONNX Runtime version and immutable source identity;
- supported GOOS/GOARCH key;
- exact archive filename, SHA-256, and byte size;
- a defensive maximum compressed size;
- exact required inner filenames;
- exact inner-file SHA-256 and byte size;
- a defensive maximum total expanded size;
- the runtime library filename for that target;
- optional build/provenance metadata that is not used as a substitute for
  hashes.

Reject unknown schema versions, duplicate platform entries, missing required
fields, invalid hashes, and a manifest version that differs from the compiled
`ModelVersion`.

### Release-source lock and reproducibility

Update `scripts/prepare-pii-model.sh` and the release workflow so provenance
starts before bundle creation:

1. Replace the Hugging Face `resolve/main` URL with an immutable commit.
2. Check in a source-lock manifest with expected hashes for model inputs and
   ONNX Runtime source archives.
3. Verify every upstream input before extracting or bundling it.
4. Produce deterministic archives: sorted entries, normalized ownership,
   permissions and timestamps, and deterministic gzip headers.
5. Generate the bundle manifest from those deterministic artifacts.
6. Check the trusted bundle manifest into the CLI package for review.
7. On a model release tag, rebuild and require a byte-for-byte manifest match
   before publishing.
8. Upload the archives, public checksums, and manifest. Public checksums remain
   useful for operators, while the CLI trusts its embedded copy.
9. Publish only targets in the tested capability matrix.

The release workflow currently uploads only `*.tar.gz`; update its asset
coverage test so a missing manifest/checksum or an extra unsupported bundle
fails the release job.

### Installer boundary

Keep network/filesystem mechanics behind one service rather than spreading
them across command handlers:

```go
type BundleInstaller struct {
    Client          *http.Client
    CacheRoot       string
    Platform        Platform
    Manifest        TrustedManifest
    Limits          InstallLimits
    ValidateRuntime func(context.Context, Paths) error
}
```

It is usually enough to inject the cache root, HTTP client, platform, limits,
and runtime validator. Avoid introducing a broad virtual filesystem interface
unless tests demonstrate a need; temporary directories provide a realistic
filesystem boundary.

### Transactional install algorithm

1. Check platform capability before creating a request or directory.
2. Select exactly one trusted manifest entry for the current target.
3. Create a private staging directory under the cache root so final promotion
   stays on one filesystem.
4. Use a context-aware request and an HTTP client with bounded response-header,
   redirect, and total-operation behavior.
5. Reject a declared `Content-Length` over the limit, but still enforce a
   streaming `LimitReader` because the header can be absent or false.
6. Stream the archive to a private staging file while hashing and counting it.
7. Require exact archive SHA-256 and expected size before extraction.
8. Extract only strict manifest allowlisted names. Normalize paths and reject,
   rather than skip:
   - absolute paths and traversal;
   - duplicate entries;
   - unexpected files;
   - symlinks, hardlinks, devices, FIFOs, and other non-regular entries;
   - entries larger than their expected size;
   - total expanded bytes over the configured limit.
9. Hash and count each extracted file while writing. Require exact inner-file
   hashes/sizes and require every expected file exactly once.
10. Use safe application-selected permissions rather than archive-controlled
    modes.
11. Load the staged runtime/model and run a tiny smoke validation.
12. Write trusted install metadata and a ready marker only after all checks
    pass.
13. Promote the completed staging directory without modifying a currently
    usable installation.
14. On every failure, remove staging data and leave the previous installation
    untouched.

Prefer immutable, content-addressed final paths such as:

```text
<cache>/versions/<model-version>/<goos>-<goarch>/<archive-sha256>/
```

The compiled manifest already identifies the expected path, so a mutable
`current` pointer is unnecessary. This also avoids cross-platform differences
when atomically replacing a non-empty directory. Clean older versions and
stale staging directories as a separate bounded maintenance step.

`ModelPaths` must resolve paths only from a `ready` status. A legacy flat cache
or `.version`-only installation should be reported as `installed-unverified`
and must be reinstalled before NER is attached.

Do not hash a large model on every CLI invocation solely to protect against a
local privileged attacker. Full hashes are required during trusted install;
normal readiness can validate trusted metadata, expected paths, marker
identity, and required file sizes. Document the local-cache threat model and
provide a future explicit `verify` command if full at-rest verification is
desired.

### Regression fixtures

Build tiny tar.gz archives in memory and serve them from `httptest.Server`.
Use a fake runtime validator. No unit test should need the real model.

Happy path:

- `TestInstaller_TrustedBundlePromotesAtomically`
- progress reporting receives bounded, monotonic byte counts;
- status becomes `ready` only after runtime validation;
- a second install is idempotent and performs no download.

Trust failures:

- `TestInstaller_RejectsMutatedArchive`
- `TestInstaller_RejectsWrongPlatformOrVersion`
- `TestInstaller_RejectsInnerFileHashMismatch`
- `TestInstaller_RejectsMissingRequiredFile`
- `TestInstaller_RejectsUnexpectedOrDuplicateEntry`

Archive safety:

- absolute path;
- `../` traversal and Windows separator variants;
- symlink and hardlink;
- device/FIFO entry;
- entry larger than declared;
- total expanded-size overflow;
- compressed download limit overflow;
- truncated gzip/tar stream.

Transaction failures:

- non-200 response;
- redirect/timeout/cancellation;
- write failure where practical;
- runtime smoke failure;
- ready-marker failure;
- an existing valid installation remains byte-for-byte usable after each
  failed replacement;
- no final directory or ready marker survives a failed first install;
- stale staging cleanup cannot delete a resolved path outside the cache root.

Status and release tests:

- `.version` alone is never `ready`;
- legacy flat files are `installed-unverified`;
- missing required files are `corrupt`;
- the embedded manifest covers exactly the supported release matrix;
- generated release assets and the checked-in manifest match.

### Acceptance criteria

- Every accepted byte is covered by an embedded trust anchor before becoming
  usable.
- Failed or interrupted installation cannot damage an existing usable model.
- Network and extraction resource use is explicitly bounded.
- No legacy self-generated sidecar is treated as evidence of provenance.
- NER is attached only after the exact current bundle passes validation.

## Test and commit strategy

Implement in small, independently reviewable green slices:

1. Strict mode regression tests and fix (#6).
2. Secret resolver regression tests and fix (#5).
3. Capability/status regression tests and unsupported-target gate (#7A).
4. Manifest/installer regression fixtures (#8 + #9).
5. Trusted manifest and transactional installer implementation (#8 + #9).
6. Windows loader and real CI smoke test (#7B), if Windows support is required
   in this delivery.

It is useful to prove each regression red locally before implementing its fix.
Keep shared branch commits green; if the project intentionally retains a red
test-only commit for review, place its green fix immediately after it.

Suggested verification after each slice:

```text
go test ./internal/pii/... ./internal/config/... ./internal/cmd/... -count=1
go test ./... -count=1
go test -race ./internal/pii/... ./internal/config/... ./internal/cmd/...
```

The Windows runtime slice additionally requires the real Windows integration
job. The ordinary unit suite must remain fixture-only and offline.

## Compatibility and operational notes

| Risk | Required mitigation |
| --- | --- |
| Explicit-secret display identities drift | Pin a golden fixture before refactoring and keep raw secret bytes/HMAC inputs unchanged. |
| Empty-secret users see new pseudonyms | Expected one-time privacy migration; document it in release notes. |
| Headless host has no keyring | Fail with an actionable `HS_INBOX_PII_SECRET` instruction; never save the key in YAML. |
| Concurrent first-run processes create different keys | Cross-process initialization lock and convergence test. |
| Invalid config cannot be repaired | Keep config commands outside runtime validation and use file-only mutation loading. |
| Valid environment overrides get persisted | Never use environment-merged config as the input to `config set`. |
| Windows is advertised prematurely | Capability matrix is driven only by executing CI smoke tests. |
| Trusted download corrupts live cache before verification | Stage, validate, smoke-test, then promote immutable content. |
| Archive digest changes between builds | Make bundle creation reproducible and assert manifest equality in release CI. |
| Readiness checks become slow | Hash fully during installation; use trusted immutable metadata for normal startup. |

## Explicitly deferred work

Do not combine these fixes with unrelated identity-contract changes:

- PII-F01: cross-payload identity aliasing;
- issue 11: visual fake-name collisions;
- issue 12: Unicode identity normalization;
- issue 13: user-facing secret rotation/migration;
- issue 14: the multilingual quality corpus.

The secret record should be versioned now, but no rotation command or identity
schema migration belongs in this delivery. Similarly, the Windows capability
gate is required; native Windows inference can remain a separate follow-up.

## Overall definition of done

- All regression and existing critical tests pass.
- Invalid modes and secret-store failures stop affected commands before API
  work and protected output.
- Enabled engines always use a private HMAC key.
- Explicit-secret deterministic display identities remain unchanged.
- Model status is truthful on every build target.
- Unsupported platforms cannot download/install a misleading bundle.
- Supported model bundles are pinned, bounded, validated, smoke-tested, and
  transactionally promoted.
- Release tooling uses immutable upstream inputs and publishes only tested
  targets.
- The progress report is updated only after the relevant acceptance criteria
  are demonstrated by tests.
