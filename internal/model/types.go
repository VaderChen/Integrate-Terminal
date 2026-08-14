package model

type Site struct {
	ID                      string   `json:"id"`
	Name                    string   `json:"name"`
	Folder                  string   `json:"folder,omitempty"`
	Protocol                string   `json:"protocol"`
	ProtocolLabel           string   `json:"protocolLabel,omitempty"`
	SupportedModes          []string `json:"supportedModes,omitempty"`
	PrimaryFileProtocol     string   `json:"primaryFileProtocol,omitempty"`
	PrimaryTerminalProtocol string   `json:"primaryTerminalProtocol,omitempty"`
	Host                    string   `json:"host"`
	Port                    int      `json:"port"`
	Username                string   `json:"username"`
	Password                string   `json:"password"`
	PPKPath                 string   `json:"ppkPath"`
	PPKPassphrase           string   `json:"ppkPassphrase"`
	LocalPath               string   `json:"localPath"`
	RemotePath              string   `json:"remotePath"`
	LastUsedAt              string   `json:"lastUsedAt"`
}

type Tab struct {
	ID            string `json:"id"`
	SiteID        string `json:"siteId"`
	Title         string `json:"title"`
	Mode          string `json:"mode"`
	Protocol      string `json:"protocol"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	PPKPath       string `json:"ppkPath"`
	PPKPassphrase string `json:"ppkPassphrase"`
	LocalPath     string `json:"localPath"`
	RemotePath    string `json:"remotePath"`
	SessionID     string `json:"sessionId"`
	Connected     bool   `json:"connected"`
	Hidden        bool   `json:"hidden"`
}

type FileEntry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
	IsDir    bool   `json:"isDir"`
	Side     string `json:"side"`
}

type TransferItem struct {
	ID        string `json:"id"`
	Direction string `json:"direction"`
	Name      string `json:"name"`
	Progress  int    `json:"progress"`
	SpeedBps  int64  `json:"speedBps"`
	Status    string `json:"status"`
}

type LogItem struct {
	ID        string `json:"id"`
	Message   string `json:"message"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type Config struct {
	WindowWidth                  int      `json:"windowWidth"`
	WindowHeight                 int      `json:"windowHeight"`
	WindowX                      int      `json:"windowX"`
	WindowY                      int      `json:"windowY"`
	LastActiveTab                string   `json:"lastActiveTab"`
	RestoreTabsOnStart           bool     `json:"restoreTabsOnStart"`
	CloseTerminalTabOnDisconnect bool     `json:"closeTerminalTabOnDisconnect"`
	ShowHiddenFiles              bool     `json:"showHiddenFiles"`
	ShowTrayIcon                 bool     `json:"showTrayIcon"`
	RememberWindowPosition       bool     `json:"rememberWindowPosition"`
	TelnetLocalEcho              bool     `json:"telnetLocalEcho"`
	RESTServerEnabled            bool     `json:"restServerEnabled"`
	RESTServerPort               int      `json:"restServerPort"`
	RESTServerAllowlist          []string `json:"restServerAllowlist"`
	FontScale                    string   `json:"fontScale"`
	Language                     string   `json:"language"`
	Theme                        string   `json:"theme"`
	SiteFolders                  []string `json:"siteFolders"`
}

type SiteLibraryMutationResult struct {
	Sites  []Site `json:"sites"`
	Config Config `json:"config"`
}

type BootstrapPayload struct {
	Sites            []Site         `json:"sites"`
	Tabs             []Tab          `json:"tabs"`
	Config           Config         `json:"config"`
	DefaultLocalPath string         `json:"defaultLocalPath"`
	LocalFiles       []FileEntry    `json:"localFiles"`
	RemoteFiles      []FileEntry    `json:"remoteFiles"`
	Transfers        []TransferItem `json:"transfers"`
	Logs             []LogItem      `json:"logs"`
}

type RESTServerStatus struct {
	Enabled   bool     `json:"enabled"`
	Running   bool     `json:"running"`
	BaseURL   string   `json:"baseURL"`
	MCPURL    string   `json:"mcpURL"`
	Port      int      `json:"port"`
	Attached  bool     `json:"attached"`
	Allowlist []string `json:"allowlist"`
}

type HostTrustPrompt struct {
	Host              string `json:"host"`
	Port              int    `json:"port"`
	HostPattern       string `json:"hostPattern"`
	KeyType           string `json:"keyType"`
	FingerprintSHA256 string `json:"fingerprintSHA256"`
	AuthorizedKey     string `json:"authorizedKey"`
}
