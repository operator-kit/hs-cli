# PII Redaction — Deferred Observations

Last reviewed: 2026-07-16

This local register tracks PII and privacy observations found outside the
original remediation of the four critical regressions:

1. top-level customer classification;
2. the mandatory Inbox presentation boundary;
3. safe HTTP diagnostics;
4. Unicode-safe name replacement.

Completed follow-ups remain here as historical context; unresolved items stay
available for later planning.

## Tracking summary

| ID | Priority | Status | Observation | Suggested follow-up |
| --- | --- | --- | --- | --- |
| PII-F01 | High | Open | One person can receive different pseudonyms when response shapes expose different identifiers. | Design and version a stable identity-key strategy. |
| PII-F02 | High | Complete | An empty HS_INBOX_PII_SECRET uses unkeyed SHA-256. | Resolved with a generated/keyring-backed private secret. |
| PII-F03 | High | Complete | Invalid PII modes silently normalize to off. | Resolved with strict parsing and preflight validation. |
| PII-F04 | High | Complete | Windows can install a model bundle but cannot run the bundled NER runtime. | Windows is now refused until a real runtime smoke test exists. |
| PII-F05 | High | Complete | Download hashes are generated after download and are not trust anchors. | Resolved with an embedded pinned manifest and source lock. |
| PII-F06 | Medium | Complete | Model installation is not atomic and lacks explicit size/time limits. | Resolved with the bounded transactional installer. |
| PII-F07 | Medium | Complete | MCP and CLI arguments can expose PII through argv, shell history, and echoed errors. | Resolved with protected stdin/private-file envelopes and safe MCP errors. |
| PII-F08 | Medium | Complete | Fake display names come from small lists and can collide visually. | Resolved with deterministic keyed display disambiguation. |
| PII-F09 | Medium | Complete | Identity canonicalization does not normalize Unicode composition. | Resolved by versioned Unicode canonicalization schema v2. |
| PII-F10 | Medium | Complete | Secret rotation changes every displayed identity with no migration/version marker. | Resolved with explicit key IDs and locked legacy-record migration. |
| PII-F11 | Medium | Complete | There is no measurable multilingual detection-quality corpus. | Resolved with hermetic and real-model corpus evaluators. |
| PII-F12 | Medium | Open | Disabled low-level identity methods still pseudonymize when called directly. | Enforce mode at each public identity-redaction boundary. |
| PII-F13 | Medium | Open | The compatibility-preserved generated-email namespace can collide at scale. | Version any email-format expansion and define migration expectations. |

## PII-F01 — Identity keys vary across payload shapes

Current identity precedence is:

1. canonical email;
2. canonical full name;
3. phone-only fallback in free-text handling.

The same Help Scout customer can therefore receive different display
pseudonyms when one response includes an email and another includes only a name
or phone. The four critical fixes must preserve the current deterministic
contract and must not silently change this precedence.

A future design could use the stable Help Scout customer ID as the primary key,
with email/name aliases linked to it. That requires an explicit key-schema
version and compatibility plan because existing pseudonyms would otherwise
change.

Questions to resolve:

- Should IDs be scoped by Help Scout account/mailbox?
- Should aliases persist locally, or be derived statelessly?
- How should records without an ID reconcile with records that later include
  one?
- Is backwards-compatible display required after an upgrade?

## PII-F02 — Empty secrets are deterministic but not private — complete

Resolved in `0cfa643`. Enabled engines can no longer use unkeyed SHA-256. An
explicit secret remains supported for intentional cross-machine consistency;
otherwise a random installation secret is initialized atomically in the OS
keyring. Rotation/version policy remains separately tracked in PII-F10.

## PII-F03 — Invalid mode values fail open — complete

Resolved in `fce500b`. Modes use strict parsing and invalid file/environment
values stop Inbox and MCP work before API access. File-only configuration
loading preserves a safe repair path.

## PII-F04 — Windows model/runtime mismatch — complete

Resolved in `f304833` and `f4416c3`. Capability is explicit; unsupported
Windows installs fail before network/cache mutation, status is truthful, and
free-form content remains fail-closed. Windows can be reintroduced only after
a real loader/model smoke job passes.

## PII-F05 — Model provenance is not independently verified — complete

Resolved in `f4416c3`. The binary embeds independently reviewed release and
inner-file identities, while the release source lock pins upstream inputs.
Downloaded sidecars remain useful operator artifacts but are not trusted by
the installer.

## PII-F06 — Model installation hardening — complete

Resolved in `f4416c3`. The installer is context-aware, size/hash bounded,
strictly allowlisted, runtime-smoke-tested, staged privately, content-addressed,
and atomically promoted. Failures preserve any prior trusted installation and
stale installer-owned staging is cleaned within a bounded policy.

## PII-F07 — Sensitive values in argv and MCP subprocesses — complete

CLI flags can contain customer emails, search terms, message bodies, and custom
field values. Shell history and process listings can retain those values.

The MCP runner also converts tool arguments into child-process argv and includes
the reconstructed command line in failures. A future application-service API
would let MCP invoke use cases directly instead of shelling out through the CLI.

Resolved with a strict, bounded protected-input envelope. Annotated direct CLI
flags require stdin or private regular files, MCP supplies the envelope
automatically, and safe error displays never reconstruct protected values. The
focused regression also proves raw direct input stops before API work.

## PII-F08 — Visual pseudonym collisions — complete

Fake first and last names come from small fixed lists. Different identities can
display the same full name, especially in table views that omit the generated
email suffix.

A short HMAC-derived display suffix plus public key ID now disambiguates the
former deterministic `Casey Stewart` collision without exposing source IDs.

## PII-F09 — Unicode normalization of identity keys — complete

Canonicalization lowercases and trims but does not normalize Unicode. Visually
identical NFC and NFD names can therefore hash to different identities. Broader
case-folding and whitespace semantics also need definition.

Schema v2 defines NFC, Unicode case folding, NFC recomposition, and Unicode
whitespace collapse for names; emails add NFC while retaining established
trim/lowercase behavior. Compatibility fixtures pin legacy canonical outputs.

## PII-F10 — Secret rotation and pseudonym versioning — complete

Changing HS_INBOX_PII_SECRET intentionally changes every token and identity, but
output has no key-version marker and there is no rotation workflow.

Every enabled engine now receives a versioned pseudonym context with explicit
public key ID. Keyring records migrate atomically while preserving legacy
secret bytes. Explicit operators set and rotate the secret and ID together;
the current stateless design retains no old key or alias cache to migrate.

## PII-F11 — Detection quality and observability — complete

There is no maintained multilingual privacy corpus measuring false negatives
and false positives across names, addresses, identifiers, HTML, custom fields,
and long chunked content.

Build a synthetic, non-production corpus tracking:

- recall for privacy-sensitive entities;
- over-redaction of operational fields and staff identities;
- per-language/script behavior;
- behavior with and without the NER model;
- performance and memory limits for large conversations and attachments.

The checked-in corpus is entirely synthetic. Hermetic tests cover policy and
redaction mechanics; the real model release smoke measures full expected-name
coverage, unexpected person spans, chunked long content, and runtime budget.

## PII-F12 — Disabled low-level identity methods bypass the mode invariant

`RedactJSONWithContext` and `RedactText` return input unchanged when the engine
is disabled, but `RedactPerson`, `RedactEmail`, and `RedactPhone` do not check
the mode themselves. Supported command paths currently enter through guarded
presentation methods, so this is not a known CLI leak. It is nevertheless a
fragile public API contract: a future direct caller can pseudonymize with the
zero context accepted by `NewEngine(ModeOff, ...)`.

Add direct `ModeOff` pass-through tests for all three methods, then put the
guard at those lowest public boundaries. Keep the higher-level guards as cheap
early exits.

## PII-F13 — Legacy generated-email namespace can collide at scale

The issue-11 fix adds a 40-bit keyed disambiguator and public key ID to display
names while intentionally preserving the established generated email format.
That format has 50 first-name choices, 50 last-name choices, and a 16-bit
suffix: roughly 164 million possible values, so birthday collisions become
plausible at tens of thousands of identities.

This is not a regression in `v2`, and the strengthened full name remains
deterministic and visually distinct. A future display schema should consider a
longer email suffix or key-ID component, with compatibility fixtures and an
explicit decision about whether already displayed pseudonym emails may change.
