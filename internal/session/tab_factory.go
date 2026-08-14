package session

import (
	"fmt"
	"time"

"github.com/VaderChen/Integrate-Terminal/internal/model"
	"github.com/VaderChen/Integrate-Terminal/internal/transport"
)

func newClient(protocol string) (transport.Client, error) {
	switch protocol {
	case "ftp":
		return &transport.FTPClient{}, nil
	case "sftp":
		return &transport.SFTPClient{}, nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}
}

func MakeTab(site model.Site) model.Tab {
	return model.Tab{
		ID:            fmt.Sprintf("tab-%d", time.Now().UnixNano()),
		SiteID:        site.ID,
		Title:         site.Name,
		Mode:          "file",
		Protocol:      site.Protocol,
		Host:          site.Host,
		Port:          site.Port,
		Username:      site.Username,
		Password:      site.Password,
		PPKPath:       site.PPKPath,
		PPKPassphrase: site.PPKPassphrase,
		LocalPath:     site.LocalPath,
		RemotePath:    site.RemotePath,
		Connected:     false,
	}
}

func MakeSSHTab(site model.Site, sessionID string) model.Tab {
	return model.Tab{
		ID:            fmt.Sprintf("tab-%d", time.Now().UnixNano()),
		SiteID:        site.ID,
		Title:         site.Name,
		Mode:          "terminal",
		Protocol:      "ssh",
		Host:          site.Host,
		Port:          site.Port,
		Username:      site.Username,
		Password:      site.Password,
		PPKPath:       site.PPKPath,
		PPKPassphrase: site.PPKPassphrase,
		LocalPath:     site.LocalPath,
		RemotePath:    site.RemotePath,
		SessionID:     sessionID,
		Connected:     true,
	}
}

func MakeTelnetTab(site model.Site, sessionID string) model.Tab {
	return model.Tab{
		ID:            fmt.Sprintf("tab-%d", time.Now().UnixNano()),
		SiteID:        site.ID,
		Title:         site.Name,
		Mode:          "terminal",
		Protocol:      "telnet",
		Host:          site.Host,
		Port:          site.Port,
		Username:      site.Username,
		Password:      site.Password,
		PPKPath:       site.PPKPath,
		PPKPassphrase: site.PPKPassphrase,
		LocalPath:     site.LocalPath,
		RemotePath:    site.RemotePath,
		SessionID:     sessionID,
		Connected:     true,
	}
}

func MakeLocalTerminalTab(sessionID string, cwd string) model.Tab {
	resolvedPath := resolveLocalTerminalPath(cwd)
	return model.Tab{
		ID:        fmt.Sprintf("tab-%d", time.Now().UnixNano()),
		Title:     "Local Terminal",
		Mode:      "terminal",
		Protocol:  "local",
		LocalPath: resolvedPath,
		SessionID: sessionID,
		Connected: true,
	}
}

func isHiddenName(name string) bool {
	return len(name) > 0 && name[0] == '.'
}
