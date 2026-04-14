#!/usr/bin/env bash
set -euo pipefail

dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$dir"

echo -e "\033[0;31mStopping all maggus instances...\033[0m"
maggus stop --all 2>/dev/null || true
while pgrep -x maggus > /dev/null 2>&1; do
    sleep 0.2
done

build_time="$(date +%H%M%S)"
echo -e "\033[0;36mCompile to /usr/local/bin/maggus (dev-$build_time)\033[0m"

go build -ldflags "-X github.com/leberkas-org/maggus/cmd.BuildTime=$build_time" -o /tmp/maggus .
sudo mv /tmp/maggus /usr/local/bin/maggus
