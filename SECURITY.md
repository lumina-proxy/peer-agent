# Security policy

The peer agent runs on other people's computers, so we take reports seriously.

## Reporting a vulnerability

Please do not open a public issue. Email **info@luminaproxy.com** with the subject
"Security: peer agent" and include what you found, how to reproduce it and the
version (`peer-agent --version`). We aim to reply within three working days and to
ship a fix for confirmed issues as quickly as we can.

## In scope

- Ways to make the agent connect to private, local or otherwise refused addresses,
  or to ports other than 80 and 443.
- Ways to run code on, or read data from, a device running the agent.
- Weaknesses in how the agent verifies the hub, or in the installers.

## Supported versions

Only the latest release is supported. Update by running the installer again.
