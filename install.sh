#!/bin/sh
set -e

REPO="operator-kit/hs-cli"
BINARY="hs"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# Detect OS
case "$(uname -s)" in
  Linux*)  OS="linux" ;;
  Darwin*) OS="darwin" ;;
  *)       echo "Unsupported OS: $(uname -s)"; exit 1 ;;
esac

# Detect arch
case "$(uname -m)" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64)  ARCH="arm64" ;;
  *)              echo "Unsupported architecture: $(uname -m)"; exit 1 ;;
esac

# Resolve version
if [ -z "$HS_VERSION" ]; then
  HS_VERSION=$(curl -sI "https://github.com/${REPO}/releases/latest" \
    | grep -i "^location:" \
    | sed 's|.*/tag/||' \
    | tr -d '\r\n')

  if [ -z "$HS_VERSION" ]; then
    echo "Error: could not determine latest version. Set HS_VERSION manually."
    exit 1
  fi
fi

VERSION_NUM="${HS_VERSION#v}"
ARCHIVE="hs_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${HS_VERSION}/${ARCHIVE}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${HS_VERSION}/checksums.txt"

echo "Installing ${BINARY} ${HS_VERSION} (${OS}/${ARCH})..."

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# Download archive + checksums
curl -sL "$URL" -o "$TMPDIR/$ARCHIVE"
curl -sL "$CHECKSUMS_URL" -o "$TMPDIR/checksums.txt"

# Verify checksum
cd "$TMPDIR"
if command -v sha256sum >/dev/null 2>&1; then
  grep "$ARCHIVE" checksums.txt | sha256sum -c --quiet
elif command -v shasum >/dev/null 2>&1; then
  grep "$ARCHIVE" checksums.txt | shasum -a 256 -c --quiet
else
  echo "Warning: no sha256sum or shasum found, skipping checksum verification"
fi

# Extract and install
tar xzf "$ARCHIVE" "$BINARY"
install -d "$INSTALL_DIR"
install "$BINARY" "$INSTALL_DIR/$BINARY"

echo "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
"$INSTALL_DIR/$BINARY" version

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    # Detect user's shell rc file
    case "$(basename "$SHELL")" in
      zsh)  RC_FILE="$HOME/.zshrc" ;;
      fish) RC_FILE="$HOME/.config/fish/config.fish" ;;
      *)    RC_FILE="$HOME/.bashrc" ;;
    esac
    echo ""
    echo "WARNING: $INSTALL_DIR is not in your PATH. Run:"
    case "$(basename "$SHELL")" in
      fish)
        echo "  echo 'fish_add_path $INSTALL_DIR' >> $RC_FILE && source $RC_FILE"
        ;;
      *)
        echo "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> $RC_FILE && source $RC_FILE"
        ;;
    esac
    ;;
esac
