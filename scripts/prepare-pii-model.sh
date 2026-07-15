#!/usr/bin/env bash
# Build reproducible, provenance-locked PII model bundles.
# Usage: ./scripts/prepare-pii-model.sh [output_dir]
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_LOCK="${ROOT_DIR}/scripts/pii-model-sources.json"
OUTDIR="${1:-${ROOT_DIR}/dist/pii-model}"

for command in curl gzip jq sha256sum tar; do
  command -v "$command" >/dev/null || { echo "missing required command: $command" >&2; exit 1; }
done

LOCK_VERSION="$(jq -r '.bundle_version' "$SOURCE_LOCK")"
VERSION="${PII_MODEL_VERSION:-$LOCK_VERSION}"
if [[ "$VERSION" != "$LOCK_VERSION" ]]; then
  echo "PII_MODEL_VERSION $VERSION does not match source lock $LOCK_VERSION" >&2
  exit 1
fi

MODEL_REPO="$(jq -r '.model.repository' "$SOURCE_LOCK")"
MODEL_REVISION="$(jq -r '.model.revision' "$SOURCE_LOCK")"
ORT_VERSION="$(jq -r '.onnx_runtime.version' "$SOURCE_LOCK")"
MAX_ARCHIVE_SIZE="$(jq -r '.limits.max_archive_size' "$SOURCE_LOCK")"
MAX_EXPANDED_SIZE="$(jq -r '.limits.max_expanded_size' "$SOURCE_LOCK")"
HF_BASE="https://huggingface.co/${MODEL_REPO}/resolve/${MODEL_REVISION}"

TMPDIR_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMPDIR_ROOT"' EXIT
MODELDIR="${TMPDIR_ROOT}/model"
FRAGMENTS="${TMPDIR_ROOT}/bundles.jsonl"
mkdir -p "$MODELDIR" "$OUTDIR"
: > "$FRAGMENTS"

sha256_file() {
  sha256sum "$1" | awk '{print $1}'
}

file_size() {
  wc -c < "$1" | tr -d '[:space:]'
}

verify_file() {
  local file="$1" expected_hash="$2" expected_size="$3" label="$4"
  local actual_hash actual_size
  actual_hash="$(sha256_file "$file")"
  actual_size="$(file_size "$file")"
  if [[ "$actual_hash" != "$expected_hash" || "$actual_size" != "$expected_size" ]]; then
    echo "$label failed provenance check" >&2
    echo "  size: $actual_size (want $expected_size)" >&2
    echo "  sha256: $actual_hash (want $expected_hash)" >&2
    exit 1
  fi
}

download_verified() {
  local url="$1" destination="$2" expected_hash="$3" expected_size="$4" label="$5"
  curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 --retry 3 "$url" -o "$destination"
  verify_file "$destination" "$expected_hash" "$expected_size" "$label"
}

echo "==> Downloading model inputs at immutable revision ${MODEL_REVISION}"
while IFS= read -r file; do
  source_path="$(jq -r '.source_path' <<<"$file")"
  name="$(jq -r '.name' <<<"$file")"
  expected_hash="$(jq -r '.sha256' <<<"$file")"
  expected_size="$(jq -r '.size' <<<"$file")"
  download_verified "${HF_BASE}/${source_path}" "${MODELDIR}/${name}" "$expected_hash" "$expected_size" "model input ${name}"
done < <(jq -c '.model.files[]' "$SOURCE_LOCK")

while IFS= read -r target; do
  goos="$(jq -r '.goos' <<<"$target")"
  goarch="$(jq -r '.goarch' <<<"$target")"
  platform="${goos}-${goarch}"
  runtime_url="$(jq -r '.url' <<<"$target")"
  runtime_hash="$(jq -r '.sha256' <<<"$target")"
  runtime_size="$(jq -r '.size' <<<"$target")"
  library_glob="$(jq -r '.library_glob' <<<"$target")"
  library_name="$(jq -r '.library_name' <<<"$target")"
  ort_archive="${TMPDIR_ROOT}/ort-${platform}.tgz"
  bundle_dir="${TMPDIR_ROOT}/bundle-${platform}"
  mkdir -p "$bundle_dir"

  echo "==> Building ${platform}"
  download_verified "$runtime_url" "$ort_archive" "$runtime_hash" "$runtime_size" "ONNX Runtime ${platform}"
  tar -xOf "$ort_archive" --wildcards "$library_glob" > "${bundle_dir}/${library_name}"
  [[ -s "${bundle_dir}/${library_name}" ]] || { echo "runtime library extraction failed for ${platform}" >&2; exit 1; }

  cp "${MODELDIR}/config.json" "${bundle_dir}/config.json"
  cp "${MODELDIR}/model_quantized.onnx" "${bundle_dir}/model_quantized.onnx"
  cp "${MODELDIR}/tokenizer.json" "${bundle_dir}/tokenizer.json"

  tarball_name="pii-model-${VERSION}-${platform}.tar.gz"
  tarball="${OUTDIR}/${tarball_name}"
  LC_ALL=C tar --sort=name --format=ustar --mtime='UTC 1970-01-01' --owner=0 --group=0 --numeric-owner --mode=0644 -cf - -C "$bundle_dir" config.json model_quantized.onnx tokenizer.json "$library_name" | gzip -n -9 > "$tarball"

  archive_hash="$(sha256_file "$tarball")"
  archive_size="$(file_size "$tarball")"
  if (( archive_size > MAX_ARCHIVE_SIZE )); then
    echo "archive ${platform} exceeds ${MAX_ARCHIVE_SIZE} bytes" >&2
    exit 1
  fi
  printf '%s  %s\n' "$archive_hash" "$tarball_name" > "${tarball}.sha256"

  config_hash="$(sha256_file "${bundle_dir}/config.json")"
  config_size="$(file_size "${bundle_dir}/config.json")"
  model_hash="$(sha256_file "${bundle_dir}/model_quantized.onnx")"
  model_size="$(file_size "${bundle_dir}/model_quantized.onnx")"
  tokenizer_hash="$(sha256_file "${bundle_dir}/tokenizer.json")"
  tokenizer_size="$(file_size "${bundle_dir}/tokenizer.json")"
  library_hash="$(sha256_file "${bundle_dir}/${library_name}")"
  library_size="$(file_size "${bundle_dir}/${library_name}")"
  expanded_size=$((config_size + model_size + tokenizer_size + library_size))
  if (( expanded_size > MAX_EXPANDED_SIZE )); then
    echo "expanded bundle ${platform} exceeds ${MAX_EXPANDED_SIZE} bytes" >&2
    exit 1
  fi

  files_json="$(jq -cn \
    --arg config_hash "$config_hash" --argjson config_size "$config_size" \
    --arg model_hash "$model_hash" --argjson model_size "$model_size" \
    --arg tokenizer_hash "$tokenizer_hash" --argjson tokenizer_size "$tokenizer_size" \
    --arg library_name "$library_name" --arg library_hash "$library_hash" --argjson library_size "$library_size" \
    '[
      {name:"config.json", sha256:$config_hash, size:$config_size},
      {name:"model_quantized.onnx", sha256:$model_hash, size:$model_size},
      {name:"tokenizer.json", sha256:$tokenizer_hash, size:$tokenizer_size},
      {name:$library_name, sha256:$library_hash, size:$library_size}
    ]')"

  release_url="https://github.com/operator-kit/hs-cli/releases/download/pii-model-v${VERSION}/${tarball_name}"
  jq -cn \
    --arg goos "$goos" --arg goarch "$goarch" \
    --arg archive_url "$release_url" --arg archive_filename "$tarball_name" --arg archive_hash "$archive_hash" \
    --argjson archive_size "$archive_size" --argjson max_archive_size "$MAX_ARCHIVE_SIZE" \
    --arg runtime_url "$runtime_url" --arg runtime_hash "$runtime_hash" --argjson runtime_size "$runtime_size" \
    --arg runtime_library "$library_name" --argjson max_expanded_size "$MAX_EXPANDED_SIZE" \
    --argjson files "$files_json" \
    '{
      goos:$goos, goarch:$goarch,
      archive:{url:$archive_url, filename:$archive_filename, sha256:$archive_hash, size:$archive_size, max_size:$max_archive_size},
      runtime_source:{url:$runtime_url, sha256:$runtime_hash, size:$runtime_size},
      runtime_library:$runtime_library, max_expanded_size:$max_expanded_size, files:$files
    }' >> "$FRAGMENTS"
done < <(jq -c '.onnx_runtime.platforms[]' "$SOURCE_LOCK")

jq -s '.' "$FRAGMENTS" > "${TMPDIR_ROOT}/bundles.json"
jq -n \
  --arg model_version "$VERSION" --arg model_repository "$MODEL_REPO" --arg model_revision "$MODEL_REVISION" \
  --arg onnx_runtime_version "$ORT_VERSION" --slurpfile bundles "${TMPDIR_ROOT}/bundles.json" \
  '{
    schema_version:1, model_version:$model_version, model_repository:$model_repository,
    model_revision:$model_revision, onnx_runtime_version:$onnx_runtime_version, bundles:$bundles[0]
  }' > "${OUTDIR}/trusted_manifest.json"

echo "==> Reproducible bundles and candidate manifest written to ${OUTDIR}"
