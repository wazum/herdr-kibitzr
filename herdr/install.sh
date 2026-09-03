#!/usr/bin/env bash
# Runs on `herdr plugin install`. Puts the hook binary in the plugin root,
# preferring the one published for this release so no Go toolchain is needed.
set -euo pipefail

cd "$(dirname "$0")/.."
mkdir -p bin

repository="wazum/herdr-kibitzr"
version="$(sed -n 's/^version = "\(.*\)"/\1/p' herdr-plugin.toml | head -1)"

case "$(uname -s)" in
	Darwin) goos=darwin ;;
	Linux) goos=linux ;;
	*) goos="" ;;
esac
case "$(uname -m)" in
	arm64 | aarch64) goarch=arm64 ;;
	x86_64 | amd64) goarch=amd64 ;;
	*) goarch="" ;;
esac

download() {
	[[ -n $goos && -n $goarch && -n $version ]] || return 1
	command -v curl >/dev/null || return 1

	local archive="herdr-kibitzr_${goos}_${goarch}.tar.gz"
	local base="https://github.com/${repository}/releases/download/v${version}"
	local work
	work="$(mktemp -d)"
	trap 'rm -rf "$work"' RETURN

	curl -fsL "${base}/${archive}" -o "${work}/${archive}" || return 1
	curl -fsL "${base}/${archive}.sha256" -o "${work}/${archive}.sha256" || return 1
	(cd "$work" && shasum -a 256 -c "${archive}.sha256" >/dev/null 2>&1) || return 1

	tar xzf "${work}/${archive}" -C "$work" || return 1
	mv "${work}/herdr-kibitzr" bin/herdr-kibitzr
	chmod +x bin/herdr-kibitzr
}

build() {
	command -v go >/dev/null || {
		echo "kibitzr: no released binary for ${goos:-this system}/${goarch:-} and no Go to build one" >&2
		return 1
	}
	go build -trimpath -ldflags "-s -w" -o bin/herdr-kibitzr ./cmd/herdr-kibitzr
}

download || build
