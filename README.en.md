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
- An MCP virtual layer with a RAM workspace and remote-site mounts.
- Background service and system tray controls.
- Manual GitHub Release checks in Settings plus one automatic check at a random time each day, with verified platform update downloads.
- English, Japanese, Korean, Traditional Chinese, and Simplified Chinese interfaces.

## Open Source Edition

The open source edition does not use StoreKit, limit the number of connections, or enable App Sandbox. The application can access files and directories already available to the current user account; macOS may still request permission for privacy-protected locations.

On first launch, the application attempts to migrate sites, settings, known hosts, PPK copies, and the REST API token from the previous sandboxed edition. The public source code does not include Apple signing certificates, provisioning profiles, private keys, or personal site data.

## MCP Integration (VFS enabled by default)

IntegTERM includes a local VFS MCP server over stdio and an optional Streamable HTTP MCP server. A virtual layer unifies RAM workspace resources with saved remote-site mounts. The `integterm-vfs://` value is a resource URI and tool path inside an MCP connection, not a URL that an agent can connect to directly.

### Local VFS MCP (stdio, default)

Local agents should start IntegTERM with the `mcp` argument and connect over stdio:

```json
{
  "mcpServers": {
    "integterm-vfs": {
      "command": "/Applications/IntegTERM.app/Contents/MacOS/IntegTERM",
      "args": ["mcp"]
    }
  }
}
```

When running from source, use `go run . mcp`. After connecting, call `tools/list`, then use `vfs_list` with an empty path or `integterm-vfs://workspace/mcp`; do not put the URI in an HTTP URL field or execute it as a shell command.

### Agent Call Sequence

After connecting, agents should:

1. Call `tools/list` to obtain the current tool schemas.
2. Call `vfs_workspace_info` to confirm the root URI and workspace limits.
3. Call `vfs_list` without `path` (or with `integterm-vfs://workspace/mcp`) to list the workspace root.
4. Use `vfs_stat`, `vfs_read`, `vfs_write`, and the other VFS tools for files. Use `resources/read` for a Resource URI only when the client supports MCP Resources.

The `integterm-vfs://workspace/mcp` value does not open a connection and must not be executed as a shell command.

### MCP Server (HTTP disabled by default)

1. Start IntegTERM and open **Settings** → **MCP**.
2. The local VFS MCP is provided over stdio by default and does not listen on a network port.
3. Enable the HTTP MCP Server only when an external agent must connect. The default port is `18080`, the default allowlist is `127.0.0.1`, and the endpoint is `http://127.0.0.1:18080/mcp`.

### Virtual Workspace: RAM and Remote Sites

The virtual root URI is `integterm-vfs://workspace/mcp`. Paths outside the `sites` namespace are bounded RAM data; `sites/{siteID}` represents a saved remote site. The first `vfs_connect` or file operation lazily opens the connection. RAM data is cleared when the background service stops, while remote operations act directly on the site's configured remote root.

Remote site paths use `integterm-vfs://workspace/mcp/sites/{siteID}/{relativeRemotePath}`. List `sites` to discover site IDs, then use `vfs_list`, `vfs_stat`, `vfs_read`, `vfs_write`, `vfs_mkdir`, `vfs_rename`, and `vfs_delete` for normal file operations. Virtual URIs never contain passwords or private keys.

### Over Network: Existing Operations and Virtual Workspace

External agents use the MCP endpoint `http://127.0.0.1:18080/mcp`, which provides saved-site management, SSH, Telnet, SFTP, FTP, local terminals, commands, interactive terminal operations, file transfers, and the `integterm-vfs` virtual workspace tools above. `integterm-vfs://` is a resource URI, not another HTTP endpoint; HTTP clients should operate on it through tools such as `vfs_list`.

### Network MCP Client Configuration

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

The UI runs as a single instance by default. Launching it again brings the existing window to the front instead of creating another UI. To intentionally run multiple UI instances during development, use the special argument:

```bash
./run.sh --multi-instance
```

This argument only allows multiple UI instances; the background service remains single-instance.

## Build Desktop Executables

### macOS Apple Silicon

```bash
./build.sh
```

The output is written to `dist/IntegTERM.app`. By default, the app uses an ad-hoc signature and does not enable App Sandbox, so it can be opened locally after the build finishes. You can also double-click `build.command` to build, or `run.command` to launch the built app; use `run.command --dev` for development mode.

### Windows x64

```powershell
powershell -ExecutionPolicy Bypass -File .\build-windows.ps1
```

The output is written to `dist\IntegTERM.exe`. The script creates only an x64 executable and does not create an installer or apply a signature.

### Linux x64

```bash
./build-linux.sh
```

The output is written to `dist/IntegTERM`. The script detects the installed WebKitGTK and AppIndicator versions and creates only an x64 executable, without AppImage, DEB, or RPM packaging.

The public GitHub repository does not include signing identities, notarization settings, private keys, or release credentials. Distributors must handle platform signing, installers, and release requirements separately.

## Data and Security

Application data is stored in the `IntegTERM` directory under the platform's `os.UserConfigDir()`, such as `~/Library/Application Support/IntegTERM` on macOS, `%AppData%\IntegTERM` on Windows, and `~/.config/IntegTERM` on Linux. Site passwords and PPK passphrases are currently stored in local site data and site backup ZIP files. Restrict file permissions and protect backups appropriately. The REST/MCP service binds only to `127.0.0.1` by default; configure the IP allowlist correctly before permitting external sources.

Do not commit `cert/`, `data/`, `.env*`, installers, signing assets, or files containing real credentials. See [SECURITY.md](SECURITY.md) for security reporting instructions.

## License

This project uses dual licensing:

1. Open source use is licensed under the [GNU General Public License v3.0](LICENSE).
2. A separate [commercial license](COMMERCIAL-LICENSE.md) is available for users who cannot comply with GPLv3, require closed-source integration, or need other commercial terms.

The commercial license only covers code and assets that the licensor has the right to license separately. It excludes third-party packages, icons, fonts, datasets, AI models, and other third-party content, which remain subject to their respective terms. See [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md) for the dependency inventory; complete license texts are generated at build time and included with release artifacts.

The build process places the GPLv3 text, third-party licensing documents, and `build-metadata.json` in release artifacts. The metadata records the source Git tag, commit, and working-tree state so that a binary can be traced back to its source revision.

Until a formal Contributor License Agreement is available, only issue reports and discussions are accepted. See [CONTRIBUTING.md](CONTRIBUTING.md) for details.
