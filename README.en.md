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

- SSH, Telnet, and local shell terminals (local shell is supported on macOS and Linux).
- SFTP and FTP file browsing with a transfer queue.
- Site groups, tab restoration, and ZIP backup and restore.
- A local MCP Streamable HTTP server for AI and automation integrations.
- Background service and system tray controls.
- English, Japanese, Korean, Traditional Chinese, and Simplified Chinese interfaces.

## Open Source Edition

The open source edition does not use StoreKit, limit the number of connections, or enable App Sandbox. The application can access files and directories already available to the current user account; macOS may still request permission for privacy-protected locations.

On first launch, the application attempts to migrate sites, settings, known hosts, PPK copies, and the REST API token from the previous sandboxed edition. The public source code does not include Apple signing certificates, provisioning profiles, private keys, or personal site data.

## MCP Integration

IntegTERM includes a standard MCP server over Streamable HTTP. MCP-compatible AI and automation clients can manage saved sites, open SSH, Telnet, SFTP, FTP, and macOS/Linux local terminal tabs, execute SSH commands, interact with terminal sessions, transfer files, and inspect transfer queues and operation logs.

### Enable the MCP Server

1. Start IntegTERM and open **Settings** → **MCP**.
2. Review the listening port and IP allowlist. The default port is `18080`, and the default allowlist is `127.0.0.1`.
3. Enable the MCP Server. The default endpoint is `http://127.0.0.1:18080/mcp`.

### MCP Client Configuration

```json
{
  "mcpServers": {
    "integterm": {
      "type": "streamable-http",
      "url": "http://127.0.0.1:18080/mcp"
    }
  }
}
```

The MCP endpoint does not require an API token or custom authentication header. Access control relies entirely on the source IP/CIDR allowlist. Keep the default `127.0.0.1` unless a trusted remote client must connect, and do not add unnecessarily broad networks. The in-app MCP page displays the current endpoint and complete tool documentation generated from the running configuration. Clients should call `tools/list` before using tools so they receive the current schemas.

## Development Requirements

- macOS 12 or later on Apple Silicon, Windows 10/11 x64, or Linux x64
- Go 1.23 or later
- Node.js and npm
- Xcode Command Line Tools for macOS builds
- GTK3, WebKitGTK, AppIndicator, and `pkg-config` for Linux builds
- The project scripts automatically use the Wails v2 CLI version declared by `go.mod`

## Run in Development Mode

```bash
git clone https://github.com/VaderChen/Integrate-Terminal.git
cd Integrate-Terminal
./run.sh
```

`run.sh` creates a development mirror in a local temporary directory to prevent AppleDouble files generated on external drives from interfering with the Wails build.

## Build Desktop Executables

### macOS Apple Silicon

```bash
./build.sh
```

The output is written to `build/bin/IntegTERM.app`. By default, the app uses an ad-hoc signature and does not enable App Sandbox, so it can be opened locally after the build finishes.

### Windows x64

```powershell
powershell -ExecutionPolicy Bypass -File .\build-windows.ps1
```

The output is written to `build\bin\IntegTERM.exe`. The script creates only an x64 executable and does not create an installer or apply a signature.

### Linux x64

```bash
./build-linux.sh
```

The output is written to `build/bin/IntegTERM`. The script detects the installed WebKitGTK and AppIndicator versions and creates only an x64 executable, without AppImage, DEB, or RPM packaging.

The public GitHub repository does not include Developer ID, Apple notarization, DMG, or release packaging for other platforms. Distributors must handle signing, installers, and release requirements separately.

## Data and Security

Application data is stored in the `IntegTERM` directory under the platform's `os.UserConfigDir()`, such as `~/Library/Application Support/IntegTERM` on macOS, `%AppData%\IntegTERM` on Windows, and `~/.config/IntegTERM` on Linux. Site passwords and PPK passphrases are currently stored in local site data and site backup ZIP files. Restrict file permissions and protect backups appropriately. The REST/MCP service binds only to `127.0.0.1` by default; configure the IP allowlist correctly before permitting external sources.

Do not commit `cert/`, `data/`, `.env*`, installers, signing assets, or files containing real credentials. See [SECURITY.md](SECURITY.md) for security reporting instructions.

## License

This project uses dual licensing:

1. Open source use is licensed under the [GNU General Public License v3.0](LICENSE).
2. A separate [commercial license](COMMERCIAL-LICENSE.md) is available for users who cannot comply with GPLv3, require closed-source integration, or need other commercial terms.

Until a formal Contributor License Agreement is available, only issue reports and discussions are accepted. See [CONTRIBUTING.md](CONTRIBUTING.md) for details.
