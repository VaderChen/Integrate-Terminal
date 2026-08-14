<div align="center">
  <img src="assets/appicon.png" alt="Integrate Terminal icon" width="128" />
  <h1>Integrate Terminal</h1>
  <p>A cross-protocol desktop terminal and file transfer tool built with Go, Wails, React, and TypeScript.</p>
</div>

<p align="center">
  <a href="README.md">繁體中文</a> |
  <a href="README.en.md">English</a> |
  <a href="README.ja.md">日本語</a> |
  <a href="README.ko.md">한국어</a>
</p>

## Features

- SSH, Telnet, and local shell terminals.
- SFTP and FTP file browsing with a transfer queue.
- Site groups, tab restoration, and ZIP backup and restore.
- A local MCP Streamable HTTP server for AI and automation integrations.
- Background service and system tray controls.
- English, Japanese, Korean, Traditional Chinese, and Simplified Chinese interfaces.

## Open Source Edition

The open source edition does not use StoreKit, limit the number of connections, or enable App Sandbox. The application can access files and directories already available to the current user account; macOS may still request permission for privacy-protected locations.

On first launch, the application attempts to migrate sites, settings, known hosts, PPK copies, and the REST API token from the previous sandboxed edition. The public source code does not include Apple signing certificates, provisioning profiles, private keys, or personal site data.

## Development Requirements

- macOS 12 or later
- Go 1.23 or later
- Node.js and npm
- Xcode Command Line Tools
- Wails v2 CLI; if it is not installed, the project scripts install it through `go install`

## Run in Development Mode

```bash
git clone https://github.com/VaderChen/Integrate-Terminal.git
cd Integrate-Terminal
./run.sh
```

`run.sh` creates a development mirror in a local temporary directory to prevent AppleDouble files generated on external drives from interfering with the Wails build.

## Build the macOS App

```bash
./build.sh
```

The output is written to `build/bin/IntegTERM.app`. By default, the app uses an ad-hoc signature and does not enable App Sandbox, so it can be opened locally after the build finishes.

The public GitHub repository provides only the standard `.app` build workflow. It does not include Developer ID, Apple notarization, or DMG release scripts. Anyone distributing a self-built app must handle the required Apple signing and release process separately.

## Data and Security

Application data is stored in `~/Library/Application Support/IntegTERM` by default. Site passwords and PPK passphrases are currently stored in local site data and site backup ZIP files. Restrict file permissions and protect backups appropriately. The REST/MCP service binds only to `127.0.0.1` by default; configure the IP allowlist correctly before permitting external sources.

Do not commit `cert/`, `data/`, `.env*`, installers, signing assets, or files containing real credentials. See [SECURITY.md](SECURITY.md) for security reporting instructions.

## License

This project uses dual licensing:

1. Open source use is licensed under the [GNU General Public License v3.0](LICENSE).
2. A separate [commercial license](COMMERCIAL-LICENSE.md) is available for users who cannot comply with GPLv3, require closed-source integration, or need other commercial terms.

Until a formal Contributor License Agreement is available, only issue reports and discussions are accepted. See [CONTRIBUTING.md](CONTRIBUTING.md) for details.
