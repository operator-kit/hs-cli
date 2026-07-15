# PII Redaction — Deferred Observations

Last reviewed: 2026-07-15

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
| PII-F07 | Medium | Open | MCP and CLI arguments can expose PII through argv, shell history, and echoed errors. | Move sensitive inputs off argv and reduce MCP subprocess coupling. |
| PII-F08 | Medium | Open | Fake display names come from small lists and can collide visually. | Add deterministic display disambiguation. |
| PII-F09 | Medium | Open | Identity canonicalization does not normalize Unicode composition. | Define a versioned Unicode normalization policy. |
| PII-F10 | Medium | Open | Secret rotation changes every displayed identity with no migration/version marker. | Add key versioning and a rotation policy. |
| PII-F11 | Medium | Open | There is no measurable multilingual detection-quality corpus. | Build a privacy-focused evaluation suite. |

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
