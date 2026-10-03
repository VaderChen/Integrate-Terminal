#!/usr/bin/env python3
"""編譯並執行 macOS 托盤尺寸與跨解析度繪製 Smoke，不啟動正式服務。"""
from pathlib import Path
import platform
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]


def main():
    if platform.system() != 'Darwin':
        print('略過：托盤 AppKit Smoke 僅適用於 macOS')
        return
    with tempfile.TemporaryDirectory(prefix='integterm-tray-smoke-') as temporary:
        executable = Path(temporary) / 'tray-layout-smoke'
        subprocess.run([
            'xcrun', 'clang', '-fobjc-arc', '-fblocks', '-mmacosx-version-min=13.0',
            '-framework', 'Cocoa', ROOT / 'scripts/tests/tray_layout_smoke.m',
            '-o', executable,
        ], check=True)
        subprocess.run([executable, ROOT / 'internal/trayicon/main.png'], check=True, timeout=30)


if __name__ == '__main__':
    main()
