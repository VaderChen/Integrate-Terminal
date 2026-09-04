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
- RAM ワークスペースとリモートサイトのマウントを提供する MCP 仮想レイヤー。
- バックグラウンドサービスとシステムトレイからの状態制御。
- 設定画面からの GitHub Release の手動確認に加え、毎日ランダムな時刻に一度自動確認し、検証済みのプラットフォーム向け更新ファイルを開く機能。
- 英語、日本語、韓国語、繁体字中国語、簡体字中国語のインターフェース。

## オープンソース版

オープンソース版は StoreKit を使用せず、接続数を制限せず、App Sandbox も有効にしません。現在ログインしているユーザーアカウントが元からアクセス権を持つファイルとディレクトリを利用できます。ただし、macOS のプライバシー保護対象となる場所では、システムがユーザーに許可を求める場合があります。

初回起動時には、以前のサンドボックス版からサイト、設定、known hosts、PPK のコピー、REST API トークンの移行を試みます。公開ソースコードには、Apple の署名証明書、Provisioning Profile、秘密鍵、個人のサイトデータは含まれません。

## サイトと SSH ホストの信頼

サイト編集画面では、「お気に入りサイト」がフォーム右下に配置され、「設定」画面と同じ Switch で操作します。SSH／SFTP への初回接続時、または保存済みホストフィンガープリントとサーバーの現在のフィンガープリントが異なる場合、IntegTERM はホスト信頼ダイアログを表示し、SHA-256 フィンガープリントの確認を求めます。

ホスト信頼記録は、承認した場合にのみ更新されます。再承認すると同じホストかつ同じ鍵タイプの古い記録が置き換えられ、キャンセルすると既存の記録を保持したまま接続を中止します。サーバーを最近再構築しておらず、ホスト鍵の意図的なローテーションもない場合は、承認前にホスト管理者へ新しいフィンガープリントを確認してください。

## MCP 連携（VFS はデフォルトで有効）

IntegTERM は stdio のローカル VFS MCP と、オプションの Streamable HTTP MCP を内蔵しています。仮想層によって RAM ワークスペースと保存済みリモートサイトのマウントを統一します。`integterm-vfs://` は MCP 接続内の Resource URI／ツールパスであり、Agent が直接接続する URL ではありません。

### ローカル VFS MCP（stdio、既定）

ローカル Agent は `mcp` 引数で IntegTERM を起動し、stdio で接続します。

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

この設定は、コンパイル済みの IntegTERM 実行ファイルをヘッドレス MCP モードで直接起動します。ソースツリーは不要で、デスクトップ UI も開きません。各 stdio クライアントは独立した MCP プロセスと RAM ワークスペースを起動して所有するため、このワークスペースはディスク上の共有ファイルではなく、他の Agent と自動的に共有されることもありません。

ソースから実行する場合は `go run . mcp` を使用します。接続後に `tools/list` を呼び出し、空の path または `integterm-vfs://workspace/mcp` を指定して `vfs_list` を使用してください。この URI を HTTP URL 欄に入れたり、shell コマンドとして実行したりしないでください。

### Agent の呼び出し手順

接続後は次の順序で実行します。

1. `tools/list` を呼び出して現在のツールスキーマを取得します。
2. `vfs_workspace_info` を呼び出してルート URI とワークスペース制限を確認します。
3. `path` を省略した `vfs_list`（または `integterm-vfs://workspace/mcp`）でルートを一覧表示します。
4. ファイル操作には `vfs_stat`、`vfs_read`、`vfs_write`、`vfs_write_chunk` などを使用します。MCP Resources に対応するクライアントだけが Resource URI に `resources/read` を使用します。

MCP の `initialize` 応答には完全な VFS ワークフローが含まれ、`tools/list` には現在の入出力スキーマが含まれます。これらが Agent の正式な操作基準であり、IntegTERM のソースコードを調べる必要はありません。

`integterm-vfs://workspace/mcp` は接続を開始する URL ではなく、shell コマンドとして実行するものでもありません。

### MCP Server（HTTP はデフォルトで無効）

1. IntegTERM を起動し、**設定** → **MCP** を開きます。
2. ローカル VFS MCP は stdio で既定提供され、ネットワークポートは待ち受けません。
3. 外部 Agent が接続する場合、または複数の Agent が同じサーバー側 RAM ワークスペースを共有する場合のみ HTTP MCP Server を有効にします。すべての Agent は同じエンドポイントへ接続する必要があります。デフォルトのポートは `18080`、許可リストは `127.0.0.1`、エンドポイントは `http://127.0.0.1:18080/mcp` です。

### 仮想ワークスペース: RAM とリモートサイト

仮想ルート URI は `integterm-vfs://workspace/mcp` です。`sites` 名前空間以外のパスは RAM データ、`sites/{siteID}` は保存済みリモートサイトを表します。最初の `vfs_connect` またはファイル操作で接続を遅延確立します。RAM データは、そのデータを所有する stdio MCP プロセスまたは Streamable HTTP バックグラウンドサービスの停止時に消去されます。同じ HTTP MCP サービスに接続した Agent は、そのサービスの RAM ワークスペースを共有します。リモート操作はサイトに設定されたリモートルートへ直接適用されます。

リモートサイトのパス形式は `integterm-vfs://workspace/mcp/sites/{siteID}/{relativeRemotePath}` です。`sites` を一覧表示してサイト ID を取得し、`vfs_list`、`vfs_stat`、`vfs_read`、`vfs_write`、`vfs_write_chunk`、`vfs_mkdir`、`vfs_rename`、`vfs_delete` で通常のファイル操作を行います。仮想 URI にパスワードや秘密鍵が含まれることはありません。

`vfs_write` は 4 MiB までの単一ペイロードに使用します。より大きなファイルは、返された `nextOffset` を使って `vfs_write_chunk` を順番に呼び出します。各 chunk は 1 MiB、完成ファイルは 32 MiB までで、最後の呼び出しでは `final: true` と完成ファイル全体の SHA-256 を指定します。Resources 対応クライアントは、ツールが返した入れ子の `integterm-vfs` URI を直接読み取れます。

### ネットワーク経由: 既存の操作と仮想ワークスペース

外部 Agent は MCP エンドポイント `http://127.0.0.1:18080/mcp` を使用します。保存済みサイト、SSH、Telnet、SFTP、FTP、ローカル Terminal、コマンド、対話型 Terminal、ファイル転送、および上記の `integterm-vfs` 仮想ワークスペースツールを提供します。`integterm-vfs://` はリソース URI であり、別の HTTP エンドポイントではありません。

### ネットワーク MCP クライアント設定

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

UI はデフォルトで単一インスタンスとして動作します。再度起動すると新しい UI は作成されず、既存のウィンドウが前面に表示されます。開発時に UI を意図的に複数起動する場合は、次の引数を使用してください。

```bash
./run.sh --multi-instance
```

この引数で複数起動できるのは UI のみで、バックグラウンドサービスは常に単一インスタンスです。

## デスクトップ実行ファイルのビルド

### macOS Apple Silicon

```bash
./build.sh
```

出力先は `dist/IntegTERM.app` です。デフォルトでは ad-hoc 署名を使用し、App Sandbox は有効にしないため、ビルド完了後にローカル環境で直接起動できます。`build.command` をダブルクリックしてビルドし、`run.command` でビルド済み App を起動できます。開発モードは `run.command --dev` を使用してください。

バージョン情報を注入しない場合、各ビルドは古いタグや `wails.json` を再利用せず、現在のシステム時刻から `1.YY.MMDD build HHmm` を生成します。再現可能なビルドでは ISO 8601 形式の `BUILD_TIMESTAMP` または標準の `SOURCE_DATE_EPOCH` を注入でき、`APP_MARKETING_VERSION`、`APP_BUILD_LABEL`、`APP_BUNDLE_VERSION` で各項目を明示的に上書きできます。

### Windows x64

```powershell
powershell -ExecutionPolicy Bypass -File .\build-windows.ps1
```

出力先は `dist\IntegTERM.exe` です。スクリプトは x64 実行ファイルのみを作成し、インストーラーの作成や署名は行いません。

### Linux x64

```bash
./build-linux.sh
```

出力先は `dist/IntegTERM` です。スクリプトはインストール済みの WebKitGTK と AppIndicator のバージョンを検出し、AppImage、DEB、RPM を作成せず、x64 実行ファイルのみを生成します。

GitHub の公開版には署名識別情報、公証設定、秘密鍵、その他のリリース資格情報は含まれません。署名、インストーラー、配布要件は配布者が別途対応してください。

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
