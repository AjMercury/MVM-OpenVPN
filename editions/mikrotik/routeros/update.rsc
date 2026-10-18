# Offline verified update staging. No shared network configuration is modified.
:global mvmName
:global mvmNextName
:global mvmNextImage
:global mvmNextRoot
:global mvmNextVeth
:global mvmNextEnv
:global mvmNextMounts
:global mvmApproved
:if ($mvmApproved != "BACKUP VERIFIED") do={ :error "Stop, back up and verify the encrypted database/key before update" }
:foreach value in={$mvmName;$mvmNextName;$mvmNextImage;$mvmNextRoot;$mvmNextVeth;$mvmNextEnv;$mvmNextMounts} do={
 :if ([:len $value] = 0) do={ :error "Validated old/new names, versioned image/root, VETH, envlist and mounts are required" }
}
:if ($mvmName = $mvmNextName) do={ :error "A distinct versioned container name is required" }
:local owned [/container find where comment=("mvm:" . $mvmName)]
:if ([:len $owned] != 1) do={ :error "Expected exactly one tracked source container" }
:if ([:len [/container find where comment=("mvm:" . $mvmNextName)]] > 0) do={ :error "Candidate already exists; inspect it rather than replace" }
:if ([:len [/interface/veth find where name=$mvmNextVeth]] != 1) do={ :error "Provision a separate reviewed candidate VETH first" }
:if ([:len [/container/envs find where list=$mvmNextEnv]] = 0) do={ :error "Secure candidate envlist must exist" }
/container stop $owned
/container set $owned start-on-boot=no
# Old container/root remain for rollback. Same bbolt data may be used only while old container is stopped.
/container add file=$mvmNextImage interface=$mvmNextVeth root-dir=$mvmNextRoot envlist=$mvmNextEnv mounts=$mvmNextMounts start-on-boot=no logging=no comment=("mvm:" . $mvmNextName)
# Wait for extraction and validate compatibility/readiness manually before enabling candidate start-on-boot.
# Never start both containers against one data mount. On failure retain backup and restart only the old compatible instance.
