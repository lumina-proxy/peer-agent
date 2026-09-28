LuminaProxy Peer Agent
======================

The LuminaProxy peer agent shares part of your unused internet connection with
the LuminaProxy network. You are paid for every GB of customer web traffic your
device carries. Details and current rates: https://luminaproxy.com/earn

What it does
- Relays customer web requests (ports 80 and 443) and nothing else.
- Never reads, stores or changes your files or your own browsing.
- Refuses private and local network addresses, so your home network stays off limits.
- Makes one encrypted outbound connection; it never opens a port on your device.

Recommended install (one command)
1. Create a free account at https://luminaproxy.com and open Dashboard -> Earn.
2. Add a device and copy its token (it starts with lpk_).
3. Run the installer for your system:
   Linux / macOS:
     curl -fsSL https://luminaproxy.com/downloads/install.sh | sh -s -- --token lpk_...
   Windows (PowerShell):
     $env:LUMINA_TOKEN="lpk_..."; irm https://luminaproxy.com/downloads/install.ps1 | iex

Running this download directly
  Linux / macOS:  ./peer-agent --token lpk_... --i-consent
  Windows:        peer-agent.exe --token lpk_... --i-consent --log-file agent.log
  Linux service:  put DEVICE_TOKEN=lpk_... and LUMINA_PEER_CONSENT=yes in
                  /etc/luminaproxy/peer-agent.env (see peer-agent.env.example)

Remove it
  Linux / macOS:  curl -fsSL https://luminaproxy.com/downloads/install.sh | sh -s -- --uninstall
  Windows:        $env:LUMINA_UNINSTALL="1"; irm https://luminaproxy.com/downloads/install.ps1 | iex

Terms: https://luminaproxy.com/earn-terms
Support: https://luminaproxy.com/contact
