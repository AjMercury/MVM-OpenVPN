#!/usr/bin/env bash
set -Eeuo pipefail
root=$(cd -- "$(dirname -- "$0")/../.." && pwd)
for file in "$root"/install.sh "$root"/editions/linux/installer/*.sh "$root"/scripts/package-linux.sh; do bash -n "$file"; done
if [[ $(uname -s) == Linux ]]; then
output=$(printf '0\n' | python3 "$root/tests/installer/terminal.py" bash "$root/editions/linux/installer/install.sh")
grep -q 'Development Preview' <<< "$output"
grep -q 'Real RouterOS and VPN client interoperability testing is incomplete' <<< "$output"
grep -q '15) Panel Information' <<< "$output"
else
 printf 'SKIP: controlling-PTY menu integration requires Linux\n'
fi
# Unsupported platforms must stop before touching any host installation.
mock=$(mktemp -d)
trap 'rm -rf -- "$mock"' EXIT
printf '#!/usr/bin/env bash\necho UnsupportedOS\n' > "$mock/uname"
chmod +x "$mock/uname"
# Test preflight in isolation; root checks are covered by the real root check in production.
if PATH="$mock:$PATH" bash -c '. "$1"; root(){ :; }; preflight' _ "$root/editions/linux/installer/common.sh" >"$mock/output" 2>&1; then exit 1; fi
grep -q 'Linux required' "$mock/output"
printf 'PASS: Bash syntax, interactive menu/warning/exit, unsupported-platform rejection. No service lifecycle tests executed.\n'

