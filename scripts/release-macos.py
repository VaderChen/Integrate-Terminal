#!/usr/bin/env python3
"""以 Developer ID 簽署、公證並驗證 macOS 正式發行包。"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import plistlib
import shutil
import subprocess
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]


def run(args, capture=False):
    result = subprocess.run([str(arg) for arg in args], check=True, cwd=ROOT,
                            text=True, stdout=subprocess.PIPE if capture else None)
    return result.stdout if capture else None


def configuration():
    identity = os.environ.get('INTEGTERM_CODESIGN_IDENTITY', '')
    profile = os.environ.get('INTEGTERM_NOTARY_PROFILE', '')
    if not identity.startswith('Developer ID Application:'):
        raise ValueError('請設定 INTEGTERM_CODESIGN_IDENTITY 為 Developer ID Application；不接受 ad-hoc 簽章')
    if not profile:
        raise ValueError('請設定 INTEGTERM_NOTARY_PROFILE 為既有公證設定名稱')
    return identity, profile


def notarize(target, profile):
    print(f'提交 Apple 公證：{target.name}', flush=True)
    result = json.loads(run(['xcrun', 'notarytool', 'submit', target,
                            '--keychain-profile', profile, '--wait', '--output-format', 'json'], capture=True))
    if result.get('status') != 'Accepted':
        raise RuntimeError(f"Apple 公證未通過：{result.get('status')}；提交 ID：{result.get('id')}")
    print(f'Apple 公證通過：{target.name}', flush=True)
    return {'id': result['id'], 'status': result['status']}


def staple(target):
    for attempt in range(10):
        result = subprocess.run(['xcrun', 'stapler', 'staple', str(target)], cwd=ROOT,
                                capture_output=True, text=True)
        if result.returncode == 0:
            run(['xcrun', 'stapler', 'validate', target])
            return
        if attempt < 9:
            time.sleep(6)
    raise RuntimeError(f'公證票根附加失敗：{target.name}\n{result.stderr}')


def sign(target, identity, identifier, runtime=True):
    args = ['codesign', '--force', '--sign', identity, '--identifier', identifier, '--timestamp']
    if runtime:
        args += ['--options', 'runtime']
    run([*args, target])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--build', action='store_true', help='驗證公證設定後，先執行 build.sh')
    options = parser.parse_args()
    identity, profile = configuration()
    # 只使用既有認證設定，不匯出密鑰或把認證寫入專案。
    run(['xcrun', 'notarytool', 'history', '--keychain-profile', profile, '--output-format', 'json'], capture=True)
    if options.build:
        run([ROOT / 'build.sh'])
    source = ROOT / 'build/bin/IntegTERM.app'
    info = plistlib.loads((source / 'Contents/Info.plist').read_bytes())
    architecture = run(['lipo', '-archs', source / 'Contents/MacOS' / info['CFBundleExecutable']], capture=True).strip()
    if architecture != 'arm64':
        raise ValueError(f'此發行流程要求 arm64，實際為 {architecture}')
    version, bundle = info['CFBundleShortVersionString'], info['CFBundleVersion']
    build = bundle.removeprefix(version)
    if not bundle.startswith(version) or len(build) != 4 or not build.isdigit():
        raise ValueError('Bundle 版本缺少四位 build 編號，請先使用 build.sh 建置')
    release = f'{version}.{build}'
    destination = ROOT / 'dist' / release
    if destination.exists():
        raise FileExistsError(f'版本產物已存在，不覆寫：{destination}')
    destination.parent.mkdir(parents=True, exist_ok=True)
    identifier = info['CFBundleIdentifier']
    with tempfile.TemporaryDirectory(prefix='integterm-signed-release-') as temporary:
        work = Path(temporary)
        image_root = work / 'image'
        image_root.mkdir()
        app = image_root / source.name
        run(['ditto', '--norsrc', '--noextattr', '--noqtn', source, app])
        # 由內而外簽署，讓 Hardened Runtime 的函式庫驗證使用同一個 Team ID。
        for library in sorted((app / 'Contents/Frameworks').glob('*.dylib')):
            sign(library, identity, f'{identifier}.{library.stem}')
        for executable in sorted((app / 'Contents/MacOS').iterdir()):
            if executable.is_file() and os.access(executable, os.X_OK):
                sign(executable, identity, identifier if executable.name == info['CFBundleExecutable'] else f'{identifier}.{executable.name}')
        sign(app, identity, identifier)
        run(['codesign', '--verify', '--deep', '--strict', app])
        archive = work / 'IntegTERM.zip'
        run(['ditto', '-c', '-k', '--sequesterRsrc', '--keepParent', app, archive])
        app_notary = notarize(archive, profile)
        staple(app)
        run(['spctl', '--assess', '--type', 'execute', '--verbose=2', app])
        (image_root / 'Applications').symlink_to('/Applications')
        dmg = work / f'IntegTERM-{release}-macOS-arm64.dmg'
        run(['hdiutil', 'create', '-volname', 'IntegTERM', '-srcfolder', image_root,
             '-format', 'ULMO', dmg])
        sign(dmg, identity, f'{identifier}.dmg', runtime=False)
        run(['codesign', '--verify', '--strict', dmg])
        dmg_notary = notarize(dmg, profile)
        staple(dmg)
        run(['hdiutil', 'verify', dmg])
        run(['spctl', '--assess', '--type', 'open', '--context', 'context:primary-signature', '--verbose=2', dmg])
        # 在目的地檔案系統暫存，所有驗證通過後才原子發布整組產物。
        with tempfile.TemporaryDirectory(prefix='.signed-release-', dir=destination.parent) as publishing:
            staged = Path(publishing) / release
            staged.mkdir()
            shutil.copy2(dmg, staged / dmg.name)
            (staged / 'SHA256SUMS.txt').write_text(f'{hashlib.sha256(dmg.read_bytes()).hexdigest()}  {dmg.name}\n')
            (staged / 'notarization.json').write_text(json.dumps({
                'version': release, 'architecture': 'arm64', 'minimum_macos': '13.0',
                'app': app_notary, 'dmg': dmg_notary,
                'gatekeeper': 'accepted', 'stapled': True,
            }, indent=2) + '\n')
            run(['ditto', '--norsrc', '--noextattr', '--noqtn', app, staged / app.name])
            run(['codesign', '--verify', '--deep', '--strict', staged / app.name])
            run(['xcrun', 'stapler', 'validate', staged / app.name])
            run(['xcrun', 'stapler', 'validate', staged / dmg.name])
            staged.rename(destination)
    print(f'簽章、公證與 Gatekeeper 驗證完成：{destination}', flush=True)


if __name__ == '__main__':
    main()
