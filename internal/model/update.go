package model

type UpdateCheckResult struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	LatestTag       string `json:"latestTag"`
	UpdateAvailable bool   `json:"updateAvailable"`
	CanDownload     bool   `json:"canDownload"`
	AssetName       string `json:"assetName"`
}
type UpdateActionResult struct {
	Downloaded       bool `json:"downloaded"`
	InstallScheduled bool `json:"installScheduled"`
	Restarting       bool `json:"restarting"`
}
