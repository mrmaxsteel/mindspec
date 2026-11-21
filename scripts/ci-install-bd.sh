#!/usr/bin/env bash
set -euo pipefail
install_dir=${1:?usage: ci-install-bd.sh directory}
scratch_dir=$(mktemp -d)
archive="$scratch_dir/beads.tar.gz"
curl --fail --location --retry 3 --max-time 120 \
  https://github.com/gastownhall/beads/releases/download/v1.3.4/beads_1.3.4_linux_amd64.tar.gz \
  --output "$archive"
printf '%s  %s\n' 175c26904bde3ff9168d2282ff6b56f17b17f60c22fa5acb3dacc9fb3465303a "$archive" | sha256sum -c -
tar -xzf "$archive" -C "$scratch_dir"
mkdir -p "$install_dir"
install -m 0755 "$scratch_dir/bd" "$install_dir/bd"
"$install_dir/bd" --version | grep -F 1.3.4
printf 'bd-installed: '
"$install_dir/bd" --version
