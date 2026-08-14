<div align="center">
  <img src="assets/appicon.png" alt="Integrate Terminal アイコン" width="128" />
  <h1>Integrate Terminal</h1>
  <p>Go、Wails、React、TypeScript で構築された、複数プロトコル対応のデスクトップターミナルおよびファイル転送ツールです。</p>
</div>

<p align="center">
  <a href="README.md">繁體中文</a> |
  <a href="README.en.md">English</a> |
  <a href="README.ja.md">日本語</a> |
  <a href="README.ko.md">한국어</a>
</p>

## 機能

- SSH、Telnet、ローカル Shell ターミナル。
- SFTP、FTP のファイル閲覧および転送キュー。
- サイトのグループ化、タブの復元、ZIP 形式のバックアップと復元。
- AI および自動化ツールと連携するローカル MCP Streamable HTTP Server。
- バックグラウンドサービスとシステムトレイからの状態制御。
- 英語、日本語、韓国語、繁体字中国語、簡体字中国語のインターフェース。

## オープンソース版

オープンソース版は StoreKit を使用せず、接続数を制限せず、App Sandbox も有効にしません。現在ログインしているユーザーアカウントが元からアクセス権を持つファイルとディレクトリを利用できます。ただし、macOS のプライバシー保護対象となる場所では、システムがユーザーに許可を求める場合があります。

初回起動時には、以前のサンドボックス版からサイト、設定、known hosts、PPK のコピー、REST API トークンの移行を試みます。公開ソースコードには、Apple の署名証明書、Provisioning Profile、秘密鍵、個人のサイトデータは含まれません。

## 開発環境

- macOS 12 以降
- Go 1.23 以降
- Node.js および npm
- Xcode Command Line Tools
- Wails v2 CLI（未インストールの場合は、プロジェクトのスクリプトが `go install` を使用してインストールします）

## 開発モードで起動

```bash
git clone https://github.com/VaderChen/Integrate-Terminal.git
cd Integrate-Terminal
./run.sh
```

`run.sh` はローカルの一時ディレクトリに開発用ミラーを作成し、外付けドライブで生成される AppleDouble ファイルが Wails のビルドに影響することを防ぎます。

## macOS App のビルド

```bash
./build.sh
```

出力先は `build/bin/IntegTERM.app` です。デフォルトでは ad-hoc 署名を使用し、App Sandbox は有効にしないため、ビルド完了後にローカル環境で直接起動できます。

GitHub の公開版では、通常の `.app` ビルド手順のみを提供しています。Developer ID、Apple notarization、DMG リリース用スクリプトは含まれません。自身でビルドした App を配布する場合は、配布者が Apple の署名およびリリース要件に別途対応する必要があります。

## データとセキュリティ

アプリケーションデータは、デフォルトで `~/Library/Application Support/IntegTERM` に保存されます。サイトのパスワードと PPK パスフレーズは、現在ローカルのサイトデータおよびサイトバックアップ用 ZIP ファイルに保存されます。ファイルのアクセス権を制限し、バックアップを適切に管理してください。REST/MCP サービスはデフォルトで `127.0.0.1` のみにバインドされます。外部からの接続を許可する前に、IP 許可リストを正しく設定してください。

`cert/`、`data/`、`.env*`、インストーラー、署名用資産、実際の認証情報を含むファイルはコミットしないでください。セキュリティ問題の報告方法については、[SECURITY.md](SECURITY.md) を参照してください。

## ライセンス

本プロジェクトはデュアルライセンスを採用しています。

1. オープンソースとしての利用には、[GNU General Public License v3.0](LICENSE) が適用されます。
2. GPLv3 に準拠できない場合、クローズドソースへの組み込みが必要な場合、またはその他の商用条件が必要な場合は、別途[商用ライセンス](COMMERCIAL-LICENSE.md)を取得できます。

正式な Contributor License Agreement が整備されるまでは、Issue による報告とディスカッションのみ受け付けます。詳細は [CONTRIBUTING.md](CONTRIBUTING.md) を参照してください。
