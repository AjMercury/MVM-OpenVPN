#!/usr/bin/env bash
set -Eeuo pipefail
root=$(cd -- "$(dirname -- "$0")/../.." && pwd)
for suite in check preflight archive recovery uninstall wizard; do bash "$root/tests/installer/$suite.sh"; done
bash -n "$root/tests/systemd/disposable-vm.sh"
