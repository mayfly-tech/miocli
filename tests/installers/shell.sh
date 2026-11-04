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
cat > "$work/mock/curl" <<'MOCK'
#!/bin/sh
set -eu
url=""
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift 2 ;;
    --connect-timeout|--max-time) shift 2 ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
case "$url" in
  */releases/latest) printf '{"tag_name":"v1.2.3"}\n' ;;
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
printf 'Shell installer behavior tests passed.\n'
