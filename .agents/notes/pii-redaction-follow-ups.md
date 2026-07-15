# PII Redaction — Deferred Observations

Last reviewed: 2026-07-15

This local register tracks PII and privacy observations outside the agreed
remediation of the four critical regressions:

1. top-level customer classification;
2. the mandatory Inbox presentation boundary;
3. safe HTTP diagnostics;
4. Unicode-safe name replacement.

Items needed to implement those four safely—fail-closed presentation,
custom-field handling, opaque attachment suppression, NER error handling, and
invocation-scoped detector cleanup—remain active scope and are not duplicated
below.

## Tracking summary

| ID | Priority | Observation | Suggested follow-up |
| --- | --- | --- | --- |
| PII-F01 | High | One person can receive different pseudonyms when response shapes expose different identifiers. | Design and version a stable identity-key strategy. |
| PII-F02 | High | An empty HS_INBOX_PII_SECRET uses unkeyed SHA-256. | Generate/store or require a private local secret. |
| PII-F03 | High | Invalid PII modes silently normalize to off. | Reject invalid configuration at startup. |
| PII-F04 | High | Windows can install a model bundle but cannot run the bundled NER runtime. | Implement Windows runtime support or block installation clearly. |
| PII-F05 | High | Download hashes are generated after download and are not trust anchors. | Publish and verify a signed or pinned manifest. |
| PII-F06 | Medium | Model installation is not atomic and lacks explicit size/time limits. | Stage, validate, and atomically promote bundles. |
| PII-F07 | Medium | MCP and CLI arguments can expose PII through argv, shell history, and echoed errors. | Move sensitive inputs off argv and reduce MCP subprocess coupling. |
| PII-F08 | Medium | Fake display names come from small lists and can collide visually. | Add deterministic display disambiguation. |
| PII-F09 | Medium | Identity canonicalization does not normalize Unicode composition. | Define a versioned Unicode normalization policy. |
| PII-F10 | Medium | Secret rotation changes every displayed identity with no migration/version marker. | Add key versioning and a rotation policy. |
| PII-F11 | Medium | There is no measurable multilingual detection-quality corpus. | Build a privacy-focused evaluation suite. |

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

## PII-F02 — Empty secrets are deterministic but not private

When HS_INBOX_PII_SECRET is empty, identity and token derivation uses plain
SHA-256. That provides stable output but permits cross-installation correlation
and offline guessing of low-entropy names, phone numbers, and common emails.

Options:

- generate a random installation secret and store it in the OS keyring;
- require an explicitly configured secret whenever PII mode is enabled;
- support an organization-managed secret for intentional cross-machine
  consistency.

Coordinate this with PII-F10 so rotation and key versioning are defined before
changing defaults.

## PII-F03 — Invalid mode values fail open

NormalizeMode maps unknown values to off. A typo such as customer instead of
customers therefore disables redaction without an error.

Parse configuration into a strong mode type and reject unknown values before
creating clients or running commands. Validate configuration before persisting
it. Cover config files, environment variables, case/whitespace, and MCP child
processes.

## PII-F04 — Windows model/runtime mismatch

Model cache paths and bundle selection support Windows and onnxruntime.dll, but
the runtime implementation is compiled only for Darwin, Linux, FreeBSD, and
NetBSD. Windows receives the unsupported runtime stub.

The model installer can therefore succeed and report the model ready while
NewDetector can never run. Either implement and test Windows inference, or block
installation and clearly document the fail-closed behavior.

## PII-F05 — Model provenance is not independently verified

The installer downloads a GitHub release tarball and calculates SHA-256 while
extracting. The resulting .sha256 files only describe the bytes just received;
they are not compared with an independently trusted digest or signature.

Follow-up:

- ship a pinned manifest with expected names, sizes, and SHA-256 values;
- sign the manifest or release and verify it before extraction;
- reject unexpected and duplicate archive members;
- verify every file before marking the model ready;
- retain the existing path-traversal tests.

## PII-F06 — Model installation hardening

Extraction writes directly into the live cache. Interrupted or corrupt
downloads can leave partial files, and networking uses the default HTTP client
without an explicit timeout or total download limit.

Install into a private temporary directory, enforce compressed/extracted size
limits, validate all files, then atomically rename the completed directory into
place. Clean stale staging directories on the next attempt.

## PII-F07 — Sensitive values in argv and MCP subprocesses

CLI flags can contain customer emails, search terms, message bodies, and custom
field values. Shell history and process listings can retain those values.

The MCP runner also converts tool arguments into child-process argv and includes
the reconstructed command line in failures. A future application-service API
would let MCP invoke use cases directly instead of shelling out through the CLI.

Shorter-term mitigations:

- accept sensitive bodies through stdin or protected temporary files;
- mark sensitive tool arguments and omit them from reconstructed errors;
- never include raw argument values in diagnostic command lines.

## PII-F08 — Visual pseudonym collisions

Fake first and last names come from small fixed lists. Different identities can
display the same full name, especially in table views that omit the generated
email suffix.

Consider a short deterministic display suffix or another collision-resolution
strategy that remains readable and reveals no source identifier.

## PII-F09 — Unicode normalization of identity keys

Canonicalization lowercases and trims but does not normalize Unicode. Visually
identical NFC and NFD names can therefore hash to different identities. Broader
case-folding and whitespace semantics also need definition.

Any normalization change affects deterministic output and should use a
versioned key schema with compatibility fixtures.

## PII-F10 — Secret rotation and pseudonym versioning

Changing HS_INBOX_PII_SECRET intentionally changes every token and identity, but
output has no key-version marker and there is no rotation workflow.

Define:

- a key identifier/version;
- whether old keys remain available for historical correlation;
- how operators intentionally reset all pseudonyms;
- how future cached aliases are migrated or discarded.

## PII-F11 — Detection quality and observability

There is no maintained multilingual privacy corpus measuring false negatives
and false positives across names, addresses, identifiers, HTML, custom fields,
and long chunked content.

Build a synthetic, non-production corpus tracking:

- recall for privacy-sensitive entities;
- over-redaction of operational fields and staff identities;
- per-language/script behavior;
- behavior with and without the NER model;
- performance and memory limits for large conversations and attachments.

Never log raw production samples to build this corpus.
