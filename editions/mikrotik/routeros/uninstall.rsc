:global mvmName
:global mvmApproved
:if ($mvmApproved != "REMOVE APPLICATION ONLY") do={ :error "Explicit application-only removal confirmation required" }
:if ([:len $mvmName] = 0) do={ :error "mvmName is required" }
:local owned [/container find where comment=("mvm:" . $mvmName)]
:if ([:len $owned] != 1) do={ :error "Expected exactly one tracked container" }
/container stop $owned
:delay 15s
/container remove $owned
# Retain mounts, VETH, images, data, keys and all VPN/firewall/bridge/NAT resources.
# Review orphaned application networking manually; names alone are not proof of ownership.
