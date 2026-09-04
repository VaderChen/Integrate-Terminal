<div align="center">
  <img src="assets/appicon.png" alt="Integrate Terminal 아이콘" width="128" />
  <h1>Integrate Terminal</h1>
  <p>Go, Wails, React 및 TypeScript로 개발한 다중 프로토콜 데스크톱 터미널 및 파일 전송 도구입니다.</p>
</div>

<p align="center">
  <a href="README.md">繁體中文</a> |
  <a href="README.en.md">English</a> |
  <a href="README.ja.md">日本語</a> |
  <a href="README.ko.md">한국어</a>
</p>

## 기능

- SSH, Telnet 및 로컬 Shell 터미널(로컬 Shell은 macOS와 Linux에서 지원).
- SFTP 및 FTP 파일 탐색과 전송 대기열.
- 사이트 그룹, 탭 복원, ZIP 백업 및 복원.
- RAM 작업 공간과 원격 사이트 마운트를 제공하는 MCP 가상 계층.
- 백그라운드 서비스 및 시스템 트레이 상태 제어.
- 설정에서 GitHub Release를 수동으로 확인하고 매일 임의의 시간에 한 번 자동 확인하며, 검증된 플랫폼 업데이트 파일을 여는 기능.
- 영어, 일본어, 한국어, 번체 중국어 및 간체 중국어 인터페이스.

## 오픈 소스 버전

오픈 소스 버전은 StoreKit을 사용하지 않고 연결 수를 제한하지 않으며 App Sandbox를 활성화하지 않습니다. 애플리케이션은 현재 로그인한 사용자 계정에 원래 접근 권한이 있는 파일과 디렉터리에 접근할 수 있습니다. 단, macOS의 개인정보 보호 대상 위치에서는 시스템이 사용자에게 권한을 요청할 수 있습니다.

처음 실행할 때 이전 샌드박스 버전의 사이트, 설정, known hosts, PPK 복사본 및 REST API 토큰 마이그레이션을 시도합니다. 공개 소스 코드에는 Apple 서명 인증서, Provisioning Profile, 개인 키 또는 개인 사이트 데이터가 포함되지 않습니다.

## 사이트 및 SSH 호스트 신뢰

사이트 편집기에서 **사이트 즐겨찾기**는 양식의 오른쪽 아래에 배치되며 **설정** 화면과 동일한 Switch 컨트롤을 사용합니다. SSH／SFTP에 처음 연결하거나 저장된 호스트 지문이 서버의 현재 지문과 다르면 IntegTERM은 호스트 신뢰 대화 상자를 표시하고 SHA-256 지문 확인을 요청합니다.

호스트 신뢰 기록은 승인한 경우에만 변경됩니다. 다시 승인하면 동일한 호스트와 키 유형의 기존 항목을 교체하고, 취소하면 기존 기록을 유지한 채 연결을 중단합니다. 서버를 최근에 재구축하지 않았거나 호스트 키를 의도적으로 교체하지 않았다면 승인하기 전에 호스트 관리자에게 새 지문을 확인하십시오.

## MCP 연동 (VFS 기본 활성화)

IntegTERM은 stdio 기반 로컬 VFS MCP와 선택적 Streamable HTTP MCP를 제공합니다. 가상 계층을 통해 RAM 작업 공간과 저장된 원격 사이트 마운트를 하나로 연결합니다. `integterm-vfs://`는 MCP 연결 내부의 리소스 URI／도구 경로이며 Agent가 직접 연결하는 URL이 아닙니다.

### 로컬 VFS MCP (stdio, 기본값)

로컬 Agent는 `mcp` 인자로 IntegTERM을 시작하고 stdio로 연결해야 합니다.

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

이 구성은 컴파일된 IntegTERM 실행 파일을 헤드리스 MCP 모드로 직접 실행합니다. 소스 트리는 필요하지 않으며 데스크톱 UI도 열리지 않습니다. 각 stdio 클라이언트는 독립적인 MCP 프로세스와 RAM 작업 공간을 시작하고 소유하므로, 이 작업 공간은 디스크의 공유 파일이 아니며 다른 Agent와 자동으로 공유되지 않습니다.

소스에서 실행할 때는 `go run . mcp`를 사용합니다. 연결 후 `tools/list`를 호출하고 빈 path 또는 `integterm-vfs://workspace/mcp`로 `vfs_list`를 사용하십시오. URI를 HTTP URL 필드에 넣거나 셸 명령으로 실행하지 마십시오.

### Agent 호출 순서

연결 후 다음 순서로 호출하십시오.

1. `tools/list`를 호출하여 현재 도구 스키마를 확인합니다.
2. `vfs_workspace_info`를 호출하여 루트 URI와 작업 공간 제한을 확인합니다.
3. `path`를 생략한 `vfs_list`(또는 `integterm-vfs://workspace/mcp`)로 루트를 나열합니다.
4. 파일에는 `vfs_stat`, `vfs_read`, `vfs_write`, `vfs_write_chunk` 등의 VFS 도구를 사용합니다. MCP Resources를 지원하는 클라이언트만 Resource URI에 `resources/read`를 사용합니다.

MCP `initialize` 응답은 전체 VFS 작업 흐름을 제공하고 `tools/list`는 현재 입력 및 출력 스키마를 제공합니다. 이 두 응답이 Agent의 공식 작업 기준이므로 IntegTERM 소스 코드를 확인할 필요가 없습니다.

`integterm-vfs://workspace/mcp` 값은 연결을 시작하지 않으며 셸 명령으로 실행해서도 안 됩니다.

### MCP Server (HTTP 기본 비활성화)

1. IntegTERM을 실행하고 **설정** → **MCP**를 엽니다.
2. 로컬 VFS MCP는 기본적으로 stdio로 제공되며 네트워크 포트를 열지 않습니다.
3. 외부 Agent가 연결해야 하거나 여러 Agent가 하나의 서버 측 RAM 작업 공간을 공유해야 할 때만 HTTP MCP Server를 활성화합니다. 모든 Agent는 동일한 엔드포인트에 연결해야 합니다. 기본 포트는 `18080`, 기본 허용 목록은 `127.0.0.1`, 엔드포인트는 `http://127.0.0.1:18080/mcp`입니다.

### 가상 작업 공간: RAM 및 원격 사이트

가상 루트 URI는 `integterm-vfs://workspace/mcp`입니다. `sites` 네임스페이스 외부의 경로는 제한된 RAM 데이터이고, `sites/{siteID}`는 저장된 원격 사이트를 나타냅니다. 최초의 `vfs_connect` 또는 파일 작업 시 연결이 지연 생성됩니다. RAM 데이터는 이를 소유한 stdio MCP 프로세스 또는 Streamable HTTP 백그라운드 서비스가 중지되면 삭제됩니다. 동일한 HTTP MCP 서비스에 연결된 Agent는 해당 서비스의 RAM 작업 공간을 공유하며, 원격 작업은 사이트에 설정된 원격 루트에 직접 적용됩니다.

원격 사이트 경로 형식은 `integterm-vfs://workspace/mcp/sites/{siteID}/{relativeRemotePath}`입니다. `sites`를 먼저 나열하여 사이트 ID를 확인한 다음 `vfs_list`, `vfs_stat`, `vfs_read`, `vfs_write`, `vfs_write_chunk`, `vfs_mkdir`, `vfs_rename`, `vfs_delete`로 일반 파일 작업을 수행합니다. 가상 URI에는 비밀번호나 개인 키가 포함되지 않습니다.

`vfs_write`는 4 MiB 이하의 단일 페이로드에 사용합니다. 더 큰 파일은 반환된 `nextOffset`을 사용하여 `vfs_write_chunk`를 순서대로 호출합니다. 각 chunk는 최대 1 MiB, 완성 파일은 최대 32 MiB이며 마지막 호출에는 `final: true`와 전체 파일의 SHA-256을 지정합니다. Resources 지원 클라이언트는 도구가 반환한 중첩 `integterm-vfs` URI를 직접 읽을 수 있습니다.

### 네트워크 사용: 기존 작업 및 가상 작업 공간

외부 Agent는 MCP 엔드포인트 `http://127.0.0.1:18080/mcp`를 사용하며 저장된 사이트 관리, SSH, Telnet, SFTP, FTP, 로컬 터미널, 명령, 대화형 터미널, 파일 전송 및 위의 `integterm-vfs` 가상 작업 공간 도구를 사용할 수 있습니다. `integterm-vfs://`는 리소스 URI이며 별도의 HTTP 엔드포인트가 아닙니다.

### 네트워크 MCP 클라이언트 설정

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

MCP 엔드포인트에는 API token이나 사용자 지정 인증 헤더가 필요하지 않으며 접근 제어는 소스 IP/CIDR 허용 목록에 전적으로 의존합니다. 신뢰할 수 있는 원격 클라이언트의 연결이 반드시 필요한 경우가 아니라면 기본값인 `127.0.0.1`을 유지하고 불필요하게 넓은 네트워크를 추가하지 마십시오. App 내 MCP 페이지에는 현재 엔드포인트와 실행 중인 설정에서 생성된 전체 도구 문서가 표시됩니다. 클라이언트는 도구를 사용하기 전에 `tools/list`를 호출하여 현재 스키마를 확인해야 합니다.

## 개발 환경

- Apple Silicon 기반 macOS 12 이상, Windows 10/11 x64 또는 Linux x64
- Go 1.23 이상
- Node.js 및 npm
- macOS 빌드용 Xcode Command Line Tools
- Linux 빌드용 GTK3, WebKitGTK, AppIndicator 및 `pkg-config`
- 프로젝트 스크립트는 `go.mod`에 선언된 Wails v2 CLI 버전을 자동으로 사용합니다.

## 개발 모드 실행

```bash
git clone https://github.com/VaderChen/Integrate-Terminal.git
cd Integrate-Terminal
./run.sh
```

`run.sh`는 로컬 임시 디렉터리에 개발용 미러를 생성하여 외장 드라이브에서 만들어지는 AppleDouble 파일이 Wails 빌드를 방해하지 않도록 합니다.

UI는 기본적으로 단일 인스턴스로 실행됩니다. 다시 실행하면 새 UI를 만들지 않고 기존 창을 앞으로 가져옵니다. 개발 중 UI를 의도적으로 여러 개 실행하려면 다음 특수 인자를 사용하십시오.

```bash
./run.sh --multi-instance
```

이 인자는 UI만 여러 개 실행하도록 하며 백그라운드 서비스는 항상 단일 인스턴스로 유지됩니다.

## 데스크톱 실행 파일 빌드

### macOS Apple Silicon

```bash
./build.sh
```

출력 파일은 `dist/IntegTERM.app`에 생성됩니다. 기본적으로 ad-hoc 서명을 사용하고 App Sandbox를 활성화하지 않으므로 빌드 완료 후 로컬에서 바로 실행할 수 있습니다. `build.command`를 두 번 클릭하여 빌드하고 `run.command`로 빌드된 App을 실행할 수 있으며, 개발 모드는 `run.command --dev`를 사용합니다.

버전 정보를 주입하지 않으면 각 빌드는 이전 태그나 `wails.json`을 재사용하지 않고 현재 시스템 시간에서 `1.YY.MMDD build HHmm`를 생성합니다. 재현 가능한 빌드는 ISO 8601 형식의 `BUILD_TIMESTAMP` 또는 표준 `SOURCE_DATE_EPOCH`를 주입할 수 있으며, `APP_MARKETING_VERSION`, `APP_BUILD_LABEL`, `APP_BUNDLE_VERSION`으로 각 필드를 명시적으로 재정의할 수 있습니다.

### Windows x64

```powershell
powershell -ExecutionPolicy Bypass -File .\build-windows.ps1
```

출력 파일은 `dist\IntegTERM.exe`에 생성됩니다. 스크립트는 x64 실행 파일만 만들며 설치 프로그램 생성이나 서명은 수행하지 않습니다.

### Linux x64

```bash
./build-linux.sh
```

출력 파일은 `dist/IntegTERM`에 생성됩니다. 스크립트는 설치된 WebKitGTK와 AppIndicator 버전을 감지하며 AppImage, DEB 또는 RPM 없이 x64 실행 파일만 생성합니다.

GitHub 공개 버전에는 서명 식별 정보, 공증 설정, 개인 키 또는 기타 릴리스 자격 증명이 포함되지 않습니다. 서명, 설치 프로그램 및 배포 요구 사항은 배포자가 별도로 처리해야 합니다.

## 데이터 및 보안

애플리케이션 데이터는 각 플랫폼의 `os.UserConfigDir()` 아래 `IntegTERM` 디렉터리에 저장됩니다. 예를 들어 macOS에서는 `~/Library/Application Support/IntegTERM`, Windows에서는 `%AppData%\IntegTERM`, Linux에서는 `~/.config/IntegTERM`입니다. 사이트 비밀번호와 PPK 암호 문구는 현재 로컬 사이트 데이터 및 사이트 백업 ZIP 파일에 저장됩니다. 파일 권한을 제한하고 백업을 안전하게 보관하십시오. REST/MCP 서비스는 기본적으로 `127.0.0.1`에만 바인딩됩니다. 외부 연결을 허용하기 전에 IP 허용 목록을 올바르게 설정하십시오.

`cert/`, `data/`, `.env*`, 설치 패키지, 서명 자산 또는 실제 인증 정보가 포함된 파일을 커밋하지 마십시오. 보안 문제 신고 방법은 [SECURITY.md](SECURITY.md)를 참조하십시오.

## 라이선스

이 프로젝트는 이중 라이선스를 사용합니다.

1. 오픈 소스 사용에는 [GNU General Public License v3.0](LICENSE)이 적용됩니다.
2. GPLv3를 준수할 수 없거나 비공개 소스 통합 또는 기타 상업적 조건이 필요한 경우 별도의 [상업용 라이선스](COMMERCIAL-LICENSE.md)를 이용할 수 있습니다.

상업용 라이선스는 라이선스 제공자가 별도로 라이선스할 권리를 가진 코드와 자산에만 적용됩니다. 타사 패키지, 아이콘, 글꼴, 데이터 세트, AI 모델 및 기타 타사 콘텐츠는 포함되지 않으며 각각의 라이선스 조건이 계속 적용됩니다. 의존성 목록은 [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md)를 참조하십시오. 전체 라이선스 전문은 빌드 시 생성되어 배포 결과물에 포함됩니다.

빌드 과정은 GPLv3 전문, 타사 라이선스 문서 및 `build-metadata.json`을 배포 결과물에 포함합니다. 메타데이터에는 소스 Git tag, commit 및 작업 트리 상태가 기록되어 바이너리를 해당 소스 리비전으로 추적할 수 있습니다.

정식 Contributor License Agreement가 마련되기 전까지는 문제 보고와 토론만 받습니다. 자세한 내용은 [CONTRIBUTING.md](CONTRIBUTING.md)를 참조하십시오.
