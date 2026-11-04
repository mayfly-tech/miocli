#!/bin/sh
set -eu

root="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
export MIOCLI_REPOSITORY="example/miocli"
export MIOCLI_INSTALL_DIR="$work/install with spaces"
export HOME="$work/home"
mkdir -p "$work/mock" "$HOME"
case "$(uname -s)" in Darwin) os=darwin ;; Linux) os=linux ;; *) exit 1 ;; esac
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; *) arch=amd64 ;; esac
name="miocli_1.2.3_${os}_${arch}"
mkdir -p "$work/package/$name"
printf 'new-cli\n' > "$work/package/$name/miocli"
tar -czf "$work/$name.tar.gz" -C "$work/package" "$name"
if command -v sha256sum >/dev/null 2>&1; then
  hash="$(sha256sum "$work/$name.tar.gz" | awk '{print $1}')"
else
  hash="$(shasum -a 256 "$work/$name.tar.gz" | awk '{print $1}')"
fi
printf '%s  %s.tar.gz\n' "$hash" "$name" > "$work/checksum"
export INSTALLER_TEST_ARCHIVE="$work/$name.tar.gz"
export INSTALLER_TEST_CHECKSUM="$work/checksum"
export INSTALLER_TEST_LOG="$work/requests"
export INSTALLER_TEST_SCENARIO=""
cat > "$work/mock/curl" <<'MOCK'
#!/bin/sh
set -eu
url=""
out=""
connect=""
limit=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift 2 ;;
    --connect-timeout) connect="$2"; shift 2 ;;
    --max-time) limit="$2"; shift 2 ;;
    --proto|--proto-redir|--max-redirs|--write-out) shift 2 ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
printf '%s\n' "$url" >> "$INSTALLER_TEST_LOG"
case "$url" in
  https://github.com/*) source=github ;;
  https://s3.usemayfly.tech/public/*) source=s3 ;;
  *) printf 'Unexpected source: %s\n' "$url" >&2; exit 1 ;;
esac
case "$source:$url" in
  github:*/releases/latest) [ "$connect:$limit" = 5:15 ] ;;
  github:*) [ "$connect:$limit" = 8:30 ] ;;
  s3:*.tar.gz) [ "$connect:$limit" = 15:180 ] ;;
  s3:*) [ "$connect:$limit" = 15:60 ] ;;
esac
case "$INSTALLER_TEST_SCENARIO:$source:$url" in
  latest:github:*/releases/latest) exit 28 ;;
  archive:github:*.tar.gz) printf partial > "$out"; exit 7 ;;
  checksum:github:*.sha256) exit 22 ;;
  trust:github:*) exit 60 ;;
  cancelled:github:*) exit 130 ;;
  both:*:*) exit 7 ;;
  badlatest:github:*/releases/latest) printf 'https://evil.test/tag/v1.2.3'; exit 0 ;;
  lag:github:*/releases/latest) printf 'https://github.com/example/miocli/releases/tag/v1.2.4'; exit 0 ;;
  lag:*:*/v1.2.4/*) exit 22 ;;
esac
case "$url" in
  */releases/latest) printf 'https://github.com/example/miocli/releases/tag/v1.2.3' ;;
  */stable.txt) printf 'v1.2.3\n' > "$out" ;;
  *.sha256) cp "$INSTALLER_TEST_CHECKSUM" "$out" ;;
  *.tar.gz) cp "$INSTALLER_TEST_ARCHIVE" "$out" ;;
  *) printf 'Unexpected URL: %s\n' "$url" >&2; exit 1 ;;
esac
MOCK
chmod 700 "$work/mock/curl"
export PATH="$work/mock:$PATH"
sh "$root/install.sh" --help > "$work/output"
env -u HOME -u MIOCLI_INSTALL_DIR sh "$root/install.sh" --help > "$work/output"
sh "$root/install.sh" --non-interactive > "$work/output"
[ "$(wc -l < "$INSTALLER_TEST_LOG" | tr -d ' ')" -eq 3 ]
! grep -q s3.usemayfly "$INSTALLER_TEST_LOG"
[ "$(cat "$MIOCLI_INSTALL_DIR/miocli")" = new-cli ]
grep -q 'PATH was not modified' "$work/output"
grep -Fq '"$PATH"' "$work/output"
[ ! -e "$HOME/.profile" ] && [ ! -e "$HOME/.zshrc" ]
printf 'old-cli\n' > "$MIOCLI_INSTALL_DIR/miocli"
if sh "$root/install.sh" v1.2.3 --non-interactive > "$work/output" 2>&1; then
  printf 'Installer overwrote without confirmation\n' >&2
  exit 1
fi
grep -q 'requires --yes' "$work/output"
[ "$(cat "$MIOCLI_INSTALL_DIR/miocli")" = old-cli ]
sh "$root/install.sh" --version 1.2.3 --yes --non-interactive > "$work/output"
[ "$(cat "$MIOCLI_INSTALL_DIR/miocli")" = new-cli ]
printf 'old-cli\n' > "$MIOCLI_INSTALL_DIR/miocli"
printf '%064d  %s.tar.gz\n' 0 "$name" > "$work/checksum"
if sh "$root/install.sh" v1.2.3 --yes --non-interactive > "$work/output" 2>&1; then
  printf 'Installer accepted bad checksum\n' >&2
  exit 1
fi
grep -q 'checksum verification failed' "$work/output"
[ "$(cat "$MIOCLI_INSTALL_DIR/miocli")" = old-cli ]
if sh "$root/install.sh" v1.2.3 --setup --non-interactive > "$work/output" 2>&1; then exit 1; fi
grep -q 'requires an interactive terminal' "$work/output"
if sh "$root/install.sh" --install-dir relative --non-interactive > "$work/output" 2>&1; then exit 1; fi
printf '%s  %s.tar.gz\n' "$hash" "$name" > "$work/checksum"
for scenario in latest archive checksum; do
  export INSTALLER_TEST_SCENARIO="$scenario"
  : > "$INSTALLER_TEST_LOG"
  sh "$root/install.sh" --yes --non-interactive > "$work/output" 2>&1
  grep -q 'from S3' "$work/output"
  [ "$(cat "$MIOCLI_INSTALL_DIR/miocli")" = new-cli ]
  [ "$(grep -c 's3.usemayfly.*tar.gz' "$INSTALLER_TEST_LOG")" -eq 2 ]
  if [ "$scenario" = latest ]; then
    [ "$(grep -c github.com "$INSTALLER_TEST_LOG")" -eq 1 ]
    grep -q stable.txt "$INSTALLER_TEST_LOG"
  else
    ! grep -q stable.txt "$INSTALLER_TEST_LOG"
  fi
done
for scenario in trust cancelled badlatest both lag; do
  export INSTALLER_TEST_SCENARIO="$scenario"
  : > "$INSTALLER_TEST_LOG"
  printf 'old-cli\n' > "$MIOCLI_INSTALL_DIR/miocli"
  if sh "$root/install.sh" --yes --non-interactive > "$work/output" 2>&1; then
    printf 'Installer accepted failure scenario %s\n' "$scenario" >&2; exit 1
  fi
  [ "$(cat "$MIOCLI_INSTALL_DIR/miocli")" = old-cli ]
  case "$scenario" in trust|cancelled|badlatest) ! grep -q s3.usemayfly "$INSTALLER_TEST_LOG" ;; esac
  if [ "$scenario" = lag ]; then
    ! grep -q stable.txt "$INSTALLER_TEST_LOG"
    ! grep -q '/v1.2.3/' "$INSTALLER_TEST_LOG"
  fi
done
export INSTALLER_TEST_SCENARIO=archive
: > "$INSTALLER_TEST_LOG"
sh "$root/install.sh" --version v1.2.3 --yes --non-interactive > "$work/output" 2>&1
! grep -Eq 'releases/latest|stable.txt' "$INSTALLER_TEST_LOG"
export INSTALLER_TEST_SCENARIO=""
: > "$INSTALLER_TEST_LOG"
printf '%064d  %s.tar.gz\n' 0 "$name" > "$work/checksum"
if sh "$root/install.sh" v1.2.3 --yes --non-interactive > "$work/output" 2>&1; then exit 1; fi
! grep -q s3.usemayfly "$INSTALLER_TEST_LOG"
printf 'Shell installer behavior tests passed.\n'
