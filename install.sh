#!/bin/sh
set -eu

REPOSITORY="${MIOCLI_REPOSITORY:-__MIOCLI_REPOSITORY__}"
BIN_NAME="miocli"
S3_ROOT="https://s3.usemayfly.tech/public"

fail() {
  printf 'miocli installer: %s\n' "$*" >&2
  exit 1
}

usage() {
  printf 'Usage: install.sh [VERSION] [--version VERSION] [--install-dir DIR] [--yes] [--non-interactive] [--setup]\n'
  printf 'Install only miocli. Existing Mihomo/Clash Verge cores are not modified.\n'
  printf 'MIOCLI_INSTALL_DIR overrides the default ~/.local/bin. PATH is never modified.\n'
  printf '%s\n' '--yes confirms CLI replacement only, not Mihomo installation.'
  printf 'GitHub is tried first; network/HTTP failure retries the same version from domestic S3.\n'
}

version=""
install_dir="${MIOCLI_INSTALL_DIR:-}"
yes=0
non_interactive=0
setup=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --help|-h) usage; exit 0 ;;
    --version|--install-dir)
      [ "$#" -ge 2 ] && [ -n "$2" ] || fail "$1 requires a value"
      case "$1" in --version) version="$2" ;; --install-dir) install_dir="$2" ;; esac
      shift 2 ;;
    --yes|-y) yes=1; shift ;;
    --non-interactive) non_interactive=1; shift ;;
    --setup) setup=1; shift ;;
    -*) fail "unknown option: $1 (use --help)" ;;
    *) [ -z "$version" ] || fail "only one version may be specified"; version="$1"; shift ;;
  esac
done
if [ -z "$install_dir" ]; then
  [ -n "${HOME:-}" ] || fail "HOME is not set; specify --install-dir or MIOCLI_INSTALL_DIR"
  install_dir="$HOME/.local/bin"
fi
interactive=0
if [ "$non_interactive" -eq 0 ] && [ -t 0 ] && [ -t 1 ]; then interactive=1; fi
[ "$setup" -eq 0 ] || [ "$interactive" -eq 1 ] || fail "--setup requires an interactive terminal; install first, then run miocli setup"
case "$install_dir" in /*) ;; *) fail "installation directory must be an absolute path" ;; esac
[ ! -d "$install_dir/$BIN_NAME" ] || fail "install target is a directory"
if [ -e "$install_dir/$BIN_NAME" ] || [ -L "$install_dir/$BIN_NAME" ]; then
  printf 'Existing CLI: %s\n' "$install_dir/$BIN_NAME"
  if [ "$yes" -ne 1 ]; then
    [ "$interactive" -eq 1 ] || fail "replacement requires --yes in non-interactive mode"
    printf 'Replace this CLI binary? [y/N] '
    IFS= read -r answer || fail "could not read confirmation"
    case "$answer" in y|Y|yes|YES) ;; *) fail "installation cancelled; existing CLI unchanged" ;; esac
  fi
fi

case "$REPOSITORY" in
  */*) owner="${REPOSITORY%%/*}"; repo="${REPOSITORY#*/}" ;;
  *) fail "installer repository is not configured" ;;
esac
[ -n "$owner" ] && [ -n "$repo" ] || fail "invalid GitHub repository name"
case "$owner$repo" in
  *[!A-Za-z0-9_.-]*) fail "invalid GitHub repository name" ;;
esac
case "$repo" in
  ""|*/*) fail "invalid GitHub repository name" ;;
esac

for tool in curl uname tar awk cat grep sed mktemp mkdir install mv rm; do
  command -v "$tool" >/dev/null 2>&1 || fail "$tool is required"
done

os="$(uname -s)"
case "$os" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *) fail "unsupported operating system: $os" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) fail "unsupported architecture: $arch" ;;
esac

tmp_dir="$(mktemp -d)"
staged=""
cleanup() {
  [ -z "$staged" ] || rm -f "$staged"
  rm -rf "$tmp_dir"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' HUP TERM

# Only curl availability failures may switch sources; trust and policy errors stop.
fetch() {
  source_name="$1"; request_limit="$2"; request_url="$3"; request_out="$4"
  shift 4
  connect_limit=8
  redirects=5
  if [ "$source_name" = S3 ]; then connect_limit=15; redirects=0; fi
  if curl --proto '=https' --proto-redir '=https' --max-redirs "$redirects" \
      --connect-timeout "$connect_limit" --max-time "$request_limit" -fsSL \
      "$@" "$request_url" -o "$request_out"; then
    return 0
  else
    curl_status=$?
    printf '%s request failed (curl exit %s).\n' "$source_name" "$curl_status" >&2
    case "$curl_status" in
      5|6|7|18|22|28|35|52|55|56) return 2 ;;
      *) return 1 ;;
    esac
  fi
}

source=GitHub
if [ -z "$version" ]; then
  # The canonical release redirect supplies the tag without a JSON parser dependency.
  if fetch GitHub 15 "https://github.com/$REPOSITORY/releases/latest" /dev/null \
      --connect-timeout 5 --head --write-out '%{url_effective}' > "$tmp_dir/latest"; then
    latest_url="$(cat "$tmp_dir/latest")"
    case "$latest_url" in
      "https://github.com/$REPOSITORY/releases/tag/"*) version="${latest_url##*/}" ;;
      *) fail "GitHub latest release returned an invalid version destination" ;;
    esac
  else
    status=$?
    [ "$status" -eq 2 ] || fail "GitHub latest request rejected; no fallback"
    printf 'GitHub unavailable; checking domestic S3 stable version.\n' >&2
    source=S3
    fetch S3 60 "$S3_ROOT/miocli/stable.txt" "$tmp_dir/latest" \
      || fail "GitHub and S3 version lookup failed"
    version="$(cat "$tmp_dir/latest")"
    printf '%s\n' "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' \
      || fail "S3 stable version is invalid"
  fi
fi
case "$version" in v*) ;; *) version="v$version" ;; esac
case "$version" in *[!A-Za-z0-9.+-]*) fail "version contains invalid characters" ;; esac
printf '%s\n' "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+([+-][A-Za-z0-9.-]+)?$' \
  || fail "version must be a semantic version such as v1.2.3"
version_number="${version#v}"
archive="miocli_${version_number}_${os}_${arch}.tar.gz"
printf 'Install miocli %s (%s/%s) to %s\n' "$version" "$os" "$arch" "$install_dir/$BIN_NAME"
mkdir -p "$install_dir" || fail "could not create install directory: $install_dir"
[ -w "$install_dir" ] || fail "install directory is not writable: $install_dir"
fetch_pair() {
  pair_source="$1"
  pair_limit=30
  base_url="https://github.com/$REPOSITORY/releases/download/$version"
  if [ "$pair_source" = S3 ]; then
    base_url="$S3_ROOT/miocli/releases/$version"
    pair_limit=180
  fi
  fetch "$pair_source" "$pair_limit" "$base_url/$archive" "$tmp_dir/$archive" || return $?
  checksum_limit=30
  [ "$pair_source" != S3 ] || checksum_limit=60
  fetch "$pair_source" "$checksum_limit" "$base_url/$archive.sha256" "$tmp_dir/$archive.sha256"
}
if fetch_pair "$source"; then
  :
else
  status=$?
  [ "$source" = GitHub ] && [ "$status" -eq 2 ] || fail "$source download failed; existing CLI unchanged"
  printf 'GitHub unavailable; retrying miocli %s archive and checksum from domestic S3.\n' "$version" >&2
  rm -f "$tmp_dir/$archive" "$tmp_dir/$archive.sha256"
  source=S3
  fetch_pair S3 || fail "GitHub and S3 downloads failed for $version; existing CLI unchanged"
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp_dir/$archive" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "$tmp_dir/$archive" | awk '{print $1}')"
else
  fail "sha256sum or shasum is required to verify the download"
fi
expected="$(awk -v file="$archive" 'NF { lines++; if (NF == 2 && $2 == file) hash = tolower($1) }
  END { if (lines == 1) print hash }' "$tmp_dir/$archive.sha256")"
case "$expected" in
  *[!0-9a-fA-F]*|'') fail "release checksum file is invalid" ;;
esac
[ "${#expected}" -eq 64 ] || fail "release checksum file is invalid"
[ "$actual" = "$expected" ] || fail "checksum verification failed"

tar -xzf "$tmp_dir/$archive" -C "$tmp_dir" || fail "could not extract $archive"
package_dir="$tmp_dir/miocli_${version_number}_${os}_${arch}"
[ -f "$package_dir/$BIN_NAME" ] || fail "release archive does not contain $BIN_NAME"

staged="$(mktemp "$install_dir/.miocli-install-XXXXXX")" || fail "could not stage CLI in $install_dir"
install -m 0755 "$package_dir/$BIN_NAME" "$staged" \
  || fail "could not stage $BIN_NAME"
mv -f "$staged" "$install_dir/$BIN_NAME" \
  || fail "could not install $BIN_NAME to $install_dir"

printf 'Installed %s %s from %s to %s\n' "$BIN_NAME" "$version" "$source" "$install_dir/$BIN_NAME"
case ":${PATH:-}:" in
  *":$install_dir:"*) ;;
  *)
    printf 'PATH was not modified. Add this directory using your shell configuration:\n  %s\n' "$install_dir"
    printf 'For the current POSIX shell, run (or adjust for your shell):\n'
    escaped_dir="$(printf '%s' "$install_dir" | sed "s/'/'\\\\''/g")"
    printf "  export PATH='%s':\"\$PATH\"\n" "$escaped_dir"
    ;;
esac
printf 'Only miocli was installed. Existing Clash Verge/Mihomo users do not need a second core.\n'
printf 'Next: "%s" setup\nCheck: "%s" doctor\n' "$install_dir/$BIN_NAME" "$install_dir/$BIN_NAME"
if [ "$interactive" -eq 1 ] && [ "$setup" -eq 0 ]; then
  printf 'Run connection setup now? [y/N] '
  IFS= read -r answer || fail "could not read setup choice (CLI is already installed)"
  case "$answer" in y|Y|yes|YES) setup=1 ;; esac
fi
if [ "$setup" -eq 1 ]; then
  "$install_dir/$BIN_NAME" setup || fail "CLI installed, but setup did not complete; rerun the setup command above"
fi
