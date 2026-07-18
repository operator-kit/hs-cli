# Privacy Filter evaluation containers

This directory contains repository-owned, digest-pinned Docker tooling for the
Privacy Filter migration gates.

`Dockerfile.g0` is deliberately limited to the Phase 0 current-baseline run. It
contains Go dependencies but no model weights. The baseline script verifies the
existing immutable DistilBERT bundle, mounts it read-only, disables container
networking for inference, and writes only aggregate synthetic-corpus evidence.

The image evaluates both the fast smoke corpus and the locked broad-quality
corpus. An authoritative invocation exits non-zero unless its report contains a
passing G0 result. G0 intentionally remains `not-run` while the checked
H0/H1/H2 hardware identity contract is marked unprovisioned; failed attempts
still leave an aggregate report for CI artifact upload.

The pinned Python reference/oracle image belongs to G2 and is intentionally not
part of Phase 0.
