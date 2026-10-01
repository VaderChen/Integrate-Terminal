# Integrate Terminal 1.26.1001 (Build 1244)

For macOS 13 or later on Apple Silicon (arm64).

## What's New

- MCP requests received directly from `127.0.0.1` no longer require an API key. Other source addresses and REST API requests retain authentication, and existing Host and Origin checks remain in place.
- Local HTTP MCP configuration now omits the unnecessary authorization header.

## Reliability Fixes

- Preserved rapid, consecutive settings changes and prevented a failed save from discarding later changes.
- Made multi-file storage transactions recoverable after write failures or process interruption, and protected unreadable settings files from being overwritten.
- Fixed terminal input and shutdown stalls, improved connection and child-process cleanup, and preserved final local-terminal output.
- Corrected IPv6 connection addresses and applied SSH/SFTP connection timeouts to the handshake.
- Prevented cyclic directory links and special files from stalling uploads while preserving support for ordinary symbolic links.
- Kept cancelled transfers cancelled when delayed progress updates arrive.
- Preserved spaces in complete file paths so batch actions target the selected files and uploads reach the selected directory.
- Kept the validated new application when old-backup cleanup fails during an update, while retaining rollback for installation failures.

## Included Improvements

- Live application-update progress with percentage, downloaded size, verification, and installation stages.
- The two-column site editor, editable tags, file-based saved credentials, and compact MCP help tooltips.
- Unencrypted PPK files can be used without a passphrase; encrypted or invalid files receive clearer diagnostics.
- The License / Pro Unlock page remains removed from Settings. Existing screens and workflows are preserved.

## Installation

Download the DMG, open it, and drag IntegTERM to Applications. Replace the previous copy when prompted. `SHA256SUMS.txt` is provided to verify the download.

## Compatibility

This release includes the improvements from Build 1350. Older installed versions retain their existing update interface until upgraded.

Back up saved sites, tabs, and settings before upgrading from the September 12 release. Favorite sites, folder authorization, and forced-update settings from that release are not included in this branch. PPK files are read directly from their configured paths. The previously reported PPK connection issue still requires diagnosis with the affected file.
