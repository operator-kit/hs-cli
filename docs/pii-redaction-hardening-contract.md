# PII Redaction Hardening Contract

Last reviewed: 2026-07-16

This is the normative engineering contract produced by the 14-finding PII
redaction review. The progress report records history and implementation detail;
this document states the invariants future code must preserve. All 14 original
findings are complete.

“Fail closed” means that when required classification, inspection, key
material, runtime capability, or artifact trust cannot be established, the CLI
must stop the protected operation or hide the affected content. It must never
fall back to emitting the original value.

## Contract 1 — Classify entities before applying policy — complete

Customer/staff policy must not be inferred only from object shape. A caller
that knows the resource or root entity must supply that context, and entity
classification must happen before identity extraction or redaction policy is
evaluated. Explicit payload type, semantic container key, and caller context
take precedence over shape-based defaults.

This preserves customers-only behavior: customer roots are redacted, while
known staff roots remain visible.

Regression proof:

- `TestPIIRegression_Critical01_CustomersModeRedactsTopLevelCustomerJSON`
- `TestRedactJSONWithContext_TopLevelEntityPolicy`

## Contract 2 — Use one mandatory Inbox presentation boundary — complete

Every Inbox JSON response must pass through `printRawWithPII`; new handlers must
not call `output.PrintRaw` directly. When PII protection is enabled, invalid or
uninspectable JSON must produce an error and no original payload. Non-JSON
views must protect the same sensitive fields, and opaque attachment data must
remain opaque unless an explicitly authorized unredacted request is active.

MCP is not a separate privacy path: its output must pass through the same CLI
presentation boundary.

Regression proof:

- `TestPIIRegression_Critical02_PIIBearingCommandsCannotBypassRedaction`
- `TestPIIOutputBoundary_FailsClosedForInvalidJSON`
- `TestPIIOutputBoundary_AllInboxJSONUsesPresenter`
- `TestRedactJSONWithContext_PathAwareSensitiveValues`

## Contract 3 — Sanitize diagnostics independently of display mode — complete

Persistent HTTP diagnostics must be sanitized before writing, independently of
the user-facing PII mode and `--unredacted`. URLs, user information, query
values, fragments, sensitive or unknown headers, request bodies, and response
bodies must be reduced to allowlisted metadata or strongest-mode redacted
content. Unreadable/non-JSON bodies and transport errors must use safe
placeholders rather than raw fallback.

Sanitization must operate on copies so callers still receive the original HTTP
response. Debug files must remain private, and OAuth token exchanges must not
be logged.

Regression proof:

- `TestPIIRegression_Critical03_DebugTransportDoesNotPersistPII`
- the remaining tests in `internal/api/debug_test.go`

## Contract 4 — Treat NER spans as Unicode byte ranges and fail closed — complete

NER replacements must use validated byte spans from the original UTF-8 text,
not ASCII word-boundary regexes. Overlapping edits must not leave an uncovered
suffix, and edits must be applied without invalidating earlier offsets. Known
identity matching must use Unicode-aware boundaries, including scripts that do
not normally use spaces.

Invalid spans, detector errors, and per-chunk inference failures must hide the
whole free-text field. They must not return partially inspected content.

Regression proof:

- `TestPIIRegression_Critical04_NERNamesWithUnicodeBoundariesAreRedacted`
- `TestRedactText_NERFailureFailsClosed`

## Contract 5 — Require private keyed pseudonyms — complete

An enabled engine must have non-empty private HMAC key material. Unkeyed hashes
are forbidden. With neither explicit key environment variable present, the CLI
must resolve the existing installation key or generate one cryptographically on
first use, persisting it only in the OS keyring under cross-process
initialization locking.

If either explicit key variable is present, both must be present and non-blank;
invalid, partial, blank, keyring, RNG, or lock states must stop protected work
before Help Scout API access. PII key material must not be persisted in YAML or
diagnostics.

Regression proof:

- `TestPIIRegression_High05_SecretFailureStopsBeforeInboxAPI`
- `TestNewEngineRequiresSecretOnlyWhenEnabled`
- the resolver failure, generation, reuse, and concurrency tests in
  `internal/pii/setup/secret_test.go`

## Contract 6 — Parse PII modes strictly before work starts — complete

Only `off`, `customers`, and `all` are valid modes. Unknown file or environment
values must return an actionable error; they must never normalize to `off`.
Inbox commands and MCP startup must validate the effective mode before API or
protected-key work. File-only configuration loading must retain a safe repair
path without writing environment overrides back to YAML.

Regression proof:

- `TestPIIRegression_High06_InvalidModeStopsBeforeInboxAPI`
- `TestPIIRegression_High06_InvalidModeStopsMCPStartup`
- `TestParseModeRejectsUnknownValues`
- the invalid-mode repair tests in `internal/cmd/config_test.go`

## Contract 7 — Advertise only executable model runtimes — complete

Runtime support is an explicit capability, currently Linux and Darwin on amd64
and arm64. An unsupported platform must be rejected before download or cache
mutation, installed-looking files must not manufacture a ready state, and
free-form content must remain fail-closed.

A new platform is not supported until its real runtime library and model pass
the release smoke test on that native target. Cross-compilation alone is not
evidence of inference support.

Regression proof:

- `TestRuntimeCapabilityMatrix`
- `TestModelStatusUnsupportedEvenWithInstalledFiles`
- `TestEnsureModelUnsupportedPlatformDoesNotDownloadOrMutateCache`
- `TestPIIRegression_High07_UnusableRuntimeKeepsFreeTextFailClosed`
- `TestPIIModelReleaseWorkflowParsesAndSmokesEverySupportedPlatform`

## Contract 8 — Anchor model trust in reviewed embedded identities — complete

Downloaded checksums and sidecars are operator artifacts, not trust anchors.
The CLI must verify the archive and every installed file against the reviewed,
schema-versioned manifest embedded in the binary. The manifest must cover
exactly the supported targets and pin immutable source identity, URL/name,
sizes, hashes, runtime filename, and defensive bounds.

Unknown schemas/fields, duplicate coverage, unsafe names, invalid identities,
or inconsistency with the release source lock must fail before download or
installation.

Regression proof:

- `TestTrustedManifestPinsPublishedReleaseDigests`
- `TestTrustedManifestRejectsUnknownFieldsAndTrailingJSON`
- `TestTrustedManifestRejectsDuplicateOrUnsupportedCoverage`
- `TestReleaseSourceLockMatchesTrustedManifest`

## Contract 9 — Install models transactionally within hard bounds — complete

Model download and extraction must be context-aware and enforce exact
compressed/expanded sizes, hashes, timeouts, redirect policy, entry allowlists,
and safe regular-file paths. Traversal, absolute or Windows paths, duplicates,
links, devices, FIFOs, truncated streams, and unexpected entries are forbidden.

Validation occurs in a private staging directory. A ready marker is written
only after real runtime/model smoke validation, and promotion is atomic into an
immutable content-addressed directory. Failure must preserve any prior trusted
installation; cleanup must remain bounded to installer-owned staging entries.

Regression proof:

- `TestInstallerTrustedBundlePromotesAtomically`
- `TestInstallerRejectsUnsafeOrIncompleteArchive`
- `TestInstallerRejectsOversizedAndTruncatedResponses`
- `TestInstallerFailureLeavesPriorTrustedInstallUntouched`
- the remaining installer tests in `internal/pii/ner/installer_test.go`

## Contract 10 — Keep protected command values off supported argv paths — complete

Flags capable of carrying PII, credentials, authored text, or private local
paths must be annotated with `markProtectedFlags` and rejected when supplied
directly. Their supported transport is the bounded schema-1 envelope over stdin
or a private regular file. The envelope must match the exact command, reject
unknown fields/types, enforce its size bound, reject links/non-regular files,
and enforce private Unix permissions.

MCP must put string values and positionals in the envelope, use placeholders in
child argv and safe displays, and scrub protected values from errors, stdout,
and stderr. Documentation must never suggest embedding an envelope containing
PII in a shell command: rejection cannot erase a value the shell has already
recorded.

Regression proof:

- `TestPIIRegression_Medium10_SensitiveMCPInputsStayOffProcessBoundaries`
- the parser/file tests in `internal/cmd/protected_input_test.go`
- `TestBuildMCPToolSpec_ClassifiesProtectedFlags`
- `TestRedactProtectedValues_CoversQuotedAndJSONEscapedErrors`

## Contract 11 — Make full display names deterministic and distinguishable — complete

For already-canonical inputs, the deterministic first name, base last name, and
legacy generated email remain stable. The displayed last name must also include
the public key ID and a 40-bit HMAC-derived disambiguator, so distinct people
that collide in the small name lists do not render as the same full display
name. Source identifiers must not be exposed.

This contract does not claim that the compatibility-preserved generated email
namespace is collision-free; that residual risk is tracked as `PII-F13`.

Regression proof:

- `TestPIIRegression_Medium11_DisplayNamesDisambiguateDistinctPeople`
- `TestRedactPerson_DisplayIdentityCompatibility`

## Contract 12 — Version identity canonicalization — complete

Identity schema `v2` is part of the pseudonym contract. Names are canonicalized
with NFC, Unicode case folding, NFC recomposition, and Unicode whitespace
collapse. Emails retain established trim/lowercase semantics and add NFC.
Visually identical NFC/NFD forms must resolve to the same identity key.

Canonicalization semantics must not change silently. A future change requires
a new identity schema, compatibility fixtures, and an explicit migration
decision.

Regression proof:

- `TestPIIRegression_Medium12_IdentityKeysNormalizeUnicodeComposition`
- `TestIdentityCanonicalizationV2FoldsUnicodeCaseAndWhitespace`

## Contract 13 — Make rotation context explicit and independently identified — complete

Every enabled engine receives one `PseudonymContext` containing private key
material, a public key ID, and the identity schema. There is no secret-only
engine construction path. The public ID is supplied/generated independently;
it must never be derived from the private key.

Explicit deployments set and rotate `HS_INBOX_PII_SECRET` and
`HS_INBOX_PII_KEY_ID` together. Stored records are schema-versioned. Legacy raw
32-byte records are migrated under the initialization lock, preserving their
exact secret bytes and assigning an independent random public ID. A persisted
record must be reloaded and validated before use.

Regression proof:

- `TestPIIRegression_Medium13_SecretRotationHasAnExplicitKeyIdentifier`
- `TestSecretResolverEnvironmentRequiresSecretAndPublicKeyIDTogether`
- `TestSecretResolverRejectsExplicitBlankEnvironmentValues`
- `TestSecretResolverMigratesLegacyRawSecretUnderLock`
- `TestSecretResolverConcurrentInitializersConverge`

## Contract 14 — Measure multilingual privacy and preservation — complete

The checked-in evaluation corpus must be synthetic and contain no Help Scout or
production data. Hermetic tests must cover Latin, Arabic, and Han scripts;
redact and preserve expectations; people, addresses, identifiers,
customer/staff separation; false-positive protection; and long content.

The native model-release smoke must run the same cases on every advertised
target, require complete coverage of expected names, reject unexpected person
spans, exercise long-content chunking, and enforce a runtime budget. A normal
unit test may skip real inference only because model artifacts are deliberately
not checked into the repository.

Regression proof:

- `TestPIIRegression_Medium14_MultilingualPrivacyCorpusIsMaintained`
- `TestRuntimeBundleSmoke`
- `.github/workflows/pii-model.yml` native smoke matrix

## Additional contract PII-F12 — Enforce ModeOff at low-level identity boundaries — complete

Every public identity-redaction method must enforce the engine mode itself.
When the engine is `ModeOff`, `RedactPerson`, `RedactEmail`, and `RedactPhone`
must return their exact inputs without hashing, deriving a pseudonym from the
zero context, or mutating pseudonym caches. Callers must not need to remember an
outer JSON/text guard for the mode contract to hold.

Higher-level guards remain required as cheap early exits; they are not a
substitute for encapsulation at the public low-level boundary.

Regression proof:

- `TestPIIRegression_FollowUp12_ModeOffLowLevelIdentityMethodsPassThrough`

## Rules for extending the PII surface

Before merging a new PII-bearing path:

1. Supply explicit `JSONContext` whenever the caller knows resource/entity
   semantics.
2. Route Inbox presentation through the mandatory redaction boundary.
3. Mark sensitive flags as protected and keep examples free of inline PII.
4. Add synthetic regression fixtures for both redaction and preservation.
5. Treat detector, parser, key, runtime, and trust failures as fail-closed.
6. Version any identity-key, canonicalization, or display-format change and pin
   compatibility expectations.
7. Require real native inference before advertising a new model platform.
8. Preserve transactional installation and the independently reviewed trust
   manifest.

Open design work is tracked separately in
[`pii-redaction-follow-ups.md`](../.agents/notes/pii-redaction-follow-ups.md).
Those observations do not reduce the completed status of the 14 original
findings, but they are boundaries this contract intentionally does not claim to
solve.
