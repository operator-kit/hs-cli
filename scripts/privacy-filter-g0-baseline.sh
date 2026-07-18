#!/usr/bin/env bash
# Produce the current DistilBERT-plus-regex baseline over the synthetic G0 corpus.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TRUST_MANIFEST="${ROOT_DIR}/internal/pii/ner/trusted_manifest.json"
SOURCE_LOCK="${ROOT_DIR}/scripts/pii-model-sources.json"
CACHE_DIR="${HS_PII_G0_CACHE_DIR:-${TMPDIR:-/tmp}/hs-cli-pii-g0-cache}"
REPORT_DIR="${HS_PII_G0_REPORT_DIR:-${ROOT_DIR}/dist/privacy-filter/g0}"
IMAGE_TAG="${HS_PII_G0_IMAGE_TAG:-hs-cli/privacy-filter-g0:local}"

for command in curl docker git jq sha256sum stat tar; do
  command -v "$command" >/dev/null || { echo "missing required command: $command" >&2; exit 1; }
done

if ! docker info >/dev/null 2>&1; then
  echo "Docker is required for the G0 baseline and its engine is unavailable." >&2
  exit 1
fi

MODEL_REVISION="$(jq -er '.model.revision' "$SOURCE_LOCK")"
MANIFEST_MODEL_REVISION="$(jq -er '.model_revision' "$TRUST_MANIFEST")"
[[ "$MODEL_REVISION" == "$MANIFEST_MODEL_REVISION" ]] || { echo "G0 model source lock and trust manifest disagree" >&2; exit 1; }
BUNDLE="$(jq -ec '.bundles[] | select(.goos == "linux" and .goarch == "amd64")' "$TRUST_MANIFEST")"
ARCHIVE_URL="$(jq -er '.archive.url' <<<"$BUNDLE")"
ARCHIVE_NAME="$(jq -er '.archive.filename' <<<"$BUNDLE")"
ARCHIVE_SHA256="$(jq -er '.archive.sha256' <<<"$BUNDLE")"
ARCHIVE_SIZE="$(jq -er '.archive.size' <<<"$BUNDLE")"
MODEL_SHA256="$(jq -er '.files[] | select(.name == "model_quantized.onnx") | .sha256' <<<"$BUNDLE")"
ARTIFACTS_JSON="$(jq -c '[
  {name: "archive", sha256: .archive.sha256, size_bytes: .archive.size}
] + [.files[] | {name: .name, sha256: .sha256, size_bytes: .size}]' <<<"$BUNDLE")"

mkdir -p "$CACHE_DIR" "$REPORT_DIR"
ARCHIVE_PATH="${CACHE_DIR}/${ARCHIVE_NAME}"
if [[ ! -f "$ARCHIVE_PATH" ]] ||
   [[ "$(stat -c '%s' "$ARCHIVE_PATH")" != "$ARCHIVE_SIZE" ]] ||
   [[ "$(sha256sum "$ARCHIVE_PATH" | awk '{print $1}')" != "$ARCHIVE_SHA256" ]]; then
  DOWNLOAD_PATH="${ARCHIVE_PATH}.download"
  curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 --retry 3 \
    "$ARCHIVE_URL" -o "$DOWNLOAD_PATH"
  [[ "$(stat -c '%s' "$DOWNLOAD_PATH")" == "$ARCHIVE_SIZE" ]] || { echo "G0 bundle size mismatch" >&2; exit 1; }
  [[ "$(sha256sum "$DOWNLOAD_PATH" | awk '{print $1}')" == "$ARCHIVE_SHA256" ]] || { echo "G0 bundle hash mismatch" >&2; exit 1; }
  mv -f "$DOWNLOAD_PATH" "$ARCHIVE_PATH"
fi

STAGING="$(mktemp -d "${TMPDIR:-/tmp}/hs-cli-pii-g0-model.XXXXXX")"
cleanup() {
  rm -rf "$STAGING"
  if [[ -n "${REPORT_PATH:-}" && -f "$REPORT_PATH" ]]; then
    chmod 0600 "$REPORT_PATH"
  fi
}
trap cleanup EXIT
ARCHIVE_NAMES="$(tar -tzf "$ARCHIVE_PATH" | sed -e 's#^\./##' -e '/^$/d' | LC_ALL=C sort)"
while IFS= read -r name; do
  [[ -n "$name" && "$name" != */* && "$name" != "." && "$name" != ".." ]] || {
    echo "G0 bundle contains an unsafe archive path" >&2
    exit 1
  }
done <<<"$ARCHIVE_NAMES"
tar -xzf "$ARCHIVE_PATH" -C "$STAGING"

EXPECTED_NAMES="$(jq -r '.files[].name' <<<"$BUNDLE" | LC_ALL=C sort)"
ACTUAL_NAMES="$(find "$STAGING" -mindepth 1 -maxdepth 1 -printf '%f\n' | LC_ALL=C sort)"
[[ "$ARCHIVE_NAMES" == "$EXPECTED_NAMES" ]] || { echo "G0 archive index contains unexpected or missing entries" >&2; exit 1; }
[[ "$ACTUAL_NAMES" == "$EXPECTED_NAMES" ]] || { echo "G0 bundle contains unexpected or missing files" >&2; exit 1; }
while IFS= read -r file; do
  NAME="$(jq -er '.name' <<<"$file")"
  EXPECTED_HASH="$(jq -er '.sha256' <<<"$file")"
  EXPECTED_SIZE="$(jq -er '.size' <<<"$file")"
  [[ -f "${STAGING}/${NAME}" && ! -L "${STAGING}/${NAME}" ]] || { echo "G0 bundle entry is not a regular file: $NAME" >&2; exit 1; }
  [[ "$(stat -c '%s' "${STAGING}/${NAME}")" == "$EXPECTED_SIZE" ]] || { echo "G0 file size mismatch: $NAME" >&2; exit 1; }
  [[ "$(sha256sum "${STAGING}/${NAME}" | awk '{print $1}')" == "$EXPECTED_HASH" ]] || { echo "G0 file hash mismatch: $NAME" >&2; exit 1; }
done < <(jq -c '.files[]' <<<"$BUNDLE")

docker build --file "${ROOT_DIR}/build/privacy-filter/Dockerfile.g0" --tag "$IMAGE_TAG" "$ROOT_DIR"
IMAGE_ID="$(docker image inspect "$IMAGE_TAG" --format '{{.Id}}')"

AUTHORITY="${HS_PII_G0_EVIDENCE_AUTHORITY:-local-sanity}"
AUTHORITATIVE="${HS_PII_G0_AUTHORITATIVE:-false}"
if [[ "$AUTHORITATIVE" == "true" ]] &&
   { [[ "$AUTHORITY" != "docker-ci" ]] || [[ "${GITHUB_ACTIONS:-false}" != "true" ]] || [[ -z "${RUNNER_NAME:-}" ]]; }; then
  echo "authoritative G0 evidence is restricted to identified GitHub Actions Docker CI runners" >&2
  exit 1
fi
if [[ "$AUTHORITY" == "local-sanity" ]]; then
  AUTHORITATIVE="false"
fi

GIT_COMMIT="${GITHUB_SHA:-$(git -C "$ROOT_DIR" rev-parse HEAD)}"
REPORT_DIR="$(cd "$REPORT_DIR" && pwd)"
REPORT_PATH="${REPORT_DIR}/distilbert-baseline.json"
if [[ "$AUTHORITY" == "local-sanity" ]] &&
   [[ "$REPORT_PATH" == "${ROOT_DIR}/internal/pii/testdata/privacy-filter/v1/performance/"* ]]; then
  echo "local-sanity evidence cannot write the checked-in performance baseline area" >&2
  exit 1
fi
# Docker hosts may remap container root to an unprivileged host identity. Create
# the synthetic-only report as the runner, grant write-only access during the
# isolated container run, and restore owner-only permissions in the EXIT trap.
: > "$REPORT_PATH"
chmod 0622 "$REPORT_PATH"

docker run --rm \
  --network none \
  --read-only \
  --cap-drop ALL \
  --security-opt no-new-privileges \
  --pids-limit 512 \
  --tmpfs /tmp:rw,exec,nosuid,nodev,size=2g \
  --mount "type=bind,src=${ROOT_DIR},dst=/workspace,readonly" \
  --mount "type=bind,src=${STAGING},dst=/models/distilbert,readonly" \
  --mount "type=bind,src=${REPORT_DIR},dst=/reports" \
  -e GITHUB_ACTIONS="${GITHUB_ACTIONS:-false}" \
  -e RUNNER_NAME="${RUNNER_NAME:-local-sanity}" \
  -e HS_PII_G0_MODEL_DIR=/models/distilbert \
  -e HS_PII_G0_REPORT=/reports/distilbert-baseline.json \
  -e HS_PII_G0_GIT_COMMIT="$GIT_COMMIT" \
  -e HS_PII_G0_MODEL_REVISION="$MODEL_REVISION" \
  -e HS_PII_G0_ARTIFACT_SHA256="$MODEL_SHA256" \
  -e HS_PII_G0_ARTIFACTS_JSON="$ARTIFACTS_JSON" \
  -e HS_PII_G0_CONTAINER_IMAGE="$IMAGE_ID" \
  -e HS_PII_G0_HARDWARE_PROFILE="${HS_PII_G0_HARDWARE_PROFILE:-docker-functional}" \
  -e HS_PII_G0_EVIDENCE_AUTHORITY="$AUTHORITY" \
  -e HS_PII_G0_AUTHORITATIVE="$AUTHORITATIVE" \
  "$IMAGE_TAG" ./internal/pii/ner -run '^TestDistilBERTTypedCorpusBaseline$' -count=1 -v -timeout=15m

echo "G0 DistilBERT baseline written to ${REPORT_PATH}"
