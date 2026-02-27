#!/usr/bin/env sh
set -eu

REPO="${ZET_SSH_REPOSITORY:-bonheur/zet-ssh-4}"
VERSION="${1:-latest}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
linux|darwin) ;;
*)
  echo "Unsupported OS: $os" >&2
  exit 1
  ;;
esac

case "$arch" in
x86_64|amd64) arch="amd64" ;;
aarch64|arm64) arch="arm64" ;;
*)
  echo "Unsupported architecture: $arch" >&2
  exit 1
  ;;
esac

if [ "$VERSION" = "latest" ]; then
  release_url="https://api.github.com/repos/$REPO/releases/latest"
  VERSION="$(curl -fsSL "$release_url" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
fi

if [ -z "$VERSION" ]; then
  echo "Unable to resolve release version" >&2
  exit 1
fi

asset="zet-${os}-${arch}.tar.gz"
url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

echo "Downloading $url"
curl -fL "$url" -o "$tmp_dir/$asset"
tar -xzf "$tmp_dir/$asset" -C "$tmp_dir"

bin_dir="${HOME}/.local/bin"
mkdir -p "$bin_dir"

bin_name="zet"
if [ "$os" = "windows" ]; then
  bin_name="zet.exe"
fi

install "$tmp_dir/$bin_name" "$bin_dir/$bin_name"
echo "Installed $bin_name to $bin_dir"
echo "Ensure $bin_dir is in PATH"
