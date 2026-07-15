# PII Redaction — Progress Report

Last updated: 2026-07-16

## Executive status

- Original review findings: **14**
- Critical findings: **4 of 4 complete**
- High findings: **4 of 4 complete**
- Original findings complete: **9 of 14**
- Remaining original findings: **5 open**
- Additional observations raised during remediation: **2 open**
- Regression-test commit: `7eb27a9 test: reproduce critical PII redaction leaks`
- Critical-fix commit: `5a7ea63 fix: enforce PII redaction boundaries`
- High-fix commits:
  - `fce500b fix: reject invalid PII modes before Inbox work`
  - `0cfa643 fix: require private keys for PII pseudonyms`
  - `f304833 fix: gate PII models by runtime capability`
  - `f4416c3 fix: verify and atomically install PII models`
- Latest verification:
  - `go test ./... -count=1 -timeout=5m`
  - `go vet ./...`
  - digest-pinned Docker race suite via `scripts/test-race.ps1`
  - native Windows race suite via `scripts/test-race-native-windows.ps1`
  - `go build -buildvcs=false ./...` for Linux and Darwin on amd64 and arm64
  - release-script syntax validation with Git Bash
  - embedded archive sizes and SHA-256 values checked against all four
    published `pii-model-v0.2.0` assets

Race verification is now complete. CGO was present but disabled, and the
default GCC configured by Go was absent. The complete suite passes in the
digest-pinned Docker test environment and natively on Windows with a compatible
MinGW-w64 compiler, including the Windows-only PII secret-store lock tests.
Docker is the canonical local dependency; a preflighted native Windows wrapper
runs in CI for platform-specific coverage.

The release workflow requires a real runtime/model smoke test on each
advertised Linux and Darwin target before publishing; those four jobs will
produce execution evidence when the model-tag workflow next runs.

This document preserves the original 1–14 review numbering. The more detailed
deferred register remains in `pii-redaction-follow-ups.md`. `PII-F01` was found
during the remediation design discussion and was not part of the original
numbered list, so it is tracked separately below rather than changing the
historical numbering.

The implementation design used for high issues 5–8 is recorded in
`pii-redaction-high-issues-implementation-plan.md`. It also includes issue 9
because provenance verification and transactional installation must ship as
one model-install boundary.

## What was already good

The original review found real architectural strengths worth preserving:

1. **The privacy objective is sound.** The feature is designed to keep support
   output useful while removing identifying data, rather than making the output
   unusable through blanket deletion.
2. **The code already had useful separation of concerns.** Policy, identity
   generation, structured JSON traversal, free-text detection, model runtime,
   and command presentation were separate enough to improve without replacing
   the whole feature.
3. **Display identities are deterministic and stateless.** Fake names and
   emails are derived rather than stored in a reversible lookup table. This is
   valuable for following the same person across outputs without persisting a
   real-to-fake mapping.
4. **A private secret is supported.** When `HS_INBOX_PII_SECRET` is set, HMAC is
   used so pseudonyms can be stable within an environment without being
   globally predictable.
5. **The policy model is understandable.** `off`, `customers`, and `all` express
   useful operator intent, and the guarded `--unredacted` override is explicit
   rather than an accidental bypass.
6. **Customer and staff identities are conceptually distinct.** The
   customers-only mode is intended to preserve internal team context while
   anonymizing customers.
7. **Detection is layered.** Structured identity fields, known identities,
   multilingual NER, and regex detection cover different PII shapes rather than
   relying on one mechanism.
8. **The no-model fallback is privacy-oriented.** When NER is unavailable,
   free-form content is hidden instead of being emitted without name
   inspection.
9. **NER runs locally.** Conversation content is not sent to a third-party
   inference service as part of redaction.
10. **There was already meaningful test coverage.** Identity determinism,
    customer/user policy, structured fields, regex PII types, NER behavior,
    archive traversal, and command behavior all had tests to build on.

The critical remediation deliberately retained these properties. In
particular, `personKey` precedence and fake-person derivation were not changed.

## Original findings 1–14

| # | Severity | Finding | Status | Tracking |
| --- | --- | --- | --- | --- |
| 1 | Critical | Top-level customer JSON can bypass customers-mode redaction because the object has no usable entity classification. | [x] Complete | Critical regression 01 |
| 2 | Critical | The presentation boundary is opt-in, so commands and fields that call output directly can leak PII. | [x] Complete | Critical regression 02 |
| 3 | Critical | HTTP debug logging persists raw URLs, headers, request bodies, and response bodies before output redaction. | [x] Complete | Critical regression 03 |
| 4 | Critical | ASCII `\b` name replacement misses Unicode-script NER names and partial NER failures are not fail-closed. | [x] Complete | Critical regression 04 |
| 5 | High | An empty `HS_INBOX_PII_SECRET` falls back to unkeyed SHA-256. | [x] Complete | `PII-F02` |
| 6 | High | Invalid PII mode values silently normalize to `off`. | [x] Complete | `PII-F03` |
| 7 | High | Windows can install a model bundle that the bundled runtime cannot execute. | [x] Complete | `PII-F04` |
| 8 | High | Downloaded model hashes are generated after download and are not independent trust anchors. | [x] Complete | `PII-F05` |
| 9 | Medium | Model installation is non-atomic and lacks explicit download size, extraction size, and timeout limits. | [x] Complete | `PII-F06` |
| 10 | Medium | CLI and MCP arguments can expose PII through argv, shell history, process inspection, and echoed command errors. | [ ] Open | `PII-F07` |
| 11 | Medium | The small fake-name lists permit visually identical pseudonyms for different people. | [ ] Open | `PII-F08` |
| 12 | Medium | Identity canonicalization does not normalize Unicode composition or define broader Unicode case/space semantics. | [ ] Open | `PII-F09` |
| 13 | Medium | Secret rotation changes every identity without a key version or migration policy. | [ ] Open | `PII-F10` |
| 14 | Medium | There is no maintained multilingual privacy corpus measuring false negatives and false positives. | [ ] Open | `PII-F11` |

## Completed critical work

### 1. Top-level customer classification — complete

Original problem:

- The JSON walker inferred an empty entity type for a top-level customer.
- Identity extraction could default a `firstName`/`lastName` identity to
  customer, but policy had already been evaluated using the empty entity type.
- In `customers` mode, customer list/get JSON could therefore retain the real
  name, email, and phone.

Implemented:

- Added explicit `JSONContext` with root-entity and resource context.
- Customer, user, conversation, rating, report, and attachment presentation
  paths now supply the facts known by the command layer.
- Entity classification happens before identity extraction.
- Explicit payload type, semantic key, and caller context take precedence over
  shape defaults, so user objects are not accidentally treated as customers.
- The existing email-first `personKey` and deterministic fake-person algorithm
  remain unchanged.

Evidence:

- `TestPIIRegression_Critical01_CustomersModeRedactsTopLevelCustomerJSON`
  covers customer list/get in both `json` and `json-full` formats.
- `TestRedactJSONWithContext_TopLevelEntityPolicy` proves customer roots are
  redacted while user roots remain visible in `customers` mode.
- `TestRedactPerson_DisplayIdentityCompatibility` pins the existing
  deterministic display-identity contract across engine instances.

### 2. Mandatory Inbox presentation boundary — complete

Original problem:

- PII protection depended on each command remembering to call a redaction
  helper.
- Ratings, reports, attachments, conversation custom-field values, and other
  direct `output.PrintRaw` paths could bypass it.
- Redaction failure fell back to the original raw payload.

Implemented:

- Every Inbox raw JSON write now routes through `printRawWithPII`.
- The presenter is fail-closed when PII mode is enabled: invalid or
  uninspectable JSON returns an error and emits no original payload.
- JSON traversal is path-aware rather than globally treating every `value`
  field as PII.
- Email and phone collection values are redacted according to their container.
- Custom-field values are inspected as free text.
- Rating comments, report table summaries, and attachment filenames are
  protected in non-JSON views as well as raw JSON views.
- Attachment `data` is replaced with an opaque marker while redaction is
  enabled; obtaining the original still requires an explicitly allowed
  `--unredacted` request.
- MCP output continues through the same CLI presentation boundary.

Evidence:

- `TestPIIRegression_Critical02_PIIBearingCommandsCannotBypassRedaction`
  covers ratings, reports, attachment metadata, and custom fields with fixtures
  rather than Help Scout API calls. Relevant commands are exercised in table
  and JSON formats.
- `TestPIIOutputBoundary_FailsClosedForInvalidJSON` proves raw fallback is gone.
- `TestPIIOutputBoundary_AllInboxJSONUsesPresenter` prevents new Inbox handlers
  from calling `output.PrintRaw` directly.
- `TestRedactJSONWithContext_PathAwareSensitiveValues` proves custom values and
  attachment data are protected without destroying unrelated `value` fields.

### 3. Safe HTTP diagnostics — complete

Original problem:

- Debug logging ran at the transport layer before presentation redaction.
- Query values, sensitive headers, request bodies, response bodies, and even
  transport errors could persist customer PII or credentials.
- Normal `--unredacted` policy could not safely govern a persistent diagnostic
  file.

Implemented:

- Added a diagnostic sanitizer independent of user-facing PII mode and
  `--unredacted`.
- Query values, URL user information, fragments, numeric identifiers, and
  email/phone-like path segments are removed or replaced.
- Header values use a narrow diagnostic allowlist; credentials, cookies, API
  keys, and unknown headers are replaced.
- JSON bodies use strongest-mode structured redaction with fail-closed
  free-text handling.
- Non-JSON and unreadable bodies receive safe placeholders.
- Error text is not persisted because transport errors can embed the original
  URL.
- Sanitization operates on copies; the original request and response bodies
  remain available to the HTTP caller.
- Debug files are created/truncated with private `0600` permissions, including
  correction of permissions on an existing file.
- OAuth token requests remain excluded from logging entirely.

Evidence:

- `TestPIIRegression_Critical03_DebugTransportDoesNotPersistPII` proves URL,
  request-body, response-body, email, and phone values are absent while the
  response body returned to the caller is unchanged.
- Debug transport tests cover credential headers, body preservation, auth
  request exclusion, safe ordinary JSON, and transport errors.

### 4. Unicode-safe NER replacement — complete

Original problem:

- Go regexp `\b` uses ASCII word-boundary behavior.
- Arabic, Chinese, accented Latin, and other Unicode names returned by NER
  could remain unchanged.
- The detector's byte offsets were ignored, and per-chunk inference failures
  were silently skipped.

Implemented:

- NER replacements now use validated byte spans from the original text.
- Edits are applied right-to-left so earlier offsets remain valid.
- Overlapping spans are coalesced so an uncovered suffix cannot leak.
- Exact-literal recovery handles a tokenizer offset immediately adjacent to the
  reported source span; irrecoverable spans hide the whole field.
- Known customer identities are still replaced through the existing
  `personKey` path so their deterministic display identity is preserved.
- Literal known-name replacement now uses rune-aware Unicode boundaries, with
  boundaryless handling for scripts that commonly omit whitespace.
- Detector and chunk failures propagate and cause fail-closed free-text output.

Evidence:

- `TestPIIRegression_Critical04_NERNamesWithUnicodeBoundariesAreRedacted`
  covers Arabic, Chinese, and accented Latin names.
- Additional tests cover overlapping spans, known/NER overlap, protected staff
  names in customers-only mode, deterministic output, and detector failure.

## Completed high-severity and model-installer work

### 5. Private keyed display identities — complete

- Enabled engines now require an opaque non-empty `pii.Secret` and always use
  HMAC; the unkeyed SHA-256 branch no longer exists.
- An explicit `HS_INBOX_PII_SECRET` retains its exact bytes and preserves the
  baseline deterministic display identities.
- When the environment variable is absent or blank, the application resolves
  a generated 32-byte installation secret from the OS keyring.
- Cross-process initialization locking makes concurrent first use converge on
  one secret. Keyring, RNG, and lock failures stop protected work before API
  access and never persist secret material in YAML or diagnostics.

Evidence includes resolver precedence, keyring round-trip, concurrent
initialization, lock, command preflight, failure-path, and golden identity
tests. Implemented in `0cfa643`.

### 6. Strict mode parsing — complete

- PII modes are parsed into a strong type and unknown file or environment
  values return an actionable error instead of becoming `off`.
- Inbox and MCP paths validate before Help Scout API work.
- Config repair remains possible through file-only loading, and environment
  overrides are not accidentally written back to YAML.

Implemented with regression coverage in `fce500b`.

### 7. Truthful runtime capability — complete

- Runtime capability is explicit and currently limited to Linux and Darwin on
  amd64 and arm64.
- Windows installation is rejected before HTTP or cache mutation, status
  explains that free-form content remains hidden, and cached marker/files
  cannot manufacture a ready state.
- Legacy bundles are reported as unverified and are not loaded.
- Release tooling publishes only supported targets and the model-tag workflow
  now smoke-tests real inference on all four advertised runner targets before
  release.

Unit, command, fail-closed fallback, cross-build, release-matrix, and workflow
coverage is in `f304833` and `f4416c3`. The four executing CI smoke results are
pending the next model-tag workflow run.

### 8. Independent model provenance — complete

- The CLI embeds a schema-versioned trusted manifest for every supported
  target, containing the immutable model revision, ONNX Runtime source
  identity, archive URL/name/size/SHA-256, inner filenames/sizes/SHA-256 values,
  runtime library name, and defensive limits.
- The embedded archive identities match the four currently published release
  assets. A downloaded sidecar is not treated as a trust anchor.
- Manifest parsing rejects unknown schemas/fields, duplicates, missing target
  coverage, unsupported targets, unsafe names, invalid hashes, and inconsistent
  bounds.
- The release source lock pins every upstream input; the builder verifies them,
  creates deterministic archives, emits public checksums and a candidate
  manifest, and CI requires the generated and reviewed manifests to match.

Implemented with offline manifest/release regressions in `f4416c3`.

### 9. Transactional and bounded model installation — complete

- Downloads use a context-aware client with response-header, redirect, and
  total-operation bounds; exact compressed size and SHA-256 are required.
- Extraction accepts only the four manifest files and rejects traversal,
  absolute/Windows paths, duplicates, unexpected entries, links, devices,
  FIFOs, wrong sizes/hashes, expanded-size overflow, and truncated streams.
- Files are installed into a private staging directory, synced, runtime/model
  smoke-tested, marked ready only after success, and atomically promoted to an
  immutable content-addressed directory.
- Failures remove staging safely or leave a native validation timeout for
  bounded stale cleanup. A prior trusted installation remains untouched.
- Normal readiness validates trusted marker identity and exact regular-file
  sizes without rehashing the large model on every invocation.

Fixture-only regressions cover success/idempotence, progress, tampering,
archive attacks, HTTP/cancellation/runtime failures, concurrency, stale
cleanup, marker/file damage, and preservation of a previous install. Implemented
in `f4416c3`.

## Open original findings

### 10. Sensitive arguments cross process boundaries — open

Emails, search terms, message bodies, and custom-field values can enter argv.
The MCP runner also reconstructs command lines in errors. Prefer stdin or
protected files for sensitive input, redact reconstructed errors, and
eventually let MCP call application services without a CLI subprocess.

### 11. Visual pseudonym collisions — open

Different identities can receive the same fake first/last name because table
views may omit the generated email suffix. Add a readable deterministic
disambiguator without exposing a source identifier.

### 12. Unicode identity canonicalization is incomplete — open

NFC and NFD spellings can derive different person keys despite appearing
identical. Define normalization, case-folding, and whitespace rules as a
versioned identity-key schema before changing current output.

### 13. Secret rotation is unversioned — open

Changing the secret intentionally changes every display identity, but output
has no key identifier and there is no migration or historical-correlation
policy. Design rotation together with issues 5 and 12.

### 14. No multilingual quality corpus — open

Build a synthetic, non-production evaluation corpus measuring privacy recall,
false positives, customer/staff separation, long-content behavior, and
performance across supported languages and scripts. Never use raw production
content to construct it.

## Additional observations raised during remediation

### A1 / PII-F01. Identity keys vary across payload shapes — open

The current contract intentionally remains email first, full name second, and
phone-only fallback in free text. The same real person can therefore receive a
different pseudonym when one payload contains email and another contains only
name or phone. A future ID/alias strategy needs account scoping, key-schema
versioning, and an explicit backwards-compatibility decision.

### A2. NER detector lifetime is process-scoped — open

`ner.Detector` exposes `Close`, but the command helper returns only a
`pii.Engine` and does not provide an explicit invocation-scoped close path. The
current CLI and MCP subprocess model lets process exit reclaim the runtime, but
an in-process server or future application-service integration should give the
engine/detector explicit ownership and deterministic cleanup.

## Suggested next sequence

1. Reduce argv/MCP exposure in issue 10 before expanding sensitive write tools.
2. Design issues 11–13 and A1 together as a versioned identity-key,
   disambiguation, normalization, and secret-rotation contract.
3. Build issue 14's evaluation corpus before changing model or normalization
   behavior, so privacy regressions become measurable.
4. Give the NER detector explicit invocation-scoped ownership before moving
   the CLI/MCP flow into a long-lived in-process service.
5. Treat native Windows inference as a separate capability: add it only after
   a real Windows loader/model smoke job passes.
