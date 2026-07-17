# OpenAI Privacy Filter Single-Detector Migration Plan

- Status: approved direction; implementation not started
- Plan date: 17 July 2026
- Decision owner: hs-cli maintainers
- Companion research: [OpenAI Privacy Filter evaluation](openai-privacy-filter-evaluation.md)
- Governing safety contract: [PII redaction hardening contract](pii-redaction-hardening-contract.md)

## Executive decision

The intended production end state is one local neural detector per process.
OpenAI Privacy Filter will replace the existing multilingual DistilBERT name
detector only if every gate in this plan passes. It will not run beside
DistilBERT in the shipped runtime.

The rest of the PII system remains layered and authoritative:

- structured resource classification;
- known customer and staff identity reconciliation;
- deterministic HMAC-backed display identities and typed tokens;
- exact regex rules for formats with deterministic coverage;
- protected-input and diagnostic boundaries;
- mode enforcement; and
- fail-closed output handling.

Privacy Filter is a local bidirectional token classifier, not a hosted API and
not a generative LLM. The migration therefore changes the residual span
detector, not the product's privacy policy or identity system.

The highest-value new behavior is secret detection. In every redacting display
mode, a detected password, API key, access token, one-time password, PIN, or
similar credential will be replaced with a non-linkable secret marker. Secrets
will never receive deterministic pseudonyms and will never be preserved merely
because they appear next to a known staff identity. Persistent diagnostics
remain independently sanitized under the existing strongest-mode contract.

Development comparison is permitted only in isolated evaluation processes over
checked-in synthetic fixtures. The two models may run sequentially to produce
comparable results, but the production process must construct and retain
exactly one selected detector.

## Outcome this plan must prove

The migration is successful only when the repository contains durable evidence
that the candidate:

1. Preserves all existing PII hardening invariants and deterministic identity
   output.
2. Removes all critical synthetic secrets from every protected output boundary.
3. Adds useful coverage for contextual account identifiers, private dates, and
   messy or obfuscated PII.
4. Uses Privacy Filter's typed spans without allowing model labels to own
   application policy.
5. Matches the pinned reference tokenizer, label mapping, calibrated BIOES
   decoder, and UTF-8 source offsets.
6. Runs within agreed CPU, memory, startup, bundle-size, and long-input budgets
   on every advertised native platform.
7. Loads one detector once per process and never silently falls back to a
   different detector.
8. Can be installed, verified, rolled back, and eventually retired using the
   existing trusted model lifecycle.

If any hard safety gate fails, the production default remains DistilBERT. A
larger-model or enhanced profile may be considered later, but it must still
select one detector rather than constructing an ensemble.

## Normative language and evidence rules

The words **must**, **must not**, **required**, and **blocking** define release
conditions. **Should** describes a preferred implementation that may change
through normal review without weakening a gate.

Gate evidence must be:

- generated from synthetic, repository-owned inputs;
- reproducible from an immutable source and model revision;
- attributable to a git commit, corpus hash, model hash, runtime version,
  platform, and hardware profile;
- produced by checked-in test or benchmark code;
- retained as normal regression coverage or as a release CI artifact; and
- free from Help Scout, production, developer-machine, or real credential data.

Published upstream benchmark scores are background evidence only. They do not
satisfy an hs-cli gate. A benchmark result without its input profile, warm/cold
state, thread count, device, runtime, and hardware identity is informational
and cannot approve a phase.

No threshold may be relaxed after seeing a failing candidate result without a
separate reviewed commit explaining the product trade-off. The changed
threshold then applies only to a fresh evaluation run.

## Scope

### In scope

- Replacing the DistilBERT residual name detector with Privacy Filter.
- Adding backend-neutral typed span contracts.
- Handling all eight upstream categories through hs-cli policy.
- Giving secret the strongest non-linkable replacement policy.
- Preserving deterministic identities for known people and appropriate unknown
  people.
- Building a pinned Docker reference/oracle environment.
- Implementing and verifying a native Go/ONNX adapter.
- Adding permanent quality, security, parity, release, and performance tests.
- Supporting an explicit preview, rollback, default migration, and retirement
  sequence.
- Inventorying and comparing the original safetensors checkpoint and every
  published ONNX export: default, fp16, generic quantized, q4, and q4f16.
- Packaging exactly one approved model variant in each production bundle.

### Out of scope for the initial replacement

- Sending unredacted text to any hosted model or API.
- Running DistilBERT and Privacy Filter as a production ensemble.
- Replacing exact regex, structured JSON, identity, mode, or diagnostic policy.
- Building a cross-conversation identity graph for unknown third parties.
- Treating model output as a compliance guarantee.
- Training on Help Scout or production conversations.
- Requiring Python or Docker to run the normal hs binary.
- Advertising Windows inference before a real native runtime smoke passes.

Optional domain adaptation is a later phase in this plan. It cannot be used to
make the initial replacement look successful on a test set that also influenced
training.

## Capability benefit traceability

| Expected benefit | Evidence that proves it | Blocking gate |
|---|---|---|
| New high-risk coverage, especially secrets, account identifiers, and private dates | Typed detector and final-output corpus | G4 and G5 |
| Better handling of messy, contextual, and obfuscated PII | Adversarial, formatting, multilingual, and preservation slices | G4 |
| Category-aware product policy | Mode/action matrix and backend-neutral SpanKind contract | G0, G1, and G5 |
| Ability to adapt open weights to the support domain | Frozen synthetic train/holdout branch that repeats all affected gates | Optional adaptation branch |

The initial migration must prove the first three benefits without fine-tuning.
Adaptation is an option, not a way to bypass a weak zero-shot result.

## Immutable product policy

These decisions must be frozen before adapter work so model behavior cannot
silently redefine the product.

### Modes

| Situation | Required behavior |
|---|---|
| ModeOff display redaction | Exact pass-through; do not initialize or call a detector |
| ModeCustomers known customer | Preserve current deterministic customer pseudonym behavior |
| ModeCustomers known staff person span | Preserve staff identity as the current contract requires |
| ModeCustomers unknown third party | Redact as an unknown private person |
| ModeCustomers secret | Always redact; attribution never protects a credential |
| ModeAll private span | Redact according to its typed policy |
| Persistent diagnostics | Sanitize independently of display mode and --unredacted |

Mode-off pass-through does not weaken the independent diagnostic contract.
Debug persistence continues to invoke its strongest sanitization boundary.

### Typed replacement policy

| Domain span | Required initial replacement |
|---|---|
| Known person | Existing deterministic identity derived from the known identity key |
| Unknown private person | Existing deterministic fake-person behavior scoped to the source name |
| Secret | Constant non-linkable marker such as [REDACTED_SECRET] |
| Account number | Existing keyed deterministic typed token |
| Private date | Keyed deterministic typed token |
| Address, email, phone, URL | Reconcile with known identity policy, then use existing deterministic typed handling |

The literal marker may be finalized in Phase 0, but secret values must never be
HMAC inputs for a user-visible stable token. Linkability is useful for people
and some identifiers; it is an unnecessary disclosure for credentials.

### Failure behavior

The selected detector is mandatory whenever free text requires ML inspection.
Any of the following must discard partial predictions and hide the affected
free-text field using the established fail-closed notice:

- model unavailable, corrupt, unsupported, or untrusted;
- tokenizer failure or source/token round-trip mismatch;
- incomplete or timed-out window inference;
- invalid class count, label map, calibration, or decoder state;
- invalid, reversed, overlapping-invalid, out-of-range, or non-UTF-8-boundary
  source span;
- panic or native runtime error; or
- selected-backend initialization failure.

The runtime must not silently retry with DistilBERT. An operator may explicitly
select or roll back to a trusted DistilBERT bundle in a later invocation, but a
coverage-changing fallback is never automatic.

## Target architecture

The target execution path is:

    Help Scout payload
        |
        +-- classify resource and collect known identities
        |
        +-- original free text
              |
              +-- selected SpanDetector (exactly one process-owned instance)
              |
              +-- normalize and validate UTF-8 byte spans
              |
              +-- reconcile mode, known identities, and typed policy
              |
              +-- render deterministic identities / typed tokens / secret marker
              |
              +-- deterministic regex defence-in-depth sweep
              |
              +-- fail-closed presentation boundary

Detection always sees the unmodified original text. Policy and rendering happen
after source spans are validated. The model adapter maps upstream labels into
domain-owned kinds; no OpenAI label string should escape the adapter package.

The intended interface is conceptually:

    type SpanKind string

    type DetectedSpan struct {
        Kind  SpanKind
        Start int // UTF-8 byte offset in the original input
        End   int // exclusive UTF-8 byte offset in the original input
        Score float32
    }

    type SpanDetector interface {
        Detect(context.Context, string) ([]DetectedSpan, error)
    }

The concrete API may evolve during review. Its non-negotiable properties are
explicit byte offsets, typed spans, cancellation, complete-result semantics,
and no dependency from internal/pii onto upstream model packages.

## Test and evidence topology

The migration requires three deliberately separate test lanes.

### Lane A — hermetic tests on every pull request

These tests must run under the existing go test ./... job without network,
Docker, Python, or model weights. They use:

- fake detector outputs;
- checked-in synthetic policy fixtures;
- canned logits for decoder tests;
- checked-in oracle span goldens;
- corrupt and boundary-case fixtures; and
- deterministic performance-independent contract assertions.

This lane owns policy, rendering, offset, fail-closed, mode, identity,
serialization, and command-boundary regressions.

### Lane B — real-model quality and release smoke

This lane runs in a repo-owned Docker reference environment and on each native
supported platform. It uses pinned model artifacts supplied through the trusted
installer or CI cache. It must never download a moving main revision.

It owns:

- official Python reference predictions;
- original, default ONNX, fp16, generic-quantized, q4, and q4f16 comparison;
- native ONNX parity;
- real tokenizer and decoder behavior;
- multilingual and adversarial corpus evaluation; and
- real bundle installation and inference smoke.

It should run on workflow dispatch, model-related pull requests, a scheduled
cadence, and every PII model release tag. Normal unit jobs continue to skip real
inference when no trusted model directory is supplied.

### Lane C — controlled performance regression

Performance gates must run on named, stable hardware profiles. Shared hosted
runner results may be recorded for trend information but must not be the only
blocking evidence because host contention makes them noisy.

The benchmark harness, workloads, budgets, and result schema remain checked in.
Individual reports are retained as CI artifacts and the approved release
baseline is committed as a small metadata record. Raw model files and large
reports are not committed.

### Sequential comparison rule

When a workflow compares backends, it must start each backend in a separate
process, record its result, terminate it, and then run the other backend. This
proves comparable behavior without hiding accidental ensemble initialization or
inflating memory measurements.

## Corpus design

The existing multilingual corpus will be retained for compatibility and
expanded through a versioned typed corpus rather than silently changing schema
1 expectations.

Proposed repository layout:

    internal/pii/testdata/privacy-filter/v1/
        policy.json
        secrets.json
        contextual-identifiers.json
        multilingual.json
        adversarial.json
        preservation.json
        long-input.json
        command-payloads/
        oracle/
            reference-spans.json
            onnx-default-spans.json
            fp16-spans.json
            generic-quantized-spans.json
            q4-spans.json
            q4f16-spans.json
        performance/
            workloads.json
            budgets.json
            approved-baseline.json

Every case must include:

- stable case ID and corpus schema;
- language, script, content shape, and risk tier;
- exact original text made solely from synthetic values;
- expected UTF-8 byte spans and domain kind;
- expected action for off, customers, and all where relevant;
- known customer/staff identity context;
- strings that must be absent from final output;
- strings that must remain present;
- whether exact-span or covering-span matching is required; and
- a short reason for the privacy or preservation expectation.

Fixture loaders must reject duplicate IDs, unknown fields, invalid UTF-8,
invalid byte boundaries, overlapping contradictory expectations, and expected
values absent from the source.

### Secret fixture families

The secret corpus is the highest-priority gate and must include safe,
non-functional examples of:

- API keys and access tokens;
- OAuth bearer and refresh tokens;
- passwords in prose, JSON, YAML, shell-like assignments, and URLs;
- JWT-shaped values;
- one-time passwords, recovery codes, PINs, and temporary passwords;
- private-key and certificate-key blocks;
- database connection strings with embedded credentials;
- cookies, authorization headers, and webhook signing secrets;
- cloud, source-control, payment, and observability credential shapes;
- values split by punctuation, whitespace, Markdown, HTML, or line wrapping;
- quoted replies, stack traces, logs, and code blocks;
- a developer's secret mentioned by a customer; and
- repeated and overlapping secret candidates.

Preservation fixtures must contain hashes, public IDs, package versions,
example placeholders, redacted markers, documentation literals, checksums, and
operational numbers that should not be classified as secrets.

No fixture may be a live credential. Synthetic values should use reserved test
domains and deliberately non-routable/non-authentic material while retaining
enough surrounding context to exercise contextual detection. Repository secret
scanning must remain enabled.

### Other new coverage

The typed corpus must also cover:

- generic customer, project, subscription, government, and financial account
  identifiers that current regexes do not recognize;
- birth dates, appointment dates, and other private dates;
- release dates and public event dates that should survive;
- obfuscated email and phone forms;
- incomplete and international addresses;
- usernames, handles, nicknames, and third parties such as a developer copied
  into a customer conversation;
- public people, company names, products, cities, and business addresses;
- all scripts already protected by Contract 14; and
- long content with private context near and far from the value.

## Gate summary

| Gate | Decision | Blocking evidence |
|---|---|---|
| G0 Policy freeze | We know what each typed span means | Reviewed corpus schema, mode matrix, secret policy, fixed budgets |
| G1 Behavior-preserving seam | The architecture can swap detectors safely | Existing suite, generic detector contract, lifecycle and exact-output tests |
| G2 Reference reproducibility | The upstream behavior is pinned and repeatable | Hashed Docker oracle, immutable revisions, deterministic golden generation |
| G3 Native parity | Go/ONNX implements the intended model contract | Tokenizer, logits, BIOES/Viterbi, offset, and per-variant parity tests |
| G4 Privacy quality | The candidate improves protection without unacceptable loss | Locked detector and end-to-end quality corpus |
| G5 Secret safety | Credentials cannot escape protected boundaries | Critical secret corpus, output/log/error/argv tests, failure injection |
| G6 Performance and footprint | The replacement fits the product | Controlled cold/warm latency, RSS, size, concurrency and longevity tests |
| G7 Platform and supply chain | Every advertised bundle is executable and trusted | Native smoke, manifest, installer, rollback and release workflow tests |
| G8 Preview readiness | Opt-in use is safe and observable | Backend selection, command matrix, one-detector and fail-closed tests |
| G9 Preview exit | The opt-in backend is ready to become the default | Stable preview evidence and rollback drill |
| G10 Default and retirement | DistilBERT can stop being the default | Default, upgrade, rollback-window, and cache tests |

Every gate has only three states: not-run, pass, or fail. An invalid or
incomplete benchmark is not-run, never pass.

## Phase 0 — freeze policy, corpus, metrics, and budgets

### Goal

Create the test contract before implementation results can influence it.

### Entry criteria

- The research evaluation is reviewed.
- The one-detector replacement direction is accepted.
- The existing hardening contract is green on main.

### Work

1. Add domain kinds for the eight Privacy Filter categories to the test schema.
2. Record the mode and replacement matrices in executable fixtures.
3. Add the typed corpus partitions described above.
4. Add a schema validator and metric calculator.
5. Run the current DistilBERT-plus-regex pipeline to produce a baseline report.
6. Freeze proposed performance workloads and budgets before running Privacy
   Filter.
7. Record exact existing deterministic outputs for known identities.

### Permanent tests

- TestPrivacyCorpusSchemaIsStrictAndSelfConsistent
- TestPrivacyCorpusContainsRequiredRiskLanguageAndFormatSlices
- TestSecretFixturesContainOnlyDeclaredSyntheticMaterial
- TestTypedPolicyMatrixCoversEveryModeAndSpanKind
- TestExistingPIIContractAgainstTypedCorpus
- existing critical, high, medium, and follow-up regression tests

### G0 pass criteria

- Every fixture validates and every required slice has at least one redact and
  one preservation case.
- Every secret family has a must-detect case and a near-miss preservation case.
- Existing deterministic known-identity outputs are recorded byte-for-byte.
- All existing PII and repository tests pass unchanged.
- Initial performance budgets below are explicitly accepted or replaced in a
  reviewed commit before candidate measurement.
- Corpus and budget files have stable hashes in the baseline report.

### Failure handling

Missing or disputed policy is a design failure. Resolve the policy and fixtures;
do not begin model integration. A failure of the current pipeline is handled as
an independent regression before the candidate is evaluated.

## Phase 1 — introduce a behavior-preserving detector seam

### Goal

Generalize the domain interface and detector lifecycle while retaining
DistilBERT as the only production backend and preserving output exactly.

### Work

1. Introduce SpanDetector, DetectedSpan, and domain-owned SpanKind types.
2. Adapt DistilBERT to emit SpanPerson through that interface.
3. Separate span validation, policy reconciliation, and rendering from model
   inference.
4. Introduce a process-scoped detector provider that lazy-loads once, closes at
   shutdown, and does nothing in ModeOff.
5. Add context cancellation and a panic/error boundary without permitting
   partial output.
6. Preserve the current engine API until callers can migrate safely.

### Permanent tests

- TestSpanDetectorContractReturnsCompleteOriginalByteSpans
- TestDistilBERTAdapterPreservesCurrentNameBehavior
- TestSpanPolicyPreservesKnownCustomerIdentityOutput
- TestSpanPolicyProtectsKnownStaffOnlyWherePolicyAllows
- TestUnknownThirdPartyGetsDeterministicDisplayIdentity
- TestModeOffDoesNotConstructOrCallDetector
- TestDetectorProviderLoadsOneInstanceAcrossMultipleEngines
- TestDetectorProviderClosesExactlyOnce
- TestDetectorErrorPanicAndCancellationFailClosed
- TestInvalidTypedSpansFailClosed
- existing command, JSON, pseudonym, multilingual, race, and release tests

### G1 pass criteria

- go test ./..., go vet ./..., and repository race jobs pass.
- All existing PII outputs are byte-for-byte unchanged for the locked corpus.
- A display-only ModeOff invocation with persistent diagnostics disabled
  constructs zero detectors and changes zero bytes.
- Ten or more engines/requests in one process construct one detector.
- Error, cancellation, and panic injection reveal no portion of an uninspected
  free-text field.
- On the controlled baseline host, the refactor alone changes warm p95 latency
  by no more than 10% and steady RSS by no more than 5% or 16 MiB, whichever is
  greater.

### Failure handling

This phase cannot be waived as a model trade-off because it is intended to be
behavior preserving. Fix the abstraction or lifecycle regression before
proceeding.

## Phase 2 — build the pinned reference oracle

### Goal

Make upstream behavior reproducible without adding Python or Docker to the
normal product runtime.

### Work

1. Add a repo-owned Dockerfile and lock file for the official Python runtime.
2. Pin the reviewed OpenAI source, model, tokenizer, calibration, and ONNX
   revisions by immutable identity and SHA-256.
3. Run the original safetensors checkpoint and the default, fp16, generic-
   quantized, q4, and q4f16 ONNX exports in separate processes.
4. Emit structured labels, character spans, scores, token IDs, offsets, and
   decoder metadata using a strict repository-owned schema.
5. Convert reference character offsets to explicit fixture byte offsets only in
   a tested boundary layer.
6. Add an intentional golden-update command that refuses a dirty tree and
   records every source hash.
7. Disable network access after the verified image/model preparation step.

The initial spike starts from the identities reviewed in the companion
evaluation: OpenAI source commit
f7f00ca7fb869683eb732c010299d901457f19c3, model repository commit
7ffa9a043d54d1be65afb281eddf0ffbe629385b, and its recorded ONNX export commit
454bcac971532e0fa5863b7043d04d5bb573cd0e. Advancing any identity requires a
reviewed source-lock change and makes downstream goldens and gate results stale.

Proposed paths:

    build/privacy-filter/Dockerfile
    build/privacy-filter/compose.yml
    build/privacy-filter/requirements.lock
    scripts/privacy-filter-eval.sh
    internal/pii/testdata/privacy-filter/v1/oracle/

### Permanent tests

- TestReferenceOutputSchemaRejectsUnknownOrIncompleteData
- TestReferenceArtifactsMatchPinnedHashes
- TestReferenceOffsetsRoundTripEveryScript
- TestGoldenMetadataPinsModelTokenizerCalibrationAndSource
- Docker oracle smoke over a minimal synthetic category set
- reproducibility test that two clean runs produce identical structured spans

### G2 pass criteria

- A clean machine can reproduce the oracle goldens from reviewed immutable
  inputs.
- Two runs for each variant produce identical typed spans and offsets.
- The image refuses an unexpected artifact, hash, class count, or calibration
  schema.
- No network call occurs during inference or golden comparison.
- No non-synthetic input path is accepted by the standard evaluation command.
- Every published variant has a distinct report identifying its artifact,
  footprint, runtime, quality metrics, and intended disposition; no variant
  inherits another variant's score.

### Failure handling

A non-reproducible or moving oracle invalidates downstream evidence. Pin or fix
the harness and regenerate results. Do not hand-edit span goldens.

## Phase 3 — implement the native Privacy Filter adapter

### Goal

Produce a Go/ONNX adapter that matches the pinned reference semantics and can be
selected without changing domain policy.

### Work

1. Validate ONNX Runtime operator and external-data support on every target.
2. Implement or adopt the exact tokenizer with source-offset preservation.
3. Validate the 33-class label map.
4. Implement BIOES constrained Viterbi decoding and all pinned calibration
   biases.
5. Normalize reference character offsets to UTF-8 byte ranges.
6. Merge windows without duplicate, fragmented, lost, or silently truncated
   spans.
7. Map upstream categories into domain-owned SpanKind values.
8. Make inference cancellation, serialization, and lifecycle explicit.
9. Keep every published ONNX variant independently selectable in the evaluation
   harness until its disposition is recorded. Only variants that pass the
   distribution budget proceed as release candidates.

### Permanent tests

- TestTokenizerMatchesReferenceIDsTokensAndOffsets
- TestTokenizerRoundTripsAccentsCombiningMarksEmojiArabicAndHan
- TestViterbiDecoderMatchesCannedReferenceLogits
- TestViterbiRejectsIllegalBIOESTransitionsAndWrongClassCount
- TestCalibrationIsRequiredAndPinned
- TestPrivacyFilterAdapterMapsAllEightKinds
- TestPrivacyFilterAdapterMatchesReferenceSpans
- TestEveryPublishedVariantHasAnIndependentEvaluation
- TestPublishedVariantInventoryMatchesPinnedSourceLock
- TestVariantResultsCannotInheritAnotherArtifactIdentity
- TestQ4AndQ4F16AreMeasuredIndependently
- TestWindowMergingPreservesBoundarySpans
- TestLongInputIsNeverSilentlyTruncated
- TestNativeErrorOrPartialWindowFailsClosed
- FuzzDetectedSpanByteBoundaries
- real-bundle native smoke for every supported target

### G3 pass criteria

- Decoder results match the reference exactly for all canned-logit fixtures.
- Token IDs and source offsets match exactly for the locked corpus.
- The native adapter and same-artifact oracle have 100% typed-span parity on
  critical cases and at least 99.5% exact typed-span parity overall.
- Any remaining non-critical parity difference is reviewed and represented by
  an explicit fixture before the gate can pass.
- Every published variant has a locked artifact-size, load-compatibility,
  quality, and reference-hardware result even if it is later marked
  non-shippable.
- q4 or q4f16 retains 100% critical covering-span recall and loses no more than
  0.5 percentage points of sensitivity-weighted F2 versus the original weights.
- All malformed model, calibration, tokenizer, offset, window, and runtime
  conditions fail closed.
- At least one quantized variant passes real native smoke on every platform that
  will continue to be advertised.

### Failure handling

First determine whether the failure belongs to the adapter, export, runtime, or
quantization. Fix adapter parity defects. The larger exports remain diagnostic
comparators even when they exceed the product size budget. If q4 and q4f16 both
fail parity, platform, quality, or hardware gates, reject the default migration
under the current budgets rather than silently shipping a multi-gigabyte
variant, the Python reference, or a weakened gate. Raising the user hardware or
download requirement needs a separate reviewed product decision followed by a
fresh gate run.

## Phase 4 — prove privacy quality and useful coverage

### Goal

Demonstrate a material privacy improvement without unacceptable over-redaction
or regression in existing protection.

Quality must be measured twice:

1. Detector-only, so regexes cannot conceal a model miss.
2. End-to-end, so the actual rendered CLI boundary is proven safe.

### Metrics

- exact-span precision, recall, F1, and F2;
- covering-span recall for values where complete removal matters more than exact
  boundaries;
- per-kind, risk-tier, language, script, format, and mode metrics;
- sensitivity-weighted false-negative score;
- preservation/over-redaction rate;
- deterministic identity compatibility;
- unknown-third-party handling; and
- final-output sentinel leak count.

### G4 quality pass criteria

| Measure | Required result |
|---|---|
| Existing hardening corpus | Zero regressions |
| Critical end-to-end values | Zero raw-value leaks |
| Existing person/name slices | No per-slice recall regression and complete current contract coverage |
| Secret detector recall | 100% on critical cases; at least 99% on the locked broad secret corpus |
| Secret covering-span recall | 100% on critical cases |
| Account/private-date recall | 100% on critical cases; at least 95% on broad cases |
| Existing email/phone/address/URL final coverage | No regression from the stronger current detector-plus-regex baseline |
| Multilingual/script contract | 100% of mandatory cases and no language slice more than 1 point below baseline |
| Sensitivity-weighted F2 | At least 2 percentage points above the current locked baseline |
| Preservation | At least 99% of preservation cases remain unchanged |
| Over-redaction | No more than 1% absolute and no more than 0.5 points above baseline |
| Deterministic known identities | 100% byte-for-byte compatibility |

The broad corpus must be large enough that one case cannot move a percentage by
more than one point. Until then, percentage gates are not-run rather than
rounded into a pass.

### Permanent tests

- TestPrivacyFilterCriticalCorpusHasZeroDetectorMisses
- TestPrivacyFilterPipelineHasZeroCriticalLeaks
- TestPrivacyFilterSecretBroadCorpusMeetsRecallBudget
- TestPrivacyFilterPreservationCorpusMeetsBudget
- TestPrivacyFilterAccountAndPrivateDatePolicy
- TestPrivacyFilterMessyAndObfuscatedCorpus
- TestPrivacyFilterMultilingualSlicesDoNotRegress
- TestPrivacyFilterKnownIdentityCompatibility
- TestPrivacyFilterUnknownDeveloperIsRedactedWithoutCustomerLinkage
- deterministic metric/gate evaluator unit tests

### Failure handling

- Any critical leak is a hard stop.
- A regression caused by policy or rendering is fixed in the domain layer.
- A detector miss is fixed by a better artifact, safe pre/post-processing, or a
  locked fixture-driven adaptation phase; it is not hidden by deleting the
  case.
- Unacceptable over-redaction requires policy adjustment or candidate rejection.
- Failure limited to one advertised platform or language is still a replacement
  failure unless support is explicitly narrowed before release and the existing
  contract remains truthful.

## Phase 5 — prove secret handling at every boundary

### Goal

Treat secrets as a separate security gate rather than relying on aggregate PII
quality.

### Work

1. Add a domain SpanSecret action that emits a constant, non-linkable marker.
2. Ensure secret rendering occurs before any output, logging, error formatting,
   clipboard/export, MCP, or debug persistence boundary can observe raw text.
3. Ensure a staff identity protection rule cannot protect an adjacent or owned
   secret.
4. Scrub protected input and detector/runtime errors independently of successful
   inference.
5. Add safe counts only if they contain no values, fragments, hashes, or stable
   correlation identifiers.
6. Verify failure paths with secret-bearing synthetic sentinels.

### Permanent tests

- TestSecretUsesConstantNonLinkableReplacement
- TestRepeatedSecretDoesNotCreateStablePublicIdentity
- TestCustomersModeRedactsCustomerStaffAndThirdPartySecrets
- TestModeOffSkipsDisplayDetectorButDiagnosticsStillSanitize
- TestSecretNeverAppearsInTableJSONCSVSourceOrRFC822
- TestSecretNeverAppearsInMCPOutputOrChildArgv
- TestSecretNeverAppearsInDebugLogsErrorsOrPanicRecovery
- TestSecretDetectorFailureHidesEntireFreeTextField
- TestOverlappingSecretAndAccountSpanCannotLeakSuffix
- FuzzSecretSentinelNeverSurvivesRedactingModes

### G5 pass criteria

- Every critical secret is absent from detector output rendering, all command
  formats, MCP responses, argv, stdout, stderr, errors, and persistent debug
  files.
- The complete original secret and every fixture-declared sensitive fragment
  have zero occurrences after redaction.
- Secrets are redacted in customers and all, regardless of attribution.
- Display ModeOff makes no detector call and remains byte-exact, while the
  independent diagnostic sanitizer still removes the secret.
- A detector/runtime failure exposes no partial field and no raw error context.
- Secret near-miss preservation meets the G4 preservation budget.
- Race and fuzz executions find no cross-request value reuse or leakage.

### Failure handling

Any failure blocks preview and release. It cannot be traded for performance,
overall F1, or an allowlist unless the fixture itself is proven to encode the
wrong product policy.

## Phase 6 — prove performance, footprint, and one-model lifecycle

### Goal

Establish that the candidate remains practical for one-shot CLI use and
long-running MCP use without loading two neural models.

### What the larger artifacts imply

The files are unquestionably much larger than the current model, so download,
disk use, cold file reads, and likely resident memory will increase materially.
Inference CPU cost cannot be inferred from file size alone. Privacy Filter has
approximately 1.5 billion total parameters but activates approximately 50
million per token, and the exports use different numeric representations and
operators. Sparse activation may keep warm inference practical even while
startup and memory become more demanding.

The plan therefore treats better-hardware requirements as a hypothesis to
measure, not an assumption to publish. A default migration fails if it needs
more than the frozen minimum profile or exceeds a hard budget. Raising the
minimum hardware requirement is a separate product decision, not an automatic
benchmark adjustment.

### Published variant inventory

The following totals are from the immutable model revision pinned in Phase 2.
Core totals include the checkpoint or ONNX graph and external data, tokenizer,
config, tokenizer config, and Viterbi calibration. They exclude the platform
ONNX Runtime library, currently approximately 18.6 to 39.6 MB, and exclude
archive compression.

| Published variant | Core bytes | Decimal size | Binary size | Planned role |
|---|---:|---:|---:|---|
| Original safetensors | 2,826,861,317 | 2,826.9 MB | 2,695.9 MiB | Accuracy and decoder oracle; never the default user bundle |
| Default ONNX | 1,850,730,517 | 1,850.7 MB | 1,765.0 MiB | Native parity and compatibility comparator |
| ONNX fp16 | 2,103,639,127 | 2,103.6 MB | 2,006.2 MiB | Reduced-precision and accelerator-oriented comparator |
| ONNX generic quantized | 1,646,076,122 | 1,646.1 MB | 1,569.8 MiB | Quantized compatibility comparator |
| ONNX q4 | 945,152,182 | 945.2 MB | 901.4 MiB | Production candidate |
| ONNX q4f16 | 837,099,555 | 837.1 MB | 798.3 MiB | Production candidate and current size leader |

All six variants must receive an artifact, quality, load, memory, and reference-
hardware result. Exceeding the current 1.25 GiB installed-bundle budget marks a
variant non-shippable under this plan but does not remove it from comparison.
Only q4 and q4f16 currently fit that budget before the runtime is added.

Using the current platform runtime sizes, the uncompressed installed payload is
approximately 856 to 877 MB for q4f16 and 964 to 985 MB for q4. The actual
compressed user downloads must be measured from the release bundles; upstream
file totals are not a substitute for that test.

The benchmark environment may cache every variant, but a released platform
bundle contains exactly one approved variant. Users must never download the
entire upstream ONNX directory or multiple variants for normal operation.

### Hardware profiles

Phase 0 must freeze concrete machines matching these provisional profiles:

| Profile | Provisional resources | Purpose |
|---|---|---|
| H0 constrained minimum | 2 vCPU, 4 GiB RAM limit, CPU-only, SSD, swap disabled | Prove the replacement remains safe and usable on modest hardware |
| H1 typical laptop/CI | 4 vCPU, 8 GiB RAM, CPU-only, SSD | Primary absolute and comparative benchmark |
| H2 developer workstation | 8 vCPU, 16 GiB RAM, CPU-only, SSD | Scaling, thread-count, and throughput characterization |

Native compatibility and smoke still run on Linux amd64, Linux arm64, macOS
Intel, and macOS Apple Silicon. At least H0 and H1 must use stable named hosts
rather than an unspecified shared runner. GPU or accelerator measurements are
useful exploratory evidence but cannot qualify a CPU-default release.

Each hardware profile record must pin:

- CPU model, architecture, supported instruction sets, physical/logical cores,
  and virtualization;
- RAM limit, swap policy, NUMA topology where relevant, and container limits;
- storage type and benchmark workspace;
- OS, kernel, power mode/governor, and thermal state;
- ONNX Runtime, Go, and Python/reference versions;
- intra-op/inter-op thread settings and environment variables; and
- git, corpus, budget, and model artifact identities.

If a candidate passes H1 but fails H0, G6 fails under the initial plan. The team
may either optimize/reject it or explicitly propose H1-class hardware as the new
minimum, update documentation and preflight behavior, and rerun every affected
gate from frozen budgets.

### Comparative benchmark method

The current DistilBERT bundle and every Privacy Filter variant run against the
same workloads in isolated processes. At minimum:

1. All variants run quality, load, first-inference, warm subject, warm message,
   and peak-RSS tests on H1.
2. q4 and q4f16 run the full workload, longevity, concurrency, and native
   platform matrix on H0, H1, and H2.
3. A load failure, unsupported operator, out-of-memory termination, or timeout
   is recorded as a variant failure rather than omitted.
4. Fresh-process measurements distinguish a warm OS page cache from an
   intentionally cold/evicted artifact cache.
5. Thread counts of one, the runtime default, and the proposed production cap
   are compared.
6. Quality is rerun whenever a graph, runtime, tokenizer, decoder, thread/window
   strategy, or quantization changes.

Hard gates are applied before any ranking. No weighted score may compensate for
a critical leak, parity failure, unsupported platform, or hardware-budget
failure. Among variants that pass every hard gate, select the smallest and least
resource-intensive variant that meets the user-experience budgets.

### Workload profiles

| Profile | Synthetic input | Primary measure |
|---|---|---|
| Subject | Up to 256 bytes | Warm interactive latency |
| Message | Approximately 2 KiB | Normal command latency |
| Thread | Approximately 20 KiB | Multi-message latency and windowing |
| Export | Approximately 100 KiB | Throughput, timeout, and peak RSS |
| Adversarial long | Dense tokens and awkward boundaries | No truncation and bounded failure |
| MCP longevity | 100 warm requests after warm-up | Reuse, RSS plateau, and race safety |
| MCP burst | Four concurrent 2 KiB requests | Queueing, serialization, and deadlock safety |

Each benchmark records variant identity, raw and compressed artifact size,
fresh-process load time, first inference, warm p50/p95/p99, CPU time, wall time,
tokens and bytes per second, Go heap, process RSS/private bytes, page faults,
disk bytes read, thread count, and detector-construction count.

### Initial acceptance budgets

These are product budgets, not predictions of model performance. Phase 0 may
replace them before candidate measurements. They apply to the agreed reference
CPU and must also pass the documented slowest supported profile unless a
platform-specific budget was frozen in advance.

| Measure | Initial blocking budget |
|---|---|
| Fresh process detector readiness | p95 no more than 5 s reference; no more than 8 s slowest supported target |
| Warm subject | p95 no more than 250 ms |
| Warm 2 KiB message | p95 no more than 750 ms |
| Warm 20 KiB thread | p95 no more than 4 s |
| Warm 100 KiB export | p95 no more than 20 s and no timeout/truncation |
| Peak process RSS | No more than 2.0 GiB on the low-memory profile |
| Post-warm steady RSS | No more than 1.5 GiB |
| RSS growth across 100 requests | No more than 5% or 64 MiB, whichever is greater |
| Installed selected bundle | No more than 1.25 GiB including tokenizer and runtime |
| Compressed download | No more than 1.10 GiB |
| Detector construction | Exactly one when any protected path needs ML; zero for display-only ModeOff with diagnostics disabled |
| Four-request burst | Completes without deadlock within 5x single-request p95 |

Performance sampling must include at least twenty fresh-process runs and one
hundred warm samples per input profile after explicit warm-up. A release gate
uses a clean controlled-host run. One automatic diagnostic rerun is allowed
after host-noise suspicion; conflicting runs leave the gate not-run, not pass.

### Permanent tests and benchmarks

- BenchmarkDetectorColdLoad
- BenchmarkDetectorWarmSubject
- BenchmarkDetectorWarmMessage
- BenchmarkDetectorWarmThread
- BenchmarkDetectorWarmExport
- BenchmarkEveryPublishedVariantOnReferenceHardware
- TestEveryPublishedVariantHasPerformanceAndMemoryEvidence
- TestVariantComparisonRunsInIsolatedProcesses
- TestPerformanceBudgetsSchemaAndHardwareProfile
- TestHardwareProfileRecordsRequiredReproductionMetadata
- TestCandidatePassesConstrainedMinimumHardwareProfile
- TestSelectedBackendConstructsExactlyOneNeuralDetector
- TestDistilBERTFactoryIsNotCalledWhenPrivacyFilterSelected
- TestPrivacyFilterFactoryIsNotCalledWhenDistilBERTSelected
- TestDisplayOnlyModeOffConstructsNoNeuralDetector
- TestDetectorRSSPlateausAcrossWarmRequests
- TestDetectorConcurrentRequestsCompleteAndCloseCleanly

### G6 pass criteria

- Every hard budget passes on H0, H1, H2, and each required native release
  profile.
- Every published variant has a separate reference-hardware report and explicit
  oracle, comparator, candidate, selected, or rejected disposition.
- q4 and q4f16 complete the full matrix and the selected variant is named in
  every result and trusted manifest.
- No benchmark process loads more than one model artifact.
- No user bundle contains more than one model variant.
- Normal command paths load the detector at most once per process.
- No deadlock, race, handle leak, unbounded native allocation, or goroutine
  growth is observed.
- The approved baseline JSON and CI report contain enough metadata to reproduce
  the run.

### Failure handling

Optimize lifecycle, windowing, thread configuration, or evaluate another
published variant, then rerun all quality and performance gates. If q4 and
q4f16 still fail the hard budgets, reject the universal replacement. A
separately selected enhanced profile or a higher minimum hardware proposal is
acceptable future work, but it requires explicit product review and fresh
evidence; loading multiple detectors is not a workaround.

## Phase 7 — integrate preview selection and trusted bundles

### Goal

Make the candidate usable by explicit opt-in while preserving fail-closed and
supply-chain guarantees.

### Work

1. Add a backend-aware trusted manifest containing backend, upstream revision,
   quantization, tokenizer, calibration, decoder schema, runtime, platform,
   file hashes, exact sizes, and defensive bounds.
2. Teach installer and status commands to identify a selected backend and
   verified bundle.
3. Add an explicit preview selection mechanism. Selection controls which one
   detector factory may run.
4. Keep DistilBERT as the production default during preview.
5. Extend transactional download, staging, runtime validation, promotion, and
   rollback tests for large external-data files.
6. Extend every table, JSON, JSON-full, CSV, source, RFC822, MCP, and diagnostic
   path through the generic policy contract.
7. Make an unavailable selected backend fail closed with an actionable status;
   never fall back silently.

### Permanent tests

- TestBackendSelectionLoadsOnlySelectedDetector
- TestSelectedPrivacyFilterUnavailableFailsClosedWithoutFallback
- TestBackendAwareManifestRejectsUnknownSchemaAndArtifacts
- TestPrivacyFilterInstallerEnforcesExternalDataBounds
- TestPrivacyFilterInstallerIsTransactionalAndRollbackSafe
- TestPrivacyFilterStatusReportsExactTrustedIdentity
- TestPrivacyFilterAllOutputFormatsSharePresentationBoundary
- TestPrivacyFilterMCPAndDirectCLIOutputsMatch
- TestPrivacyFilterStructuredIdentityCompatibility
- native release smoke on Linux and macOS amd64/arm64

### G7 and G8 pass criteria

- Every advertised platform installs and executes the exact trusted bundle on a
  native runner.
- Archive and installed artifacts match reviewed hashes and size bounds.
- Failed install, smoke, or promotion leaves the prior trusted installation
  untouched.
- Preview selection loads Privacy Filter only; default selection loads
  DistilBERT only. Display-only mode off with diagnostics disabled loads neither;
  independently sanitized diagnostics may use the same single selected detector.
- All protected output formats pass the critical corpus.
- Selected-backend unavailability is observable and fail-closed.
- Rollback to the prior trusted backend is exercised end to end.
- Docker and Python remain evaluation/release dependencies only.

### Failure handling

A platform with no native smoke is unsupported. A manifest, installer, output
boundary, or silent-fallback failure blocks preview. Large-file distribution
limits must be solved in the release design, not bypassed with unverified
downloads.

## Phase 8 — opt-in preview and release observation

### Goal

Exercise the complete installation and runtime path before changing the default
without collecting raw user content.

### Work

1. Publish Privacy Filter as an explicitly documented preview backend.
2. Publish download, disk, memory, startup, supported-platform, and rollback
   expectations.
3. Run the full gate workflow on every candidate tag and scheduled cadence.
4. Collect only explicit operator reports and non-sensitive CI evidence; do not
   add raw-content telemetry.
5. Keep one stable release cycle and at least fourteen calendar days available
   for preview use before proposing a default change.
6. Perform a documented rollback drill using released artifacts.

### G9 preview-exit criteria

- At least one normal release has shipped the opt-in backend.
- The latest release artifacts pass all G0-G8 gates without waivers.
- No confirmed critical leak, corrupt-install, silent fallback, unbounded
  resource, or deterministic-identity compatibility defect remains open.
- Any confirmed defect has a checked-in synthetic regression before closure.
- The rollback drill succeeds from installed Privacy Filter to trusted
  DistilBERT without loading both in one process.
- Documentation and support guidance match actual platform behavior.

### Failure handling

Keep DistilBERT as default. Fix the defect, add its permanent regression, publish
a new preview candidate, and restart the observation window for critical or
release-integrity failures.

## Phase 9 — make Privacy Filter default and retire DistilBERT

### Goal

Complete the replacement after all evidence is durable.

### Work

1. Change new installs to select the approved Privacy Filter bundle.
2. Preserve an explicit rollback path for one stable release without automatic
   fallback or concurrent loading.
3. Test upgrades with absent, ready, corrupt, and legacy DistilBERT caches.
4. Remove DistilBERT as a selectable production backend only after the rollback
   window and support decision are complete.
5. Remove obsolete model preparation and runtime code in a separate, reviewable
   cleanup change.
6. Keep generic detector, corpus, parity goldens, quality gates, secret tests,
   performance budgets, and release smoke permanently.

### Permanent tests

- TestDefaultBackendIsApprovedPrivacyFilterRevision
- TestUpgradeDoesNotTreatLegacyBundleAsPrivacyFilter
- TestDefaultInstallAndSmokePrivacyFilter
- TestExplicitRollbackLoadsOnlyDistilBERT
- TestLegacyCacheCleanupIsBoundedAndSafe
- TestReleaseContainsOneSelectedModelBundlePerBackendAsset
- all G0-G9 tests and benchmarks

### G10 final pass criteria

- The default release repeats every gate against the exact released artifacts.
- Upgrade, clean install, rollback, and cache cleanup pass on every platform.
- The shipped process loads exactly one detector whenever a protected path
  requires ML. Display-only ModeOff loads none; an independently sanitized
  diagnostic path may initialize that same single selected detector.
- Removing DistilBERT code does not change deterministic display identities or
  any non-detector PII contract.
- The regression and performance suites remain runnable without the retired
  implementation.

### Failure handling

Revert the default selection through a normal release and retain the failing
fixture. Do not reintroduce a silent fallback or ensemble to mask the defect.

## Optional adaptation branch — synthetic domain adaptation

Open weights make domain adaptation possible, but it is not required for the
initial replacement. It may be considered only after the base-model evaluation
infrastructure is trustworthy.

### Additional rules

- Training uses synthetic data only unless a separate privacy and legal process
  authorizes another source.
- Training and locked holdout cases are disjoint by template, value, and context.
- The holdout corpus hash is frozen before training.
- The resulting weights are a new model revision with a new trusted manifest.
- All parity, quality, secret, multilingual, performance, platform, installer,
  preview, and rollback gates repeat.
- The adaptation must improve its declared target slice by at least 2 F2 points
  without violating any existing gate.

A fine-tune that merely memorizes fixtures, reduces preservation, or requires
real customer data fails.

## CI workflow design

The implementation should evolve CI into the following shape.

### Existing test workflow

Every pull request continues to run:

    go test ./...
    go vet ./...
    bash ./scripts/test-race.sh

It gains hermetic corpus, policy, decoder, offset, golden, selection, and
failure-injection tests. It performs no model download.

### Privacy Filter evaluation workflow

A new model-focused workflow should run on:

- changes to PII adapters, policy, corpus, model locks, installer, or evaluator;
- manual dispatch;
- a scheduled cadence; and
- model release candidates.

Jobs should be separated into:

1. Verify immutable source locks and build the reference image.
2. Generate/compare original safetensors, default ONNX, fp16, generic-
   quantized, q4, and q4f16 oracle reports sequentially.
3. Run the Python reference and every ONNX variant's applicable runtime
   compatibility, quality, load, and resource comparison on the Linux H1
   reference host.
4. Run q4 and q4f16 through the full controlled H0/H1/H2 performance matrix.
5. Run the selected candidate's native smoke on Linux and macOS amd64/arm64.
6. Prove the release bundle contains exactly one selected model variant.
7. Publish only synthetic reports and reproducibility metadata.

### Model release workflow

The existing pii-model.yml remains the release authority and must be extended
only after preview integration. A release cannot be created until build, trust,
native smoke, quality, secret, performance, and rollback jobs all pass for the
same immutable candidate manifest.

## Gate result record

Each candidate should produce one small machine-readable summary with this
conceptual shape:

    {
      "schema": 1,
      "git_commit": "...",
      "corpus_sha256": "...",
      "budget_sha256": "...",
      "backend": "privacy-filter",
      "model_revision": "...",
      "variant": "q4",
      "tokenizer_sha256": "...",
      "calibration_sha256": "...",
      "runtime_version": "...",
      "platform": "linux-amd64",
      "hardware_profile": "...",
      "gates": {
        "G0": "pass",
        "G1": "pass",
        "G2": "pass",
        "G3": "pass",
        "G4": "pass",
        "G5": "pass",
        "G6": "pass",
        "G7": "pass",
        "G8": "pass",
        "G9": "pass",
        "G10": "pass"
      }
    }

The real schema will include metric values and evidence artifact names. The
evaluator must calculate gate states; a workflow must not declare pass by
manually setting a label.

## Failure taxonomy and decision rules

| Failure class | Examples | Required response |
|---|---|---|
| Safety blocker | Raw secret/PII survives; mode or diagnostic contract breaks | Stop; fix and add permanent regression |
| Correctness blocker | Offset, decoder, partial result, overlap, identity compatibility | Fix implementation; rerun all downstream gates |
| Candidate-quality failure | Model misses locked cases or over-redacts beyond budget | Try reviewed variant/adaptation or reject candidate |
| Performance failure | Latency, RSS, size, load count, leak, or deadlock exceeds budget | Optimize/retest, offer separately selected profile, or reject |
| Platform failure | No native runtime or smoke | Do not advertise platform; universal replacement normally fails |
| Supply-chain failure | Moving source, hash mismatch, unsafe archive, non-atomic install | Stop release; restore trusted reproducibility |
| Infrastructure-invalid | Noisy host, incomplete report, wrong hardware, cache contamination | Mark not-run; repair and rerun |

Downstream gate results become stale whenever model weights, tokenizer,
calibration, decoder, windowing, ONNX Runtime, policy, corpus, or rendering
changes. CI should encode those dependencies and force re-evaluation.

## Regression retention rules

1. Every confirmed privacy or runtime defect receives the smallest synthetic
   reproducer before it is considered fixed.
2. Critical fixtures are never deleted to improve a score.
3. A changed policy moves a fixture only through a reviewed decision explaining
   the expected user-visible behavior.
4. Golden updates require the explicit generator, pinned metadata, and a human-
   readable diff of typed spans.
5. Performance budgets live in source control; CI results do not rewrite them.
6. Model tests never call Help Scout or OpenAI services.
7. Tests and logs must not print full secret fixtures on failure. Report case ID,
   kind, offsets, and safe hashes only where necessary.
8. Fuzz failures are minimized and checked in as deterministic regressions.
9. Retiring DistilBERT does not retire the baseline reports used to explain the
   migration decision.
10. The hardening contract remains normative over this plan.

## Suggested implementation sequence

Keep changes reviewable and avoid mixing policy, model runtime, and rollout in
one pull request:

1. Corpus schema, secret policy, metric evaluator, and frozen budgets.
2. Generic span contract and behavior-preserving DistilBERT adapter.
3. Process-scoped single-detector lifecycle.
4. Pinned Docker reference/oracle and golden tooling.
5. BIOES/Viterbi decoder and tokenizer/offset tests.
6. Native all-variant comparison, followed by full q4/q4f16 parity evidence.
7. End-to-end quality and secret boundary regressions.
8. Controlled performance harness and selected-variant decision.
9. Backend-aware manifest, installer, status, and preview selection.
10. Native release matrix and rollback drill.
11. Opt-in release and observation.
12. Default switch, rollback window, and later DistilBERT retirement.

Each change should name the gates it advances and include the permanent tests
that create its evidence.

## Definition of done

The migration is complete only when:

- Privacy Filter is the default residual detector;
- DistilBERT is not loaded in a Privacy Filter process;
- all permanent gates pass against the released artifacts;
- secret handling is proven across every output and failure boundary;
- deterministic identities remain compatible;
- quality and performance evidence is reproducible;
- installation and rollback are trusted and tested;
- the old detector can be retired without deleting the generic regression
  suite; and
- the evaluation, this plan, development documentation, and user-facing model
  expectations match the shipped behavior.

Until then, the current detector remains the production default.

## Primary references

- [OpenAI: Introducing OpenAI Privacy Filter](https://openai.com/index/introducing-openai-privacy-filter/)
- [OpenAI Privacy Filter model card](https://cdn.openai.com/pdf/c66281ed-b638-456a-8ce1-97e9f5264a90/OpenAI-Privacy-Filter-Model-Card.pdf)
- [OpenAI Privacy Filter source repository](https://github.com/openai/privacy-filter)
- [OpenAI Privacy Filter model repository](https://huggingface.co/openai/privacy-filter)
- [Pinned first-party ONNX variant directory](https://huggingface.co/openai/privacy-filter/tree/7ffa9a043d54d1be65afb281eddf0ffbe629385b/onnx)
- [hs-cli OpenAI Privacy Filter evaluation](openai-privacy-filter-evaluation.md)
- [hs-cli PII redaction hardening contract](pii-redaction-hardening-contract.md)
