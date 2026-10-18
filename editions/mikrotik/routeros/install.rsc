# Template: provide every global through a reviewed local script before importing.
# RouterOS version-specific syntax MUST be validated on authorized lab hardware.
:global mvmName
:global mvmImage
:global mvmRoot
:global mvmData
:global mvmSecrets
:global mvmVeth
:global mvmAddress
:global mvmGateway
:global mvmEnv
:global mvmApproved
:if ($mvmApproved != "VALIDATED") do={ :error "Validate hardware, image architecture, network ACLs, mounts and secrets first" }
:foreach value in={$mvmName;$mvmImage;$mvmRoot;$mvmData;$mvmSecrets;$mvmVeth;$mvmAddress;$mvmGateway;$mvmEnv} do={
 :if ([:len $value] = 0) do={ :error "All environment-specific mvm globals are required" }
}
:if ([:len [/container find where comment=("mvm:" . $mvmName)]] > 0) do={ :error "Owned container exists; use update procedure" }
:if ([:len [/interface find where name=$mvmVeth]] > 0) do={ :error "Refusing to replace existing interface" }
# Existing envlist must contain TLS/origin, loopback-safe proxy policy, external key file and bootstrap token path.
:if ([:len [/container/envs find where list=$mvmEnv]] = 0) do={ :error "Secure envlist must be provisioned first" }
/interface/veth add name=$mvmVeth address=$mvmAddress gateway=$mvmGateway comment=("mvm:" . $mvmName)
/container/mounts add name=($mvmName . "-data") src=$mvmData dst="/data"
/container/mounts add name=($mvmName . "-secrets") src=$mvmSecrets dst="/secrets"
/container add file=$mvmImage interface=$mvmVeth root-dir=$mvmRoot envlist=$mvmEnv mounts=($mvmName . "-data," . $mvmName . "-secrets") start-on-boot=yes logging=no comment=("mvm:" . $mvmName)
# Deliberately no bridge/NAT/firewall mutation or automatic start. Review ACLs then start the exact container ID.
