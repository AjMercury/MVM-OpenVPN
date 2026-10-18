#!/usr/bin/env bash
set +x
set -Eeuo pipefail
umask 077
BASE=/opt/mvm
CONF=/etc/mvm
DATA=/var/lib/mvm
BACKUPS=/var/backups/mvm
WARNING='WARNING: This application is currently a Development Preview. Real RouterOS and VPN client interoperability testing is incomplete. Production deployment is not recommended.'
die() { printf '%s\n' "$*" >&2; exit 1; }
root() { [[ $EUID == 0 ]] || die 'Run with sudo.'; }
installation_state() {
 if [[ -L $BASE/current && -f $CONF/config.env && -s $CONF/master.key && -f $DATA/mvm.db ]]; then printf 'installed\n'
 elif [[ -f $CONF/config.env && -s $CONF/master.key && -f $DATA/mvm.db && ! -e $BASE/current ]]; then printf 'preserved-data\n'
 elif [[ -e $BASE/current || -L $BASE/current || -e $CONF/config.env || -e $CONF/master.key || -e $DATA/mvm.db ]]; then printf 'incomplete\n'
 else printf 'fresh\n'; fi
}
validate_os() {
 [[ $ID == debian || $ID == ubuntu ]] || die 'Only Debian and Ubuntu are supported'
 if [[ $ID == ubuntu ]]; then
  [[ ${VERSION_ID:-} == 22.04 || ${VERSION_ID:-} == 24.04 ]] || die 'Ubuntu LTS 22.04 or 24.04 required'
 else [[ ${VERSION_ID:-} == 12 || ${VERSION_ID:-} == 13 ]] || die 'Debian 12 or 13 required'; fi
}
ensure_service_user() {
 id mvm >/dev/null 2>&1 || useradd --system --home-dir "$DATA" --shell /usr/sbin/nologin mvm
 [[ $(id -u mvm) =~ ^[0-9]+$ && $(id -u mvm) -gt 0 ]] || die 'mvm must be an unprivileged service account'
}
validate_arch() {
 case "$1" in x86_64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; *) die 'Unsupported architecture';; esac
}
check_space() {
 for location in /var /opt; do
  [[ $(df -Pk "$location" | awk 'END {print $4}') -ge 524288 ]] || die "At least 512 MiB free in $location is required"
 done
}
preflight() {
 root
 [[ $(uname -s) == Linux ]] || die 'Linux required'
 . /etc/os-release
 validate_os
 validate_arch "$(uname -m)"
 [[ -d /run/systemd/system ]] || die 'A running systemd instance is required'
 check_space
 ensure_prerequisites
 for c in ss ip systemctl journalctl curl sha256sum tar openssl useradd runuser df flock install readlink find awk sed grep cp mv ln rm mkdir chmod chown id basename sleep; do command -v "$c" >/dev/null || die "Missing dependency: $c"; done
 check_space
 if id mvm >/dev/null 2>&1; then [[ $(id -u mvm) -gt 0 ]] || die 'Refusing a root-privileged mvm account'; fi
 exec 9>/run/lock/mvm.lock
 flock -n 9 || die 'Another management operation is running'
}
load_config() {
 [[ -f $CONF/config.env ]] || die 'Configuration is missing; repair required'
 set -a; . "$CONF/config.env"; set +a
 [[ -s ${MVM_MASTER_KEY_FILE:-} ]] || die 'Existing master key missing; recovery required'
 [[ ${MVM_DATABASE:-} == "$DATA/mvm.db" ]] || die 'Unexpected database path; use reviewed manual recovery'
 if [[ -n ${MVM_TLS_CERT:-} ]]; then
  openssl x509 -in "$MVM_TLS_CERT" -noout -checkend 0 >/dev/null || die 'TLS certificate invalid/expired'
  [[ -s ${MVM_TLS_KEY:-} ]] || die 'TLS key missing'
 else [[ ${MVM_LISTEN:-} == 127.0.0.1:* && ${MVM_DEV_HTTP:-} == true ]] || die 'HTTP must bind loopback'; fi
}
ctl() { runuser -u mvm -- env MVM_MASTER_KEY_FILE="$CONF/master.key" "$BASE/current/mvmctl" "$@"; }
ready() {
 local origin
 origin=$(sed -n 's/^MVM_ORIGIN=//p' "$CONF/config.env")
 for ((i=0;i<30;i++)); do
  local host port bind
  host=${origin#*://}; host=${host%:*}; port=${origin##*:}
  bind=$(sed -n 's/^MVM_LISTEN=//p' "$CONF/config.env"); bind=${bind%:*}
  if systemctl is-active --quiet mvm && curl --noproxy '*' --resolve "$host:$port:$bind" --fail --silent --max-time 2 "${origin}/readyz" >/dev/null; then return 0; fi
  sleep 1
 done
 return 1
}
preview_confirm() {
 printf '%s\n' "$WARNING"
 if [[ ${MVM_ACCEPT_PREVIEW:-} == 'I ACCEPT DEVELOPMENT PREVIEW' ]]; then return; fi
 local answer
 read -r -p 'Type INSTALL PREVIEW to continue: ' answer
 [[ $answer == 'INSTALL PREVIEW' ]] || die 'Installation cancelled'
}
# Digest must come through an authenticated independent channel. Never trust an adjacent untrusted checksum.
stage_release() {
 [[ $# == 3 ]] || die 'Usage: mvm install|update ARCHIVE_OR_HTTPS_URL TRUSTED_SHA256 VERSION'
 local source=$1 digest=$2 version=$3
 [[ $digest =~ ^[a-fA-F0-9]{64}$ && $version =~ ^[0-9][a-zA-Z0-9._-]{0,63}$ ]] || die 'Invalid digest/version'
 STAGE=$(mktemp -d "$BASE/.stage-XXXXXX")
 trap 'rm -rf -- "$STAGE"' EXIT
 if [[ $source == https://* ]]; then curl --fail --connect-timeout 10 --max-time 600 --location --proto '=https' --proto-redir '=https' --tlsv1.2 --output "$STAGE/release.tar.gz" "$source" || die 'Release download failed'; else cp -- "$source" "$STAGE/release.tar.gz" || die 'Release archive copy failed'; fi
 printf '%s  %s\n' "$digest" "$STAGE/release.tar.gz" | sha256sum --check --status || die 'Checksum mismatch'
 # Only regular files/directories; refuse links, traversal and arbitrary archive paths.
 tar -tzf "$STAGE/release.tar.gz" | while IFS= read -r p; do
  [[ $p != /* && $p != *..* && $p != -* ]] || exit 1
 done || die 'Unsafe archive path'
 tar -tvzf "$STAGE/release.tar.gz" | awk 'substr($1,1,1)!="-" && substr($1,1,1)!="d" {exit 1}' || die 'Archive links/special files forbidden'
 mkdir "$STAGE/payload"
 tar -xzf "$STAGE/release.tar.gz" -C "$STAGE/payload" --no-same-owner --no-same-permissions
 [[ -x $STAGE/payload/server && -x $STAGE/payload/mvmctl && -f $STAGE/payload/metadata.env ]] || die 'Incomplete release'
 grep -qx "ARCH=$ARCH" "$STAGE/payload/metadata.env" || die 'Architecture mismatch'
 grep -qx "VERSION=$version" "$STAGE/payload/metadata.env" || die 'Version mismatch'
 grep -qx 'SCHEMA=1' "$STAGE/payload/metadata.env" || die 'Unsupported migration compatibility'
 [[ ! -e $BASE/releases/$version ]] || die 'Release already staged; inspect/repair explicitly'
 TARGET="$BASE/releases/$version"
 install -d -m 755 "$BASE/releases"
 mv "$STAGE/payload" "$TARGET"
 chmod -R go-w "$TARGET"
}
active_release() { [[ -L $BASE/current ]]; }
switch_release() {
 [[ ! -e $BASE/.current-new && ! -L $BASE/.current-new ]] || die 'Stale activation link; inspect before repair'
 ln -s "$1" "$BASE/.current-new" || die 'Cannot stage activation symlink'
 mv -Tf "$BASE/.current-new" "$BASE/current" || die 'Cannot activate release'
}
backup_db() {
 load_config
 local path=${1:-$BACKUPS/mvm-$(date -u +%Y%m%dT%H%M%S)-$RANDOM.db}
 [[ $path == "$BACKUPS/"* ]] || die "Backup must be inside $BACKUPS"
 ctl backup "$DATA/mvm.db" "$path"
 printf '%s\n' "$path"
}






# Offline only. Keep WAL/SHM with their main file; never attach a failed database's WAL to another main file.
move_database() {
 local source=$1 destination=$2 suffix
 for suffix in '' -wal -shm; do [[ ! -e $destination$suffix ]] || die 'Database preservation destination already exists'; done
 mv -- "$source" "$destination" || die 'Cannot preserve database; service must remain stopped'
 for suffix in -wal -shm; do
  if [[ -e $source$suffix ]]; then mv -- "$source$suffix" "$destination$suffix" || die 'Cannot preserve SQLite sidecar; service must remain stopped'; fi
 done
}


# Install runtime packages only, after explicit approval. Native releases include SQLite.
ensure_prerequisites() {
 local command package answer
 local -a packages=()
 for pair in curl:curl openssl:openssl ss:iproute2 ip:iproute2 flock:util-linux runuser:util-linux useradd:passwd tar:tar gzip:gzip awk:mawk sed:sed grep:grep find:findutils sha256sum:coreutils df:coreutils install:coreutils readlink:coreutils mktemp:coreutils cut:coreutils systemctl:systemd journalctl:systemd; do
  command=${pair%%:*}; package=${pair#*:}
  command -v "$command" >/dev/null || packages+=("$package")
 done
 [[ -s /etc/ssl/certs/ca-certificates.crt ]] || packages+=(ca-certificates)
 [[ -f /usr/share/zoneinfo/UTC ]] || packages+=(tzdata)
 if [[ ${#packages[@]} -gt 0 ]]; then
  printf 'Missing runtime packages: %s\n' "${packages[*]}"
  read -r -p 'Allow apt-get update and installation of these packages? [yes/no]: ' answer
  [[ $answer == yes ]] || die 'Runtime prerequisites not installed; cancelled'
  command -v apt-get >/dev/null || die 'APT required'
  apt-get update || die 'APT metadata/network unavailable'
  apt-get install -y --no-install-recommends "${packages[@]}" || die 'Runtime prerequisite installation failed'
 fi
}
