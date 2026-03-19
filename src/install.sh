#!/usr/bin/env bash
set -euo pipefail

dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$dir"

echo "Compile to /usr/local/bin/maggus"

go build -o /usr/local/bin/maggus .
