#!/usr/bin/env bash
uninstall_panel() {
 preflight
 printf 'Remove: /opt/mvm /usr/local/bin/mvm /etc/systemd/system/mvm.service\nPreserve: /etc/mvm /var/lib/mvm /var/backups/mvm\n'
 local answer
 read -r -p 'Type REMOVE to uninstall: ' answer
 [[ $answer == REMOVE ]] || die 'Cancelled'
 systemctl disable --now mvm
 rm -f /etc/systemd/system/mvm.service /usr/local/bin/mvm
 # Fixed absolute application-only path, never derived from database or environment input.
 rm -rf -- /opt/mvm
 systemctl daemon-reload
 if [[ ${1:-} == --purge ]]; then
  printf 'Irreversible deletion: /etc/mvm /var/lib/mvm /var/backups/mvm\n'
  read -r -p 'Type DELETE ALL MVM DATA to permanently purge: ' answer
  [[ $answer == 'DELETE ALL MVM DATA' ]] || die 'Persistent data preserved'
  rm -rf -- /etc/mvm /var/lib/mvm /var/backups/mvm
 fi
}
