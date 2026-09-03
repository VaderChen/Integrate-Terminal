package trayservice

import (
	"os"
	"strings"
)

type trayMessages struct {
	tooltip                   string
	statusFailed              string
	statusStopped             string
	statusRunning             string
	statusSection             string
	statusSectionHint         string
	serviceStatus             string
	localService              string
	remoteService             string
	actionSection             string
	actionSectionHint         string
	serviceStartupFailed      string
	serviceStoppedHint        string
	backgroundConnections     string
	backgroundConnectionsHint string
	backendService            string
	backendServiceHint        string
	backendUnavailable        string
	versionNumber             string
	versionHint               string
	openMainWindow            string
	openMainWindowHint        string
	clearBackground           string
	clearBackgroundHint       string
	quitBackground            string
	quitBackgroundHint        string
	runningValue              string
	stoppedValue              string
}

var trayLocales = map[string]trayMessages{
	"zh-TW": {
		tooltip:                   "IntegTERM 背景服務",
		statusFailed:              "服務狀態：啟動失敗",
		statusStopped:             "服務狀態：關閉",
		statusRunning:             "服務狀態：執行中",
		statusSection:             "狀態",
		statusSectionHint:         "服務狀態資訊",
		serviceStatus:             "服務狀態",
		localService:              "本機服務",
		remoteService:             "遠端服務",
		actionSection:             "功能",
		actionSectionHint:         "服務功能操作",
		serviceStartupFailed:      "服務啟動失敗",
		serviceStoppedHint:        "後端服務尚未執行",
		backgroundConnections:     "背景連線",
		backgroundConnectionsHint: "背景連線數量",
		backendService:            "後端服務",
		backendServiceHint:        "後端服務位址",
		backendUnavailable:        "無法取得",
		versionNumber:             "版本號碼",
		versionHint:               "目前應用程式版本",
		openMainWindow:            "開啟主視窗",
		openMainWindowHint:        "開啟 IntegTERM 主視窗",
		clearBackground:           "清除背景連線",
		clearBackgroundHint:       "關閉所有背景連線",
		quitBackground:            "結束背景服務",
		quitBackgroundHint:        "結束背景服務",
		runningValue:              "執行中",
		stoppedValue:              "關閉",
	},
	"zh-CN": {
		tooltip:                   "IntegTERM 后台服务",
		statusFailed:              "服务状态：启动失败",
		statusStopped:             "服务状态：已停止",
		statusRunning:             "服务状态：运行中",
		statusSection:             "状态",
		statusSectionHint:         "服务状态信息",
		serviceStatus:             "服务状态",
		localService:              "本机服务",
		remoteService:             "远端服务",
		actionSection:             "功能",
		actionSectionHint:         "服务功能操作",
		serviceStartupFailed:      "服务启动失败",
		serviceStoppedHint:        "后端服务尚未运行",
		backgroundConnections:     "后台连接",
		backgroundConnectionsHint: "后台连接数量",
		backendService:            "后端服务",
		backendServiceHint:        "后端服务地址",
		backendUnavailable:        "无法取得",
		versionNumber:             "版本号",
		versionHint:               "当前应用版本",
		openMainWindow:            "打开主窗口",
		openMainWindowHint:        "打开 IntegTERM 主窗口",
		clearBackground:           "清除后台连接",
		clearBackgroundHint:       "关闭所有后台连接",
		quitBackground:            "结束后台服务",
		quitBackgroundHint:        "结束后台服务",
		runningValue:              "运行中",
		stoppedValue:              "已停止",
	},
	"en": {
		tooltip:                   "IntegTERM background service",
		statusFailed:              "Service Status: Failed to start",
		statusStopped:             "Service Status: Stopped",
		statusRunning:             "Service Status: Running",
		statusSection:             "Status",
		statusSectionHint:         "Service status information",
		serviceStatus:             "Service Status",
		localService:              "Local Service",
		remoteService:             "Remote Service",
		actionSection:             "Actions",
		actionSectionHint:         "Service actions",
		serviceStartupFailed:      "Service startup failed",
		serviceStoppedHint:        "Backend service is not running",
		backgroundConnections:     "Background Connections",
		backgroundConnectionsHint: "Background connection count",
		backendService:            "Backend Service",
		backendServiceHint:        "Backend service address",
		backendUnavailable:        "Unavailable",
		versionNumber:             "Version",
		versionHint:               "Current app version",
		openMainWindow:            "Open Main Window",
		openMainWindowHint:        "Launch the main IntegTERM window",
		clearBackground:           "Clear Background Connections",
		clearBackgroundHint:       "Close all background connections",
		quitBackground:            "Quit Background Service",
		quitBackgroundHint:        "Quit the background service",
		runningValue:              "Running",
		stoppedValue:              "Stopped",
	},
	"ja": {
		tooltip:                   "IntegTERM バックグラウンドサービス",
		statusFailed:              "サービス状態：起動失敗",
		statusStopped:             "サービス状態：停止中",
		statusRunning:             "サービス状態：実行中",
		statusSection:             "状態",
		statusSectionHint:         "サービス状態情報",
		serviceStatus:             "サービス状態",
		localService:              "ローカルサービス",
		remoteService:             "リモートサービス",
		actionSection:             "機能",
		actionSectionHint:         "サービス操作",
		serviceStartupFailed:      "サービスの起動に失敗しました",
		serviceStoppedHint:        "バックエンドサービスは停止しています",
		backgroundConnections:     "バックグラウンド接続",
		backgroundConnectionsHint: "バックグラウンド接続数",
		backendService:            "バックエンドサービス",
		backendServiceHint:        "バックエンドサービスのアドレス",
		backendUnavailable:        "取得不可",
		versionNumber:             "バージョン",
		versionHint:               "現在のアプリバージョン",
		openMainWindow:            "メイン画面を開く",
		openMainWindowHint:        "IntegTERM のメイン画面を開く",
		clearBackground:           "バックグラウンド接続をクリア",
		clearBackgroundHint:       "すべてのバックグラウンド接続を閉じる",
		quitBackground:            "バックグラウンドサービスを終了",
		quitBackgroundHint:        "バックグラウンドサービスを終了します",
		runningValue:              "実行中",
		stoppedValue:              "停止中",
	},
	"ko": {
		tooltip:                   "IntegTERM 백그라운드 서비스",
		statusFailed:              "서비스 상태: 시작 실패",
		statusStopped:             "서비스 상태: 중지됨",
		statusRunning:             "서비스 상태: 실행 중",
		statusSection:             "상태",
		statusSectionHint:         "서비스 상태 정보",
		serviceStatus:             "서비스 상태",
		localService:              "로컬 서비스",
		remoteService:             "원격 서비스",
		actionSection:             "기능",
		actionSectionHint:         "서비스 기능",
		serviceStartupFailed:      "서비스 시작 실패",
		serviceStoppedHint:        "백엔드 서비스가 실행 중이 아닙니다",
		backgroundConnections:     "백그라운드 연결",
		backgroundConnectionsHint: "백그라운드 연결 수",
		backendService:            "백엔드 서비스",
		backendServiceHint:        "백엔드 서비스 주소",
		backendUnavailable:        "사용 불가",
		versionNumber:             "버전",
		versionHint:               "현재 앱 버전",
		openMainWindow:            "메인 창 열기",
		openMainWindowHint:        "IntegTERM 메인 창 열기",
		clearBackground:           "백그라운드 연결 정리",
		clearBackgroundHint:       "모든 백그라운드 연결 닫기",
		quitBackground:            "백그라운드 서비스 종료",
		quitBackgroundHint:        "백그라운드 서비스를 종료합니다",
		runningValue:              "실행 중",
		stoppedValue:              "중지됨",
	},
}

func resolveTrayLocale(language string) string {
	if locale := normalizeTrayLocale(language); locale != "" {
		return locale
	}

	for _, candidate := range []string{
		os.Getenv("LC_ALL"),
		os.Getenv("LC_MESSAGES"),
		os.Getenv("LANG"),
	} {
		if locale := normalizeTrayLocale(candidate); locale != "" {
			return locale
		}
	}

	return "zh-TW"
}

func normalizeTrayLocale(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return ""
	}
	if dot := strings.Index(normalized, "."); dot >= 0 {
		normalized = normalized[:dot]
	}
	if at := strings.Index(normalized, "@"); at >= 0 {
		normalized = normalized[:at]
	}
	if normalized == "zh-tw" || normalized == "zh_tw" || normalized == "zh-hk" || normalized == "zh_hk" || normalized == "zh-mo" || normalized == "zh_mo" {
		return "zh-TW"
	}
	if normalized == "zh-cn" || normalized == "zh_cn" || normalized == "zh-sg" || normalized == "zh_sg" {
		return "zh-CN"
	}
	if strings.HasPrefix(normalized, "en") {
		return "en"
	}
	if strings.HasPrefix(normalized, "ja") {
		return "ja"
	}
	if strings.HasPrefix(normalized, "ko") {
		return "ko"
	}
	if strings.HasPrefix(normalized, "zh") {
		return "zh-CN"
	}
	return ""
}

func (s *Service) messages() trayMessages {
	cfg := s.app.ReloadConfig()
	locale := resolveTrayLocale(cfg.Language)
	if messages, ok := trayLocales[locale]; ok {
		return messages
	}
	return trayLocales["zh-TW"]
}
