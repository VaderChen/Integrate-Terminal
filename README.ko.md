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

- SSH, Telnet 및 로컬 Shell 터미널.
- SFTP 및 FTP 파일 탐색과 전송 대기열.
- 사이트 그룹, 탭 복원, ZIP 백업 및 복원.
- AI 및 자동화 도구 연동을 위한 로컬 MCP Streamable HTTP Server.
- 백그라운드 서비스 및 시스템 트레이 상태 제어.
- 영어, 일본어, 한국어, 번체 중국어 및 간체 중국어 인터페이스.

## 오픈 소스 버전

오픈 소스 버전은 StoreKit을 사용하지 않고 연결 수를 제한하지 않으며 App Sandbox를 활성화하지 않습니다. 애플리케이션은 현재 로그인한 사용자 계정에 원래 접근 권한이 있는 파일과 디렉터리에 접근할 수 있습니다. 단, macOS의 개인정보 보호 대상 위치에서는 시스템이 사용자에게 권한을 요청할 수 있습니다.

처음 실행할 때 이전 샌드박스 버전의 사이트, 설정, known hosts, PPK 복사본 및 REST API 토큰 마이그레이션을 시도합니다. 공개 소스 코드에는 Apple 서명 인증서, Provisioning Profile, 개인 키 또는 개인 사이트 데이터가 포함되지 않습니다.

## 개발 환경

- macOS 12 이상
- Go 1.23 이상
- Node.js 및 npm
- Xcode Command Line Tools
- Wails v2 CLI. 설치되어 있지 않으면 프로젝트 스크립트가 `go install`을 통해 설치합니다.

## 개발 모드 실행

```bash
git clone https://github.com/VaderChen/Integrate-Terminal.git
cd Integrate-Terminal
./run.sh
```

`run.sh`는 로컬 임시 디렉터리에 개발용 미러를 생성하여 외장 드라이브에서 만들어지는 AppleDouble 파일이 Wails 빌드를 방해하지 않도록 합니다.

## macOS App 빌드

```bash
./build.sh
```

출력 파일은 `build/bin/IntegTERM.app`에 생성됩니다. 기본적으로 ad-hoc 서명을 사용하고 App Sandbox를 활성화하지 않으므로 빌드 완료 후 로컬에서 바로 실행할 수 있습니다.

GitHub 공개 버전은 일반적인 `.app` 빌드 절차만 제공합니다. Developer ID, Apple notarization 또는 DMG 배포 스크립트는 포함하지 않습니다. 직접 빌드한 App을 배포하려면 배포자가 Apple 서명 및 배포 요구 사항을 별도로 처리해야 합니다.

## 데이터 및 보안

애플리케이션 데이터는 기본적으로 `~/Library/Application Support/IntegTERM`에 저장됩니다. 사이트 비밀번호와 PPK 암호 문구는 현재 로컬 사이트 데이터 및 사이트 백업 ZIP 파일에 저장됩니다. 파일 권한을 제한하고 백업을 안전하게 보관하십시오. REST/MCP 서비스는 기본적으로 `127.0.0.1`에만 바인딩됩니다. 외부 연결을 허용하기 전에 IP 허용 목록을 올바르게 설정하십시오.

`cert/`, `data/`, `.env*`, 설치 패키지, 서명 자산 또는 실제 인증 정보가 포함된 파일을 커밋하지 마십시오. 보안 문제 신고 방법은 [SECURITY.md](SECURITY.md)를 참조하십시오.

## 라이선스

이 프로젝트는 이중 라이선스를 사용합니다.

1. 오픈 소스 사용에는 [GNU General Public License v3.0](LICENSE)이 적용됩니다.
2. GPLv3를 준수할 수 없거나 비공개 소스 통합 또는 기타 상업적 조건이 필요한 경우 별도의 [상업용 라이선스](COMMERCIAL-LICENSE.md)를 이용할 수 있습니다.

정식 Contributor License Agreement가 마련되기 전까지는 문제 보고와 토론만 받습니다. 자세한 내용은 [CONTRIBUTING.md](CONTRIBUTING.md)를 참조하십시오.
