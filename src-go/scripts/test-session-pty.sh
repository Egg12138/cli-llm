#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
module_root=$(cd -- "$script_dir/.." && pwd)

cd "$module_root"
go test ./internal/session/acceptance -run '^TestSessionPTY$' -count=1 -v
