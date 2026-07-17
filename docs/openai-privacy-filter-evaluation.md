# OpenAI Privacy Filter Evaluation for hs-cli

- Status: investigation complete; implementation not started
- Research date: 16 July 2026
- Decision owner: hs-cli maintainers
- Recommended decision: run a pinned, local evaluation and native-runtime spike; do not replace the production detector until the quality and performance gates in this document pass.

The executable migration sequence, permanent regression suite, numerical
acceptance budgets, and failure rules are defined in the
[single-detector migration plan](openai-privacy-filter-migration-plan.md).

## Executive conclusion

OpenAI Privacy Filter is a credible candidate for a major upgrade to hs-cli's residual free-text detection. It is not a hosted OpenAI API model. It is an Apache-2.0 open-weight token classifier designed to run locally, so adopting it would not require sending unredacted Help Scout content to OpenAI and would not introduce per-token API charges.

The model's strongest potential benefit is coverage. The current local ML detector finds person names; regular expressions and structured-field rules handle the other supported data. Privacy Filter detects contextual spans across eight broad classes:

- private people, including identifying usernames and handles;
- private addresses;
- private email addresses and phone numbers;
- private URLs and IP addresses;
- private dates, including dates of birth;
- account numbers, including financial and government-style identifiers; and
- secrets such as passwords, API keys, one-time passwords, and PINs.

That would address meaningful gaps in unstructured support conversations: pasted credentials, arbitrary customer or project account identifiers, dates that identify a person, obfuscated contact details, usernames, and address or number formats that do not match our present regular expressions. Its contextual policy can also avoid some unnecessary redaction when the same-looking value is public or non-personal.

The trade-off is operational weight and uncertainty:

- The current INT8 ONNX model is 129.1 MiB and ships in trusted platform bundles of 95.4–99.6 MiB compressed. Privacy Filter's original weights are 2.8 GB, while its first-party ONNX q4 and q4f16 external-data files are approximately 874.6 MiB and 771.6 MiB respectively, before the 27.9 MB tokenizer and ONNX Runtime.
- The first-party Python runtime is PyTorch-based. First-party ONNX exports now exist, which makes a native Go path plausible, but they are not drop-in replacements for the current model. Tokenization, 33-label BIOES decoding, calibrated Viterbi decoding, external-data loading, and UTF-8 offset parity all need to be proven.
- OpenAI describes the model as high throughput but publishes no reproducible CPU/GPU latency, memory, or cold-start benchmark with named hardware. We cannot responsibly claim that it will be faster or acceptably interactive for hs-cli until we measure it.
- The model card says the base model is primarily English, shows lower performance on several non-Latin and out-of-distribution languages, and reports substantial degradation on some adversarial formats and context-free categories.
- It is a preview and a detection component, not an anonymization or compliance guarantee.

The recommended end state, if the evaluation succeeds, is:

1. Keep the structured JSON policy, known-identity handling, deterministic HMAC pseudonyms, regex rules, protected-input boundary, and fail-closed behavior.
2. Replace only the residual DistilBERT name detector with a general local span detector backed by Privacy Filter.
3. Translate model spans into hs-cli's existing deterministic replacements instead of emitting OpenAI's generic placeholders.
4. Keep regex rules as defense in depth, including categories the new taxonomy does not clearly cover, such as MAC addresses.
5. Load and reuse one detector per process. A model of this size makes the existing detector-lifetime follow-up a prerequisite rather than an optional optimization.

This is a recommendation to prototype and benchmark, not yet a recommendation to ship.

## Scope and evidence boundary

This report answers four questions:

1. What OpenAI actually released.
2. What additional PII hs-cli could detect.
3. What privacy, runtime, footprint, and maintenance costs adoption would introduce.
4. How to evaluate and integrate it without weakening the hardened redaction contract.

The product and model facts come from the [OpenAI release announcement](https://openai.com/index/introducing-openai-privacy-filter/), the [OpenAI model card](https://cdn.openai.com/pdf/c66281ed-b638-456a-8ce1-97e9f5264a90/OpenAI-Privacy-Filter-Model-Card.pdf), the [OpenAI implementation repository](https://github.com/openai/privacy-filter), and the [OpenAI Hugging Face model repository](https://huggingface.co/openai/privacy-filter). They were reviewed on 16 July 2026.

Repository baseline facts come from the current hs-cli source, especially:

- [the free-text pipeline](../internal/pii/text.go);
- [the current detector](../internal/pii/ner/ner.go);
- [the ONNX runtime adapter](../internal/pii/ner/runtime.go);
- [the source lock](../scripts/pii-model-sources.json);
- [the trusted bundle manifest](../internal/pii/ner/trusted_manifest.json);
- [the multilingual fixture corpus](../internal/pii/testdata/multilingual_privacy_corpus.json); and
- [the PII hardening contract](pii-redaction-hardening-contract.md).

No Privacy Filter weights were downloaded and no local performance benchmark was run as part of this documentation task. All performance conclusions below are therefore one of:

- a directly published architectural or artifact fact;
- a comparison of published artifact sizes; or
- an explicitly labelled hypothesis that the proposed benchmark must test.

That boundary matters. A single example in the model card reports 65.1 ms, but it does not identify hardware, input length, device, warm-up state, or model format. It is not sufficient evidence for an hs-cli latency estimate.

## What OpenAI released

### Product identity

The exact release is OpenAI Privacy Filter, published on 22 April 2026. It is:

- an open-weight model, not an OpenAI API endpoint;
- a bidirectional token-classification model, not a text-generating LLM;
- available under the Apache 2.0 license;
- intended for local, on-premises, browser, or laptop execution;
- fine-tunable for domain-specific data and policy; and
- described by OpenAI as a preview.

This distinction removes the largest concern that the phrase “OpenAI model” might imply: unredacted content does not need to leave the machine. The model download requires network access, but inference does not.

### Architecture

Privacy Filter starts from an autoregressively pretrained checkpoint and is converted into an eight-layer bidirectional classifier. It has:

- 1.5 billion total parameters;
- 50 million active parameters per token through sparse mixture-of-experts routing;
- 128 experts with four selected per token;
- 33 output logits per token: background plus four BIOES boundary states for each of eight privacy classes;
- a maximum sequence length of 131,072 positions and an advertised 128,000-token operating context;
- local bidirectional attention over 128 tokens on each side, or 257 tokens including the current token in the original runtime contract; and
- calibrated constrained Viterbi decoding for coherent span boundaries and tunable precision/recall.

The model labels all tokens in a forward pass rather than autoregressively generating replacement text. That is the right shape for a security boundary: it returns spans, avoids prompt injection as a control mechanism, and does not rewrite unrelated content.

The 128,000-token claim still needs careful interpretation. The effective attention is locally banded, not global across all 128,000 tokens. OpenAI's own one-hop reasoning evaluation shows recall falling as a cue and its later value move farther apart. In addition, the first-party Python CLI defaults CPU execution to 4,096-token windows for safety, even though the checkpoint can be configured for a larger context. For hs-cli, “no chunking” should be treated as a capability to test on target hardware, not a guaranteed default.

### Native taxonomy

| Privacy Filter label | Published meaning | Likely hs-cli action |
|---|---|---|
| private_person | Private person's name, username, or identifying handle | Deterministic fake display identity |
| private_address | Specific location associated with a private person | Deterministic address token |
| private_email | Personal or person-identifying email | Known identity pseudonym or deterministic email token |
| private_phone | Phone number associated with a private person | Known identity pseudonym or deterministic phone token |
| private_url | Private-audience or person-identifying URL; model evaluation also maps IP addresses here | Deterministic URL/IP token |
| private_date | Date of birth, birth year, or another date/time identifying a private person | Deterministic private-date token |
| account_number | Credit card, bank, government, or other account identifier | Deterministic account token |
| secret | API key, password, credential, OTP, or PIN | Constant non-linkable secret marker, with no reversible value |

The model identifies candidate spans. The surrounding application remains responsible for deciding whether to mask, remove, pseudonymize, alert, or route them. This aligns well with hs-cli: the model should detect; the domain layer should enforce policy and choose deterministic display replacements.

### First-party artifacts and runtime options

OpenAI publishes:

- a 2.8 GB safetensors checkpoint and 27.9 MB tokenizer;
- a Python 3.10+ reference implementation using PyTorch, safetensors, tiktoken, NumPy, and Hugging Face Hub;
- Hugging Face Transformers and Transformers.js support; and
- ONNX exports in full, fp16, q4, q4f16, and generic quantized forms.

The ONNX directory is 11.8 GB because it contains every variant. A production bundle would contain exactly one graph and its external data, not the entire directory. The smallest published external-data variants observed in the first-party model repository are:

| Variant | External model data | Relative to current 129.1 MiB model |
|---|---:|---:|
| q4f16 | 809,061,992 bytes / 771.6 MiB | 6.0 times |
| q4 | 917,120,144 bytes / 874.6 MiB | 6.8 times |
| generic quantized | 1,618,042,064 bytes / 1.51 GiB | 12.0 times |
| original safetensors | approximately 2.8 GB / 2.61 GiB | 20.7 times |

These numbers exclude the tokenizer, runtime library, manifest, and compression. Quantized ONNX makes native integration plausible, but the download and resident-memory cost remain materially larger than today.

For reproducibility, an evaluation should pin both upstream identities reviewed here:

- OpenAI code repository commit f7f00ca7fb869683eb732c010299d901457f19c3; and
- Hugging Face model repository commit 7ffa9a043d54d1be65afb281eddf0ffbe629385b, including ONNX export commit 454bcac971532e0fa5863b7043d04d5bb573cd0e.

Pinning only a model name or main branch would not meet hs-cli's current supply-chain standard.

## Current hs-cli baseline

### Existing layered design

The production path is already more than an NER model:

1. Structured identities are recognized from Help Scout resource shape and explicit command context.
2. Known customer and user identities are handled according to off, customers, and all modes.
3. Deterministic keyed HMAC pseudonyms preserve readable display identities without storing a reverse map.
4. A local multilingual DistilBERT model detects residual person names in free text.
5. Regex rules catch email, phone, SSN, credit-card, IBAN, Bitcoin, IP, MAC, URL, postal, PO-box, and street-address forms.
6. Invalid JSON, invalid model spans, detector errors, unavailable models, and unsupported platforms fail closed.
7. Protected command input keeps authored content and credentials out of process arguments and rendered errors.
8. Model installation is provenance-locked, bounded, transactional, hash-verified, and smoke-tested before use.

Privacy Filter should enter this design as one detector adapter. It must not become the policy engine, pseudonym store, installer, output boundary, or only line of defense.

### Current detector and footprint

| Dimension | Current implementation |
|---|---|
| ML task | Person-name NER only |
| Model | Xenova distilbert-base-multilingual-cased-ner-hrl, INT8 ONNX |
| Model file | 135,359,829 bytes / 129.1 MiB |
| Tokenizer | 2,919,362 bytes / 2.8 MiB |
| Trusted platform archive | 95.4–99.6 MiB |
| Expanded bundle | approximately 149.6–169.6 MiB, depending on runtime library |
| Max sequence | 512 tokens |
| Application chunk | at most 1,200 UTF-8 bytes, split at safe boundaries |
| Runtime | ONNX Runtime 1.23 through a Go purego adapter |
| Runtime threading | one intra-op thread |
| Supported release targets | Linux amd64/arm64 and macOS amd64/arm64 |
| Windows behavior | unsupported detector; free text remains fail-closed |
| Published product SLO | none; release smoke has a broad two-minute corpus budget, not an interactive latency SLO |

The present model is much smaller, but repeated 1,200-byte chunks mean inference cost grows as a series of model calls on longer content. Privacy Filter may improve long-input throughput because of one-pass classification, sparse activation, and larger contexts. It may still be slower for short CLI inputs because its checkpoint, tokenizer, runtime initialization, and memory traffic are much larger. Both outcomes are plausible; only measurement can decide.

### Current coverage

The current detector's ML contribution is deliberately narrow: person names. Everything else depends on known structured fields or format-oriented regular expressions.

This is strong for predictable formats and weak for contextual forms. Examples of current gaps or fragile cases include:

- passwords and API tokens whose format is not already known;
- one-time passwords, PINs, recovery codes, and pasted credentials;
- arbitrary bank, customer, utility, tax, government, passport, driving-licence, or internal account identifiers;
- dates of birth and identifying dates;
- usernames and handles not represented as known identities;
- addresses outside the regular expressions' street vocabulary or layout;
- phone numbers written as words or split by unusual whitespace;
- contact details with bracketed “at” or “dot” substitutions;
- values that are sensitive only because nearby prose explains what they are; and
- deciding that a public institution, public office address, or non-personal date should remain visible.

Privacy Filter is specifically designed for this contextual middle ground.

## Coverage comparison

| Data or behavior | Current hs-cli | Privacy Filter potential | Adoption decision |
|---|---|---|---|
| Known Help Scout customer/user identities | Strong, structured, mode-aware, deterministic | Model can detect but cannot supply stable Help Scout identity keys | Keep current path authoritative |
| Unknown third-party person names in prose | DistilBERT NER, then deterministic name-based pseudonym | Context-aware private_person, including names and short fragments | Candidate replacement after recall test |
| Usernames and handles | Incidental only | Explicitly part of private_person | Clear coverage gain |
| Emails and phones | Strong regex plus known identities | Context-aware private_email/private_phone; more obfuscation tolerance | Add as model coverage, retain regex |
| Addresses | English/international street regex plus context patterns | Contextual private_address across varied formats | Likely large gain; test multilingual precision |
| Dates of birth/private dates | Not a dedicated category | Explicit private_date | Clear coverage gain |
| Financial/government/account IDs | Credit cards, SSNs, IBAN, and crypto patterns only | Broad account_number taxonomy; benchmark maps bank, BIC, card, crypto, document, driving-licence, IBAN, ID-card, passport, social, and tax numbers | Large gain; retain checks such as card patterns |
| Passwords/API keys/OTP/PIN | No general secret detector | Explicit secret class | Large gain, but published secret precision is weaker |
| URLs and IPs | Regex for URLs, IPv4, and IPv6 | Contextual private_url, including private IP use in evaluation | May reduce over-redaction; retain regex in high-recall policy |
| MAC addresses | Explicit regex | Not explicitly in native taxonomy | Keep current rule |
| Public versus private context | Mostly format/type based | Trained to distinguish private personal spans from public or benign context | Potential precision gain, but may conflict with local policy |
| Obfuscated formatting | Limited regex coverage | Evaluated on spacing, line breaks, words for digits, symbol substitutions, bracketed dot, phonetic alphabet, and emoji substitutions | Useful but uneven; not a guarantee |
| Long content | Repeated 1,200-byte chunks | Up to 128,000 tokens with local attention and configurable windows | Potential throughput/boundary gain; benchmark required |
| Deterministic readable pseudonyms | Strong HMAC-derived behavior | Emits generic typed spans/placeholders only | Keep hs-cli replacement logic |
| Customer-only mode | Known staff names can be protected | No native concept of hs-cli modes or Help Scout roles | Apply mode policy outside model |
| Offline operation | Yes | Yes | Equivalent privacy posture if kept local |

### The most valuable likely gains

#### 1. Secrets

Support conversations commonly contain credentials during debugging. A general secret class is materially safer than attempting to enumerate every vendor's token prefix. It could detect API keys, passwords, OTPs, and PINs based on both format and surrounding prose.

This class must be treated as high recall but not high precision. On OpenAI's CredData evaluation, secret token recall is 96.5%, while precision is approximately 75% and exact-span F1 is approximately 62%. Benign hashes, placeholders, sample keys, and high-entropy strings can be over-redacted; new credential formats can still be missed.

#### 2. Broad account identifiers

The current rules cover a few familiar numeric formats. Privacy Filter's account_number class covers a much wider set and uses context to distinguish an identifier from an ordinary number. This is especially relevant to support text such as “the utility account is…”, “passport number…”, “customer reference…”, or “bank account…”.

#### 3. Private dates

A date is not always PII. “The release is 18 September” should normally survive, while a date of birth should not. The current regex layer has no general private-date policy. Privacy Filter provides a dedicated contextual class.

#### 4. Contextual names and handles

The current NER model detects names but does not use a privacy-specific public/private taxonomy. Privacy Filter can detect usernames and handles and is trained to preserve public or non-personal context where appropriate. That could reduce false positives while expanding detection.

For the earlier “developer Bob” scenario, Bob should still be detected as an unknown private_person and receive a deterministic name-based display pseudonym. We do not need to identify Bob as a Help Scout customer later. Stable customer/user pseudonyms continue to come from the structured known-identity path; unknown third parties keep the existing best-effort, span-derived identity behavior.

#### 5. Messy and obfuscated text

The model card includes tests for unusual spacing, line breaks, words for digits, bracketed dot forms, Unicode substitutions, emoji replacements, and phonetic spelling. Those are difficult to cover safely with regex alone.

The results are uneven. For example, the reported recall for bracketed-dot obfuscation is high, but precision is only about 56%; phonetic-alphabet precision is about 27%; unusual spacing recall is about 63%. These features expand coverage but cannot replace deterministic rules or targeted adversarial tests.

### What it does not solve

Privacy Filter does not:

- produce a stable customer identity key;
- know whether an entity is a Help Scout customer, staff member, or unrelated third party;
- preserve hs-cli's deterministic fake names or emails by itself;
- dynamically change its label policy for customers versus all mode;
- guarantee anonymization, regulatory compliance, or zero leakage;
- reliably resolve long-range aliases across a long conversation;
- cover every organization-specific secret or identifier without evaluation or fine-tuning;
- clearly replace MAC-address detection; or
- remove the need for structured-field and regex defenses.

## Published quality evidence

### Main benchmark

OpenAI reports the following token-level results on the remapped PII-Masking-300k test set:

| Dataset treatment | Precision | Recall | F1 | Exact span F1 |
|---|---:|---:|---:|---:|
| Baseline labels | 94.0% | 98.0% | 96.0% | 92.6% |
| Corrected labels | 96.8% | 98.1% | 97.4% | 94.2% |

These are strong results, particularly recall. They are not directly comparable to hs-cli's present tests:

- the dataset taxonomy was mapped into Privacy Filter's eight labels;
- some original categories without a matching definition were excluded;
- the corrected result removes adjudicated dataset-label problems;
- the benchmark is not a corpus of Help Scout threads; and
- hs-cli currently asserts specific safety invariants rather than calculating corpus-wide precision and recall.

The corrected benchmark is useful evidence of capability, not proof of our production quality.

### Domain adaptation

On the out-of-distribution SPY legal/medical dataset, the published zero-shot token F1 is approximately 54.5%. Fine-tuning with 1% of the training split raises it to approximately 87.9%; 10% raises it to approximately 96.2%.

This result has two implications:

1. Small, targeted fine-tuning can be effective.
2. A policy or domain mismatch can be severe before adaptation.

We should not begin by fine-tuning. First evaluate the base checkpoint against a sufficiently rich hs-cli corpus. Fine-tuning introduces training-data governance, artifact ownership, reproducibility, and ongoing maintenance. It is a second-stage option only if the base model narrowly misses an otherwise attractive adoption gate.

### Multilingual evidence

On the in-distribution PII-Masking-300k languages reported by OpenAI, F1 is approximately:

- Dutch 91.4%;
- English 93.4%;
- French 92.7%;
- German 92.6%;
- Italian 92.1%; and
- Spanish 93.3%.

On additional synthetic languages, reported F1 ranges from 75.8% for Hausa to 93.3% for Portuguese. Bengali, Hindi, Japanese, Korean, Mandarin Chinese, Modern Standard Arabic, Russian, Turkish, Urdu, and Western Punjabi fall between those values.

In a separate “category clue before PII” evaluation, recall drops materially outside English: approximately 71.5–82.1% across the listed non-English languages versus 95.6% for English. The model metadata itself says “primarily English”.

This is not a reason to reject it, but it prevents us from inheriting the published aggregate score as a multilingual claim. Our existing Arabic and Han fixtures are too small to answer the question. The evaluation corpus must expand before any advertised language change.

### Context sensitivity

OpenAI's synthetic clue-position results show why the model is interesting and why it remains probabilistic:

- private_person recall is approximately 88.5% with no clue and 99.4% when a clear clue appears first;
- private_email recall is approximately 96.2% with no clue and 100% with a clear clue first;
- account_number recall rises from approximately 79.7% to 100% with a clear clue first;
- private_date recall rises from approximately 22.0% to 89.4%;
- private_phone recall rises from approximately 31.8% to 91.9%;
- secret recall rises from approximately 36.1% to 70.8%; and
- private_address recall remains low in this toy set, from approximately 21.4% without a clue to 41.4% with a clue first.

The model is strongest when real conversational context identifies a span. Bare values and very short strings can still be hard. Our regex layer remains especially valuable for context-free emails, phone numbers, cards, and identifiers.

### Failure modes that matter to hs-cli

The model card explicitly calls out:

- uncommon names, regional naming conventions, initials, and honorific-heavy references;
- non-English text and non-Latin scripts;
- organization- or domain-specific identifiers;
- ambiguous public entities, locations, and common nouns;
- heavy punctuation, layout artifacts, and fragmented span boundaries;
- novel or split credential formats;
- benign high-entropy values and example credentials; and
- reduced long-range alias resolution as the defining clue moves away.

These overlap strongly with support content, which often includes quoted email chains, signatures, HTML-to-Markdown artifacts, logs, stack traces, copied configuration, and short fragments. The model must be tested on those shapes, not only on clean prose.

## Performance and operational analysis

### What is likely better

The architecture has credible performance advantages:

- It labels tokens rather than generating output token by token.
- Only four of 128 experts are active per token.
- Its attention is locally banded.
- It can process far larger windows than the current 512-token model.
- Larger windows can remove repeated tokenization and inference calls and reduce chunk-boundary errors.
- The ONNX q4 variants provide a possible CPU-friendly native path.

For a long conversation, these properties could offset some of the larger checkpoint cost.

### What is likely worse

The larger artifacts create predictable costs:

- first install download and verification are substantially larger;
- cold start must map or load much more model data;
- peak and resident memory are likely higher;
- filesystem and antivirus scanning may become more visible;
- CI smoke artifacts and release storage become larger;
- low-memory laptops and small containers may be excluded;
- model initialization repeated within a process would be unacceptable; and
- Docker image size grows sharply if weights are embedded rather than mounted or cached.

The 50-million-active-parameter figure describes compute routing, not storage. All experts still need to be available in the checkpoint. Sparse activation therefore does not make a 0.8–2.8 GB artifact behave like a 50-million-parameter download.

### Unknown latency

OpenAI does not publish a hardware-normalized benchmark for:

- cold load time;
- warm p50 or p95 latency;
- tokens per second;
- CPU model and thread count;
- GPU model;
- peak or resident memory;
- q4 versus q4f16 accuracy and speed;
- short versus long inputs; or
- concurrent requests.

The release's “fast” and “high-throughput” descriptions are architectural/product claims, not a sizing guide for this CLI. We must measure the exact artifact, runtime, decoder, and hardware we intend to ship.

### Detector lifetime is a prerequisite

At present, newPIIEngine can construct a new ONNX detector whenever an engine is created. The detector is not a process-scoped dependency owned by a clear lifecycle boundary. That is already tracked as follow-up A2.

Privacy Filter increases the cost of that design enough that A2 should be completed before a production integration:

- initialize the detector once per process;
- share it safely across engines and MCP calls;
- serialize or bound concurrent inference according to runtime support;
- close it exactly once at process shutdown; and
- expose load and inference failures without returning partially inspected content.

This change should be backend-neutral so it improves the current DistilBERT path even if Privacy Filter is not adopted.

### Platform considerations

The current trusted model matrix covers Linux and macOS on amd64 and arm64. Windows deliberately fails closed because native runtime/model smoke is not yet established.

Privacy Filter's Hugging Face repository makes ONNX and browser/WebGPU forms available, but that does not automatically expand hs-cli support:

- the existing Go runtime adapter must load an ONNX graph with external data;
- the selected quantized graph's operators must work with the pinned ONNX Runtime on each CPU architecture;
- the tokenizer must produce source-compatible offsets;
- calibrated decoding must match the reference implementation;
- peak memory must fit supported machines; and
- every advertised platform must pass native smoke with the exact released bundle.

Windows may become possible, but it should be treated as a separate capability gate, not an assumed benefit.

### Cost model

There is no OpenAI API usage fee. Costs move to:

- artifact storage and transfer;
- developer and CI time;
- CPU/GPU time;
- local disk and memory;
- release engineering;
- vulnerability and dependency maintenance; and
- optional fine-tuning.

For hs-cli's deployment model, local compute is preferable to sending raw text to a hosted service. It is not free; it is simply paid in infrastructure and product footprint instead of API tokens.

## Privacy, security, licensing, and supply chain

### Privacy

A local integration preserves the best property of the current design: text is filtered before it reaches any LLM or remote analysis workflow, and the unfiltered content need not leave the user's machine.

The implementation must avoid accidentally weakening that property:

- do not invoke the model through a hosted Hugging Face inference provider;
- do not send raw content to a remote OpenAI endpoint;
- do not put text in child-process arguments;
- do not write temporary plaintext request or result files;
- do not log the upstream JSON output, which includes the original input text;
- if a local sidecar is used for a spike, communicate over inherited pipes or a private local socket; and
- retain fail-closed behavior on model, tokenizer, decoding, offset, or transport failure.

### Supply-chain standard

The current installer is stronger than the upstream reference downloader. It pins immutable revisions and hashes, embeds reviewed identities, applies archive and expansion limits, installs transactionally, and runs real inference before promotion.

The upstream Python helper downloads the named Hugging Face repository when its default checkpoint is absent. That is convenient for experimentation but is not acceptable as the production hs-cli trust boundary.

A production Privacy Filter bundle must:

- pin the model repository revision;
- list every graph, external-data, tokenizer, calibration, config, runtime, and license file;
- hash and size every file;
- reject symlinks and unexpected files;
- set revised but finite archive/expansion limits;
- verify an artifact in private staging;
- run tokenizer, logits, decoder, and end-to-end smoke tests;
- promote atomically; and
- include Apache 2.0 license and notice obligations in release review.

The published model.sig file should not be treated as a trust mechanism until its format, signer identity, verification procedure, and key-rotation policy are documented and implemented. Our embedded digest remains the trust anchor.

### Dependency surface

The Python reference stack introduces Python, PyTorch, Hugging Face Hub, NumPy, safetensors, tiktoken, and packaging. That is appropriate for a repo-owned benchmark container, but a major regression for a standalone Go CLI if required on every user's machine.

The preferred production path is therefore the pinned ONNX artifact through the existing native runtime model, provided parity and performance pass. A Python sidecar is a useful reference and evaluation oracle, not the preferred default distribution.

### Licensing

OpenAI publishes the code and model under Apache 2.0 and explicitly describes commercial deployment as permitted. hs-cli is MIT licensed. Distribution should preserve the upstream licence and notices and be reviewed as a normal third-party model dependency. This report is an engineering assessment, not legal advice.

## Preserving deterministic identities and modes

Privacy Filter must not replace hs-cli's identity semantics.

### Known identities

Known customer and staff records continue through the structured identity path. Their deterministic display names and emails remain derived from the current PseudonymContext, key ID, and identity schema. A model prediction must never choose a different pseudonym for a known identity.

### Unknown people

For a private_person span not linked to a known identity:

- canonicalize the detected source text as today;
- derive the fake person through the existing HMAC path;
- preserve the current display disambiguator and key ID; and
- accept that two mentions with different aliases may not resolve to the same person.

That matches the agreed trade-off for third parties such as a developer mentioned by a customer. Attempting global entity resolution would add complexity and new privacy state for little current benefit.

### Non-person spans

Map each model label to a stable internal kind and use the existing keyed token generation:

| Model label | Internal replacement kind |
|---|---|
| private_address | address |
| private_email | email |
| private_phone | phone |
| private_url | url or ip after deterministic classification |
| private_date | private_date |
| account_number | account_number |
| secret | secret |

This provides stable tokens without retaining raw-to-redacted mappings.

### customers mode

The detector has no concept of staff versus customer. The application must reconcile predictions with known identities:

- redact known customers;
- protect known staff identity spans in customers mode;
- redact unknown private third-party spans;
- retain current deterministic handling for all non-person PII; and
- continue to treat mode off as pass-through at every low-level boundary.

The exact staff treatment for model-detected email, phone, URL, and address spans should be captured in policy fixtures before implementation. A model backend must not silently redefine customers mode.

## Architecture options

| Option | Coverage | Runtime cost | Architectural fit | Recommendation |
|---|---|---|---|---|
| Replace the entire pipeline with Privacy Filter placeholders | Broad, but loses deterministic and structured guarantees | High | Poor; weakens existing contracts | Reject |
| Replace DistilBERT only; keep structured, deterministic, regex, and fail-closed layers | Broad residual coverage | Medium/high | Strong | Preferred target if gates pass |
| Run DistilBERT and Privacy Filter permanently as an ensemble | Highest potential recall | Highest latency/memory | Acceptable only for a high-assurance profile | Use for evaluation, not default |
| Use the Python implementation as a local subprocess/sidecar | Reference-correct and fast to prototype | Python/PyTorch footprint and process complexity | Good evaluation harness, poor standalone default | Use for spike only |
| Use a pinned ONNX variant in the Go process | Broad coverage, no Python | Approximately 0.8–1.6 GB model plus integration work | Best production fit if parity passes | Preferred implementation spike |
| Send text to a hosted model | Potentially simple operations | Network latency, availability, privacy, and service cost | Conflicts with local-first boundary | Reject for this release |

### Recommended processing flow

The target flow should remain layered:

    Help Scout payload
        |
        +-- structured resource policy and known identities
        |
        +-- original free text
              |
              +-- local general span detector
              |
              +-- validate UTF-8 source offsets and reconcile overlaps
              |
              +-- protect or redact known identities according to mode
              |
              +-- deterministic person pseudonyms / deterministic typed tokens
              |
              +-- existing regex defense-in-depth sweep
              |
              +-- fail-closed output boundary

The model supplies evidence. The domain layer owns decisions and replacements.

## Proposed clean architecture

### 1. Generalize the domain interface

The current NameDetector returns only NameSpan values. Introduce a backend-neutral span contract in internal/pii:

    type SpanKind string

    type DetectedSpan struct {
        Kind  SpanKind
        Start int
        End   int
        Score float32
    }

    type SpanDetector interface {
        Detect(text string) ([]DetectedSpan, error)
    }

The domain package should define hs-cli meanings, not import OpenAI label strings. An adapter maps private_person to SpanPerson and so on. The existing DistilBERT adapter can return SpanPerson, allowing both backends to exercise the same policy and replacement tests.

### 2. Separate detection, policy, and rendering

Split the current RedactText responsibilities conceptually:

- Detector: identify source spans and confidence.
- Span validator: verify bounds, UTF-8 byte alignment, source identity, order, and overlap.
- Policy reconciler: apply mode and known-identity protections.
- Replacement renderer: choose deterministic fake people or typed tokens.
- Deterministic fallback layer: apply regex rules and fail closed.

This prevents model-specific concerns from spreading into JSON traversal, identity generation, or command code.

### 3. Make offset units explicit

The OpenAI Python JSON schema reports Python character offsets. hs-cli's critical contract requires UTF-8 byte offsets. Hugging Face tokenizers may expose character or byte-based offsets depending on the path.

The adapter must declare and normalize its offset unit. Tests must cover:

- accented Latin text;
- Arabic;
- Han characters;
- emoji;
- combining marks;
- CRLF;
- HTML/Markdown punctuation; and
- a tokenizer decode mismatch.

Any unresolvable or inconsistent span fails closed. We must not repeat the class of Unicode bug already fixed under critical contract 4.

### 4. Match calibrated decoding

The published results use BIOES-aware constrained Viterbi decoding and a calibration file. Taking argmax directly from ONNX logits is not equivalent.

The Go adapter should:

- load the pinned label map and viterbi_calibration.json;
- validate all 33 output classes;
- implement the allowed BIOES transitions;
- apply the six calibrated transition biases;
- produce coherent non-overlapping spans; and
- pass golden parity fixtures against the pinned Python reference.

Model adoption must be based on the full reference behavior, not a simpler decoder with unknown quality.

### 5. Own one detector per process

Introduce a detector provider/lifecycle owner above individual Engine instances. It should:

- lazy-load once when redaction is enabled;
- share immutable tokenizer/config state;
- safely guard runtime calls;
- expose status and backend identity;
- close at process shutdown;
- avoid initialization in mode off; and
- make failure observable while preserving fail-closed output.

This encapsulates the large runtime and completes the existing A2 follow-up in a backend-neutral way.

### 6. Version model bundles by backend

Do not overwrite the current DistilBERT bundle in place during experimentation. The installer/status model should distinguish:

- detector backend;
- model version and upstream revision;
- artifact format and quantization;
- runtime version;
- label schema;
- decoder schema/calibration; and
- supported platform.

That permits safe rollback and side-by-side offline comparison without making an old CLI interpret a new bundle.

### 7. Keep Docker in the right role

A repo-owned Docker image is the best initial environment for:

- pinned Python and PyTorch dependencies;
- downloading/verifying the upstream reference;
- reproducible quality evaluation;
- CPU baseline benchmarks;
- optional GPU benchmark profiles; and
- producing reference predictions for ONNX parity.

Requiring Docker to use the normal hs binary would be a large product regression. Docker should be the evaluation and release-engineering harness unless the project intentionally introduces a server deployment profile.

## Evaluation and implementation plan

### Phase 0 — freeze the policy before comparing models

Add a versioned synthetic span corpus that describes what hs-cli means by PII. It should include at least:

- known customers, known staff, and unknown third parties;
- names, handles, emails, phones, addresses, dates, account IDs, URLs/IPs, and secrets;
- public people, public business addresses, release dates, order numbers, hashes, placeholders, and example credentials that should survive;
- English, French, German, Spanish, Arabic, Han, Japanese, Korean, Hindi, and Portuguese cases;
- quoted replies, signatures, HTML-to-Markdown output, logs, stack traces, JSON, and code;
- unusual spacing, line breaks, bracketed dot/at, digit words, Unicode substitution, and minimal names;
- repeated values and overlapping candidate spans;
- long threads with values near and far from contextual clues; and
- all three modes.

Each case should carry exact byte spans, category, expected action, mode, and known-identity context. No real Help Scout or customer data should enter the repository.

### Phase 1 — build a pinned reference harness

Create a repo-owned Docker evaluation target that:

1. Pins the OpenAI code and model revisions listed above.
2. Verifies every downloaded artifact hash.
3. Runs only on synthetic fixtures.
4. Emits structured spans, not just masked text.
5. Captures model variant, device, thread count, input tokens, cold/warm state, elapsed time, and peak RSS.
6. Stores aggregate metrics but never unredacted real content.

Use the original Python runtime as the behavioral oracle. This establishes expected Viterbi and offset behavior independently of our Go adapter.

### Phase 2 — benchmark quality and performance

Run the existing detector and Privacy Filter against the same policy corpus.

Quality metrics:

- exact-span and overlap-tolerant precision, recall, F1, and F2;
- per-category recall;
- per-language and per-script recall;
- false negatives weighted by data sensitivity;
- over-redaction rate;
- customer/staff separation;
- deterministic replacement stability; and
- fail-closed behavior.

Performance metrics:

- model download and verified-install size;
- cold load time;
- warm p50, p95, and p99 latency;
- tokens per second;
- peak RSS and steady RSS;
- CPU time and wall time;
- short subject, typical thread, full conversation, and long export sizes;
- one process versus repeated process startup;
- concurrency 1 and expected MCP concurrency; and
- q4 versus q4f16 quality/latency parity.

Target hardware:

- Linux amd64 CI-class CPU;
- Linux arm64;
- macOS Intel;
- macOS Apple Silicon;
- a low-memory laptop/container profile; and
- Windows amd64 only as an exploratory, separately gated target.

The current two-minute release-smoke budget should remain a safety ceiling, not become the benchmark acceptance criterion.

### Phase 3 — prove native ONNX parity

Before product integration:

1. Load the pinned q4 and q4f16 graphs with the current or intentionally upgraded ONNX Runtime.
2. Verify external-data resolution from a trusted bundle directory.
3. Verify tokenizer compatibility or implement a narrow o200k tokenizer adapter.
4. Implement calibrated Viterbi decoding.
5. Compare logits, labels, spans, UTF-8 byte offsets, and final decisions with the Python oracle.
6. Measure accuracy drift caused by quantization.
7. Run the full native platform matrix.

Choose q4 or q4f16 based on measured quality, memory, and latency. Smaller is not automatically safer if quantization reduces recall.

### Phase 4 — integrate behind an explicit backend selection

Add a development/preview detector selection without creating a redaction bypass:

- distilbert remains the production default initially;
- privacy-filter selects the new trusted bundle;
- a selected but unavailable/corrupt backend keeps free text fail-closed;
- mode off remains a true low-level pass-through;
- backend identity appears in pii-model status and diagnostics without exposing input; and
- the existing output API and pseudonym formats remain stable.

Do not silently fall back from a selected high-assurance backend to a weaker detector. A fallback that changes coverage should be explicit and observable.

### Phase 5 — controlled rollout

Promote only after:

- quality and performance gates pass on every advertised platform;
- model bundles are reproducible and smoke-tested;
- documentation explains the larger download and resource requirements;
- rollback to the previous backend is tested;
- release CI has adequate artifact/storage limits; and
- maintainers accept the support cost.

A sensible rollout order is opt-in preview, then default for new installs, then optional retirement of DistilBERT after at least one stable release cycle.

## Regression-test plan

### Domain unit tests

Add backend-independent tests for:

- every SpanKind to replacement mapping;
- known customer precedence over model-detected person spans;
- known staff protection in customers mode;
- unknown third-party deterministic fake identities;
- repeated unknown names mapping consistently;
- account, date, address, URL, email, and phone deterministic tokens;
- secret spans using a constant non-linkable marker rather than a pseudonym;
- overlap precedence between known identity, model, and regex spans;
- invalid, reversed, out-of-range, non-UTF-8-boundary, and source-mismatched spans failing closed;
- detector error, panic boundary, and partial-result failure;
- mode off making no detector call and changing no value; and
- customers/all mode differences.

### Adapter parity tests

For each pinned synthetic input, compare Python reference and Go ONNX adapter:

- token count and token/source round trip;
- label ordering and calibration;
- exact typed spans;
- byte offsets;
- quantized versus original predictions; and
- long-window overlap aggregation.

Use checked-in expected spans, not a live Hugging Face or OpenAI call in normal tests.

### Release smoke

Extend the model-release workflow to:

- build one bundle per explicitly supported platform;
- check exact artifact names, hashes, and bounded sizes;
- load the real ONNX graph and external data;
- run all critical categories and scripts;
- include no-PII preservation cases;
- verify no unexpected raw values remain after rendering;
- enforce a measured runtime budget chosen from the benchmark; and
- publish benchmark metadata with the model release.

### Command and end-to-end tests

Reuse Help Scout fixtures to prove:

- table, JSON, JSON-full, CSV, source, and RFC822 paths use the same detector;
- MCP and direct CLI behavior match;
- no input or model output reaches argv, logs, debug files, or error strings;
- multiple engines/MCP requests reuse the process detector;
- a missing/corrupt selected model fails closed; and
- structured deterministic identities remain byte-for-byte compatible.

## Proposed adoption gates

The team should agree absolute product SLOs after measuring the current baseline. The following are recommended safety gates.

### Quality

- No regression in any existing PII hardening or fixture test.
- Zero leaks in the finite critical safety corpus.
- Per-category recall for names, email, phone, account number, and secret is no worse than the stronger of the current detector-plus-regex baseline or the agreed minimum.
- Overall F2 and sensitivity-weighted false-negative score improve materially.
- Non-English/script slices do not regress from currently advertised behavior.
- Public/non-PII preservation stays within an agreed over-redaction budget.
- q4/q4f16 results are evaluated independently; published full-precision metrics are not inherited.

### Performance

- Warm p95 latency meets an explicit interactive CLI budget on all supported CPU targets.
- Cold-start impact is acceptable for one-shot CLI use.
- MCP process reuse produces stable memory and no resource growth.
- Peak RSS fits the lowest supported memory profile with headroom.
- Install/download size is documented and accepted.
- Long content completes within an explicit timeout without silent truncation.

### Correctness and reliability

- Python/ONNX span parity passes on the reference corpus.
- UTF-8 offset conversion passes all scripts and combining-mark cases.
- Viterbi calibration is loaded and validated.
- Any incomplete inference, window, decode, or replacement fails closed.
- Selected-backend unavailability never silently returns raw text.

### Operations and security

- Every artifact is immutable, hashed, bounded, and reproducible.
- Native smoke passes on every advertised platform.
- No raw input crosses a remote boundary.
- No mandatory user-managed Python or Docker dependency is introduced for the default binary.
- Rollback and bundle coexistence are tested.

If quality improves but the default footprint or latency is unacceptable, a valid outcome is an optional “enhanced” detector profile for long-running MCP/server deployments while retaining DistilBERT for lightweight one-shot use.

## Likely implementation impact

This is not a final diff plan, but the likely seams are:

| Area | Likely change |
|---|---|
| internal/pii/identity.go | General SpanDetector and DetectedSpan domain contracts |
| internal/pii/text.go | Span policy, replacement mapping, overlap handling, and regex layering |
| internal/pii/ner | Adapt current DistilBERT detector to the generic contract |
| new internal/pii/privacyfilter package | Tokenizer, ONNX adapter, 33-label map, Viterbi decoder, and backend tests |
| internal/cmd/inbox_pii_helpers.go | Process-scoped detector provider and backend selection |
| internal/pii/ner model/manifest installer code | Backend-aware trusted bundles, larger bounded external-data files, and status |
| scripts/pii-model-sources.json | Pinned Privacy Filter source/artifact identities |
| scripts/prepare-pii-model.sh | Reproducible selected-variant bundle creation |
| .github/workflows/pii-model.yml | Larger artifact handling, native parity/smoke, and benchmark metadata |
| internal/pii/testdata | Versioned typed span, adversarial, multilingual, and preservation corpus |
| README.md and DEVELOPMENT.md | Download/resource expectations, backend behavior, and contributor workflow |

Avoid coupling core packages to the upstream Python CLI schema or OpenAI-specific strings. Those belong in the adapter.

## Open questions to answer in the spike

1. Does q4 or q4f16 preserve the recall we care about, especially names, secrets, and non-Latin scripts?
2. Can ONNX Runtime 1.23 execute the selected graph and quantized operators on every target, or is an upgrade required?
3. Does the existing purego wrapper correctly resolve ONNX external-data files and dynamic input sizes?
4. Can the existing Go tokenizer library load the 27.9 MB tokenizer with exact reference offsets, or is a dedicated o200k adapter required?
5. What is the cold load time and RSS for one-shot CLI use?
6. What is warm p95 latency for typical Help Scout threads on CPU?
7. Does a 4,096-token window outperform larger contexts on CPU without reducing recall?
8. How should long-window overlap and predictions be merged without duplicate or fragmented spans?
9. Which public/business entities should hs-cli preserve in all mode?
10. Should model-detected staff contact fields remain visible in customers mode, matching protected staff names, or should non-name PII remain universally redacted?
11. Do release storage, GitHub artifact limits, npm distribution, Homebrew, and end-user cache expectations tolerate an approximately 0.8–1.0 GB detector bundle?
12. Is an optional enhanced profile a better product fit than making this the universal default?

## Final recommendation

Proceed with a bounded evaluation.

Privacy Filter is the first candidate found in this review that could materially expand hs-cli from “name NER plus known formats” to contextual PII and secret detection while preserving local inference. The first-party ONNX variants make a clean native path plausible, and the Apache 2.0 licence permits the required inspection, packaging, and adaptation.

Do not yet replace DistilBERT. The artifact is six to seven times larger even in the smallest relevant ONNX forms, published performance lacks hardware-specific latency data, multilingual and adversarial results are uneven, and the base model's private/public policy may not exactly match hs-cli.

The best next investment is a repo-owned, synthetic-only Docker benchmark using the pinned Python implementation as an oracle, followed by a q4/q4f16 ONNX parity spike. If it improves sensitivity-weighted recall without violating CLI latency, memory, platform, deterministic-identity, and fail-closed gates, adopt it as the residual detector beneath the existing hardened contract.

## Primary sources

- [OpenAI: Introducing OpenAI Privacy Filter, 22 April 2026](https://openai.com/index/introducing-openai-privacy-filter/)
- [OpenAI Privacy Filter model card](https://cdn.openai.com/pdf/c66281ed-b638-456a-8ce1-97e9f5264a90/OpenAI-Privacy-Filter-Model-Card.pdf)
- [OpenAI Privacy Filter source repository](https://github.com/openai/privacy-filter)
- [OpenAI Privacy Filter model repository and model card](https://huggingface.co/openai/privacy-filter)
- [First-party ONNX artifacts](https://huggingface.co/openai/privacy-filter/tree/7ffa9a043d54d1be65afb281eddf0ffbe629385b/onnx)
- [First-party structured output schema](https://github.com/openai/privacy-filter/blob/f7f00ca7fb869683eb732c010299d901457f19c3/OUTPUT_SCHEMAS.md)
- [Hugging Face Transformers implementation documentation](https://huggingface.co/docs/transformers/model_doc/openai_privacy_filter)
- [Apache 2.0 license in the OpenAI repository](https://github.com/openai/privacy-filter/blob/f7f00ca7fb869683eb732c010299d901457f19c3/LICENSE)
