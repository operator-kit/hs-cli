#!/usr/bin/env bash
# prepare-pii-model.sh — Download PII model + ONNX Runtime, create per-platform bundles.
#
# Usage: ./scripts/prepare-pii-model.sh [output_dir]
#
# Output: per-platform tarballs + SHA-256 hashes ready for GitHub Release upload.
set -euo pipefail

VERSION="1.0.0"
MODEL_REPO="Xenova/distilbert-base-multilingual-cased-ner-hrl"
ORT_VERSION="1.23.0"
OUTDIR="${1:-dist/pii-model}"

# HuggingFace ONNX model files (universal)
HF_BASE="https://huggingface.co/${MODEL_REPO}/resolve/main"
MODEL_FILES=(
  "onnx/model_quantized.onnx"
  "tokenizer.json"
  "config.json"
)

# ONNX Runtime CPU releases per platform
declare -A ORT_URLS=(
  ["linux-amd64"]="https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-linux-x64-${ORT_VERSION}.tgz"
  ["linux-arm64"]="https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-linux-aarch64-${ORT_VERSION}.tgz"
  ["darwin-amd64"]="https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-osx-x86_64-${ORT_VERSION}.tgz"
  ["darwin-arm64"]="https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-osx-arm64-${ORT_VERSION}.tgz"
  ["windows-amd64"]="https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-win-x64-${ORT_VERSION}.zip"
  ["windows-arm64"]="https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-win-arm64-${ORT_VERSION}.zip"
)

# Runtime lib filenames per platform (target name in bundle)
declare -A ORT_LIBS=(
  ["linux-amd64"]="libonnxruntime.so"
  ["linux-arm64"]="libonnxruntime.so"
  ["darwin-amd64"]="libonnxruntime.dylib"
  ["darwin-arm64"]="libonnxruntime.dylib"
  ["windows-amd64"]="onnxruntime.dll"
  ["windows-arm64"]="onnxruntime.dll"
)

# Real (versioned) filenames inside ORT archives — avoids symlink issues on Windows.
# Linux/macOS tarballs ship symlinks (e.g. .so → .so.1 → .so.1.23.0); we extract
# the versioned file directly and rename it to the canonical name.
declare -A ORT_REAL_LIBS=(
  ["linux-amd64"]="libonnxruntime.so.${ORT_VERSION}"
  ["linux-arm64"]="libonnxruntime.so.${ORT_VERSION}"
  ["darwin-amd64"]="libonnxruntime.${ORT_VERSION}.dylib"
  ["darwin-arm64"]="libonnxruntime.${ORT_VERSION}.dylib"
  ["windows-amd64"]="onnxruntime.dll"
  ["windows-arm64"]="onnxruntime.dll"
)

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "==> Downloading model files from HuggingFace..."
MODELDIR="$TMPDIR/model"
mkdir -p "$MODELDIR"
for f in "${MODEL_FILES[@]}"; do
  out="$MODELDIR/$(basename "$f")"
  echo "  $f -> $(basename "$out")"
  curl -fsSL "${HF_BASE}/${f}" -o "$out"
done

mkdir -p "$OUTDIR"

for platform in "${!ORT_URLS[@]}"; do
  echo "==> Building bundle for $platform..."
  url="${ORT_URLS[$platform]}"
  lib="${ORT_LIBS[$platform]}"

  # Download ORT
  ort_file="$TMPDIR/ort-${platform}"
  curl -fsSL "$url" -o "$ort_file"

  # Extract the runtime lib (use versioned filename to avoid symlink issues on Windows)
  bundledir="$TMPDIR/bundle-${platform}"
  mkdir -p "$bundledir"
  real_lib="${ORT_REAL_LIBS[$platform]}"

  if [[ "$url" == *.zip ]]; then
    unzip -q -j "$ort_file" "**/lib/${lib}" -d "$bundledir" 2>/dev/null || \
    unzip -q -j "$ort_file" "**/${lib}" -d "$bundledir"
  else
    # Extract the real (versioned) file, not the symlink
    ort_extract="$TMPDIR/ort-extract-${platform}"
    mkdir -p "$ort_extract"
    tar xzf "$ort_file" -C "$ort_extract" --wildcards "**/lib/${real_lib}" 2>/dev/null || \
    tar xzf "$ort_file" -C "$ort_extract" --wildcards "**/${real_lib}"
    find "$ort_extract" -name "$real_lib" -exec cp {} "$bundledir/${lib}" \;
    rm -rf "$ort_extract"
  fi

  if [[ ! -f "$bundledir/${lib}" ]]; then
    echo "  ERROR: failed to extract ${lib} for ${platform}"
    exit 1
  fi

  # Copy model files
  cp "$MODELDIR/model_quantized.onnx" "$bundledir/"
  cp "$MODELDIR/tokenizer.json" "$bundledir/"
  cp "$MODELDIR/config.json" "$bundledir/"

  # Create tarball
  tarball="$OUTDIR/pii-model-${VERSION}-${platform}.tar.gz"
  tar czf "$tarball" -C "$bundledir" .
  sha256sum "$tarball" | awk '{print $1}' > "${tarball}.sha256"

  echo "  Created: $(basename "$tarball") ($(du -h "$tarball" | awk '{print $1}'))"
  echo "  SHA-256: $(cat "${tarball}.sha256")"
done

echo ""
echo "=== Done ==="
echo "Upload assets from $OUTDIR to GitHub Release: pii-model-v${VERSION}"
echo ""
echo "Go constants for embedding:"
for platform in "${!ORT_URLS[@]}"; do
  sha=$(cat "$OUTDIR/pii-model-${VERSION}-${platform}.tar.gz.sha256")
  govar=$(echo "$platform" | tr '-' '_' | tr '[:lower:]' '[:upper:]')
  echo "  SHA256_${govar} = \"${sha}\""
done
