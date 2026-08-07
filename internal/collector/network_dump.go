package collector

const networkManagerDumpScript = `#!/bin/sh

INTERFACE=$1

section()
{
    echo
    echo "============================================================"
    echo "$1"
    echo "============================================================"
}

run()
{
    echo
    echo "\$ $*"
    "$@" 2>&1
}

nmcli_cmd()
{
    LC_ALL=C \
    PAGER="" \
    NO_COLOR=1 \
    nmcli --colors no "$@"
}

resolvectl_cmd()
{
    SYSTEMD_PAGER=cat \
    SYSTEMD_COLORS=0 \
    resolvectl "$@"
}

if ! command -v nmcli >/dev/null 2>&1; then
    echo "[network-dump] Error: nmcli is not available"
    exit 1
fi

PROFILE="$(
    nmcli_cmd -g GENERAL.CONNECTION device show "$INTERFACE" 2>/dev/null |
    head -n 1
)"

if [ -z "$PROFILE" ] || [ "$PROFILE" = "--" ]; then
    echo "[network-dump] Error: No active profile found for $INTERFACE"
    exit 1
fi

section "GENERAL INFORMATION"

echo "Timestamp: $(date)"
echo "Hostname:  $(hostname)"
echo "Interface: $INTERFACE"
echo "Profile:   $PROFILE"

run nmcli_cmd general status
run nmcli_cmd device status

section "ACTIVE CONNECTION"

run nmcli_cmd \
    -f NAME,UUID,TYPE,DEVICE,STATE \
    connection show --active

section "PROFILE CONNECTION SETTINGS"

run nmcli_cmd \
    -f connection.id,connection.uuid,connection.type,connection.interface-name,connection.autoconnect \
    connection show "$PROFILE"

section "PROFILE IPV4 SETTINGS"

run nmcli_cmd \
    -f ipv4.method,ipv4.addresses,ipv4.gateway,ipv4.dns,ipv4.ignore-auto-dns \
    connection show "$PROFILE"

section "802.1X CONFIGURATION"

run nmcli_cmd \
    -f 802-1x.eap,802-1x.identity,802-1x.anonymous-identity,802-1x.ca-cert,802-1x.client-cert,802-1x.private-key,802-1x.phase2-auth \
    connection show "$PROFILE"

section "APPLIED DEVICE CONFIGURATION"

run nmcli_cmd \
    -f GENERAL.DEVICE,GENERAL.TYPE,GENERAL.STATE,GENERAL.CONNECTION,GENERAL.CON-UUID,GENERAL.HWADDR,GENERAL.MTU,IP4.ADDRESS,IP4.GATEWAY,IP4.ROUTE,IP4.DNS \
    device show "$INTERFACE"

if command -v ip >/dev/null 2>&1; then
    section "KERNEL NETWORK CONFIGURATION"

    run ip link show dev "$INTERFACE"
    run ip -4 address show dev "$INTERFACE"
    run ip route show dev "$INTERFACE"
fi

section "DNS CONFIGURATION"

if command -v resolvectl >/dev/null 2>&1; then
    run resolvectl_cmd status "$INTERFACE"
fi

if [ -e /etc/resolv.conf ]; then
    run ls -l /etc/resolv.conf
    run cat /etc/resolv.conf
fi
`
