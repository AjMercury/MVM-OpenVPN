#!/usr/bin/env bash
set -Eeuo pipefail
root=$(cd -- "$(dirname -- "$0")/../.." && pwd)
. "$root/editions/linux/installer/common.sh"
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
BASE="$tmp/opt";mkdir -p "$BASE" "$tmp/payload"
ARCH=amd64
printf '#!/usr/bin/env bash\nexit 0\n' > "$tmp/payload/server"
cp "$tmp/payload/server" "$tmp/payload/mvmctl"
chmod +x "$tmp/payload/server" "$tmp/payload/mvmctl"
printf 'VERSION=1\nARCH=amd64\nSCHEMA=1\n' > "$tmp/payload/metadata.env"
tar -czf "$tmp/release.tar.gz" -C "$tmp/payload" .
digest=$(sha256sum "$tmp/release.tar.gz" | cut -d ' ' -f 1)
if (stage_release "$tmp/release.tar.gz" "$(printf '%064d' 0)" 1);then exit 1;fi
[[ ! -e $BASE/releases/1 ]]
(stage_release "$tmp/release.tar.gz" "$digest" 1)
[[ -f $BASE/releases/1/server ]]
if [[ $(uname -s) == Linux ]]; then
 [[ $(stat -c %a "$BASE/releases/1") == 755 ]]
 [[ $(stat -c %a "$BASE/releases/1/server") == 755 ]]
 [[ $(stat -c %a "$BASE/releases/1/mvmctl") == 755 ]]
 [[ $(stat -c %a "$BASE/releases/1/metadata.env") == 644 ]]
else printf 'SKIP: POSIX release permission assertions require Linux\n'; fi
if (stage_release "$tmp/release.tar.gz" "$digest" 1);then exit 1;fi
(REPAIR_STAGING=true; stage_release "$tmp/release.tar.gz" "$digest" 1; [[ $TARGET == "$BASE/releases/1-repair-"* && -f $TARGET/server ]])
if (ARCH=arm64;stage_release "$tmp/release.tar.gz" "$digest" 1);then exit 1;fi
if (stage_release "$tmp/missing.tar.gz" "$digest" 2);then exit 1;fi
curl(){ return 22; }
if (stage_release https://invalid.example/test-only "$digest" 2);then exit 1;fi
printf 'PASS: archive staging, checksum mismatch, repeated version, architecture mismatch, missing local archive.\n'

