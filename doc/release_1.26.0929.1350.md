# Integrate Terminal 1.26.0929 (Build 1350)

For macOS 13 or later on Apple Silicon (arm64).

## What's New

- Added a live progress bar with download percentage and downloaded/total size for application updates.
- Displayed separate download, file verification, and installation preparation stages.
- Restored Check for Updates in Settings → About, with retry support and protection against duplicate downloads.
- Removed the License / Pro Unlock page from Settings.

## Included Improvements

- Preserved the two-column site editor and restored editable tags with file-based persistence.
- Supported unencrypted PPK files without a passphrase and improved messages for encrypted or invalid PPK files.
- Kept saved-site credentials in local files and moved MCP explanations into tooltips to reduce clutter.
- Included the macOS packaging improvements from Build 1335.

## Installation

Download the DMG, open it, and drag IntegTERM to Applications. Replace the previous copy when prompted.

The new progress display is available for updates initiated from this build or later. Older installed versions keep their existing update interface until upgraded.

## Compatibility

Back up saved sites, tabs, and settings before upgrading from the September 12 release. Favorite sites, folder authorization, and forced-update settings from that release are not included in this branch. PPK files are read directly from their configured paths. The previously reported PPK connection issue still requires diagnosis with the affected file.

This release includes and supersedes Build 1335.
