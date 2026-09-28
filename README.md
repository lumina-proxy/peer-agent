# LuminaProxy Peer Agent

The peer agent lets you share part of your unused internet connection with the
[LuminaProxy](https://luminaproxy.com) network and get paid for every GB of
customer web traffic your device carries. This repository is the complete source
of the program you install, so you can read exactly what it does before you run it.

- Earn program, rates and payouts: https://luminaproxy.com/earn
- Earner terms: https://luminaproxy.com/earn-terms

## What the agent does

- Makes **one outbound TLS connection** to `peers.luminaproxy.com:443` and keeps it
  open. It never listens on a port, so nothing on your router needs to change.
- Verifies the hub against a **pinned CA certificate** built into the binary
  ([`internal/hubca`](internal/hubca)), not the system trust store.
- Receives customer requests over that connection (multiplexed with
  [yamux](https://github.com/hashicorp/yamux)). Each request names a target
  `host:port`; the agent connects to it and relays bytes in both directions.

## What it refuses to do

Every target is checked on your device by [`internal/netguard`](internal/netguard)
before any connection is made, so these limits hold even if the hub misbehaves:

- **Web ports only:** targets on any port other than 80 and 443 are refused, so
  your connection is never used for email (SMTP), SSH or other services.
- **No home network access:** the agent resolves DNS itself and refuses any
  address that is private, loopback, link-local, carrier-grade NAT, multicast,
  unspecified or reserved, for IPv4 and IPv6. This covers your router, printers
  and other LAN devices, and cloud metadata addresses such as `169.254.169.254`.
  Every resolved address is vetted, which also defeats DNS rebinding.
- It does not read, store or change your files or your own browsing.
- It does not start until you give consent (`--i-consent`), and it has no
  auto-update: it only ever runs the version you installed.

The tests in [`internal/agent`](internal/agent) and
[`internal/netguard`](internal/netguard) exercise these rules.

## Install

Create a free account at https://luminaproxy.com, open **Dashboard → Earn →
Add device** and copy the device token (it starts with `lpk_`). Then run:

| System | Command |
|---|---|
| Linux / macOS | `curl -fsSL https://luminaproxy.com/downloads/install.sh \| sh -s -- --token lpk_...` |
| Windows (PowerShell) | `$env:LUMINA_TOKEN="lpk_..."; irm https://luminaproxy.com/downloads/install.ps1 \| iex` |
| Docker | `docker run -d --name luminaproxy-peer --restart unless-stopped -e DEVICE_TOKEN=lpk_... -e LUMINA_PEER_CONSENT=yes ghcr.io/lumina-proxy/peer-agent:latest` |

The installers ([`packaging/install.sh`](packaging/install.sh),
[`packaging/install.ps1`](packaging/install.ps1)) download a release, check its
SHA-256 checksum and set the agent up to start automatically: a systemd service on
Linux, a per-user LaunchAgent on macOS and a per-user startup entry on Windows (no
administrator rights needed on Windows or macOS).

To remove it: `install.sh --uninstall` on Linux and macOS, or set
`$env:LUMINA_UNINSTALL="1"` before the Windows command.

## Verify a download

Releases are built by the [release workflow](.github/workflows/ci.yml) in this
repository, from the tagged source. Each release lists SHA-256 checksums and
carries a signed build-provenance attestation, so you can check that a file came
from this code:

```sh
gh attestation verify peer-agent_linux_amd64.tar.gz --repo lumina-proxy/peer-agent
```

The files on https://luminaproxy.com/downloads are byte-for-byte copies of the
release assets.

## Build from source

Requires Go 1.25 or later.

```sh
go build -trimpath -o peer-agent ./cmd/agent
./peer-agent --token lpk_... --i-consent
```

## Options

| Flag | Environment | Default |
|---|---|---|
| `--token` | `DEVICE_TOKEN` | required |
| `--i-consent` | `LUMINA_PEER_CONSENT=yes` | off (the agent will not start without it) |
| `--hub` | `HUB_ADDR` | `peers.luminaproxy.com:443` |
| `--ca` | `HUB_CA_FILE` | built-in CA |
| `--sni` | `HUB_SNI` | `peer-hub.luminaproxy` |
| `--pause-file` | `PAUSE_FILE` | none (the agent pauses while this file exists) |
| `--log-file` | `LOG_FILE` | stdout |
| `--version` | | |
| | `LOG_LEVEL=debug` | info |

## Security

Please report vulnerabilities privately; see [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
