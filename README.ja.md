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

- SSH、Telnet、ローカル Shell ターミナル（ローカル Shell は macOS と Linux に対応）。
- SFTP、FTP のファイル閲覧および転送キュー。
- サイトのグループ化、タブの復元、ZIP 形式のバックアップと復元。
- AI および自動化ツールと連携するローカル MCP Streamable HTTP Server。
- バックグラウンドサービスとシステムトレイからの状態制御。
- 英語、日本語、韓国語、繁体字中国語、簡体字中国語のインターフェース。

## オープンソース版

オープンソース版は StoreKit を使用せず、接続数を制限せず、App Sandbox も有効にしません。現在ログインしているユーザーアカウントが元からアクセス権を持つファイルとディレクトリを利用できます。ただし、macOS のプライバシー保護対象となる場所では、システムがユーザーに許可を求める場合があります。

初回起動時には、以前のサンドボックス版からサイト、設定、known hosts、PPK のコピー、REST API トークンの移行を試みます。公開ソースコードには、Apple の署名証明書、Provisioning Profile、秘密鍵、個人のサイトデータは含まれません。

## MCP 連携

IntegTERM は、Streamable HTTP を使用する標準 MCP Server を内蔵しています。MCP 対応の AI や自動化クライアントから、保存済みサイトの管理、SSH、Telnet、SFTP、FTP、macOS／Linux のローカルターミナルタブの起動、SSH コマンドの実行、対話型ターミナルの操作、ファイル転送、転送キューおよび実行ログの確認を行えます。

### MCP Server の有効化

1. IntegTERM を起動し、**設定** → **MCP** を開きます。
2. 待受ポートと IP 許可リストを確認します。デフォルトのポートは `18080`、許可リストは `127.0.0.1` です。
3. MCP Server を有効にします。デフォルトのエンドポイントは `http://127.0.0.1:18080/mcp` です。

### MCP クライアント設定

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

MCP エンドポイントは API token や独自の認証ヘッダーを必要とせず、アクセス制御は送信元 IP／CIDR 許可リストのみに依存します。信頼できるリモートクライアントからの接続が必要な場合を除き、デフォルトの `127.0.0.1` を維持し、必要以上に広いネットワークを追加しないでください。App 内の MCP ページには、現在のエンドポイントと実行中の設定から生成された完全なツールドキュメントが表示されます。クライアントはツールを使用する前に `tools/list` を呼び出し、現在のスキーマを取得してください。

## 開発環境

- Apple Silicon 搭載 macOS 12 以降、Windows 10/11 x64、または Linux x64
- Go 1.23 以降
- Node.js および npm
- macOS のビルドには Xcode Command Line Tools
- Linux のビルドには GTK3、WebKitGTK、AppIndicator、`pkg-config`
- プロジェクトのスクリプトは `go.mod` に記載された Wails v2 CLI を自動的に使用します

## 開発モードで起動

```bash
git clone https://github.com/VaderChen/Integrate-Terminal.git
cd Integrate-Terminal
./run.sh
```

`run.sh` はローカルの一時ディレクトリに開発用ミラーを作成し、外付けドライブで生成される AppleDouble ファイルが Wails のビルドに影響することを防ぎます。

## デスクトップ実行ファイルのビルド

### macOS Apple Silicon

```bash
./build.sh
```

出力先は `build/bin/IntegTERM.app` です。デフォルトでは ad-hoc 署名を使用し、App Sandbox は有効にしないため、ビルド完了後にローカル環境で直接起動できます。

### Windows x64

```powershell
powershell -ExecutionPolicy Bypass -File .\build-windows.ps1
```

出力先は `build\bin\IntegTERM.exe` です。スクリプトは x64 実行ファイルのみを作成し、インストーラーの作成や署名は行いません。

### Linux x64

```bash
./build-linux.sh
```

出力先は `build/bin/IntegTERM` です。スクリプトはインストール済みの WebKitGTK と AppIndicator のバージョンを検出し、AppImage、DEB、RPM を作成せず、x64 実行ファイルのみを生成します。

GitHub の公開版には Developer ID、Apple notarization、DMG、その他のプラットフォーム向けリリースパッケージ処理は含まれません。署名、インストーラー、配布要件は配布者が別途対応してください。

## データとセキュリティ

アプリケーションデータは、各プラットフォームの `os.UserConfigDir()` 配下にある `IntegTERM` ディレクトリに保存されます。たとえば macOS では `~/Library/Application Support/IntegTERM`、Windows では `%AppData%\IntegTERM`、Linux では `~/.config/IntegTERM` です。サイトのパスワードと PPK パスフレーズは、現在ローカルのサイトデータおよびサイトバックアップ用 ZIP ファイルに保存されます。ファイルのアクセス権を制限し、バックアップを適切に管理してください。REST/MCP サービスはデフォルトで `127.0.0.1` のみにバインドされます。外部からの接続を許可する前に、IP 許可リストを正しく設定してください。

`cert/`、`data/`、`.env*`、インストーラー、署名用資産、実際の認証情報を含むファイルはコミットしないでください。セキュリティ問題の報告方法については、[SECURITY.md](SECURITY.md) を参照してください。

## ライセンス

本プロジェクトはデュアルライセンスを採用しています。

1. オープンソースとしての利用には、[GNU General Public License v3.0](LICENSE) が適用されます。
2. GPLv3 に準拠できない場合、クローズドソースへの組み込みが必要な場合、またはその他の商用条件が必要な場合は、別途[商用ライセンス](COMMERCIAL-LICENSE.md)を取得できます。

商用ライセンスの対象は、ライセンサーが個別にライセンスする権利を持つコードと資産に限られます。第三者のパッケージ、アイコン、フォント、データセット、AI モデル、その他の第三者コンテンツは含まれず、それぞれ固有のライセンス条件が引き続き適用されます。依存関係の一覧は [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md) を参照してください。完全なライセンス本文はビルド時に生成され、配布物に含まれます。

ビルド処理では、GPLv3 の本文、第三者ライセンス文書、および `build-metadata.json` を配布物に含めます。メタデータにはソースの Git tag、commit、作業ツリーの状態が記録され、バイナリから対応するソースリビジョンを追跡できます。

正式な Contributor License Agreement が整備されるまでは、Issue による報告とディスカッションのみ受け付けます。詳細は [CONTRIBUTING.md](CONTRIBUTING.md) を参照してください。
