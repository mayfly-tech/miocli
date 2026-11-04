#!/bin/sh
set -eu

repo_root=$(git rev-parse --show-toplevel 2>/dev/null) || {
    echo "install-hooks: run this script from inside a Git working tree" >&2
    exit 1
}

hook="$repo_root/.githooks/commit-msg"
if [ ! -x "$hook" ]; then
    echo "install-hooks: commit-msg hook is missing or not executable: $hook" >&2
    exit 1
fi

if configured_hooks_path=$(git -C "$repo_root" config --local --get core.hooksPath) &&
    [ "$configured_hooks_path" != ".githooks" ]
then
    echo "install-hooks: refusing to replace existing core.hooksPath: $configured_hooks_path" >&2
    exit 1
fi

git -C "$repo_root" config --local core.hooksPath .githooks
echo "Installed repository hooks from .githooks."
