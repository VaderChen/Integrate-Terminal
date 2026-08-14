package app

import "github.com/VaderChen/Integrate-Terminal/internal/model"

func cloneSites(sites []model.Site) []model.Site {
	return cloneSlice(sites)
}

func cloneTabs(tabs []model.Tab) []model.Tab {
	return cloneSlice(tabs)
}

func cloneConfig(config model.Config) model.Config {
	config.RESTServerAllowlist = cloneSlice(config.RESTServerAllowlist)
	config.SiteFolders = cloneSlice(config.SiteFolders)
	return config
}

func cloneBootstrapPayload(payload model.BootstrapPayload) model.BootstrapPayload {
	payload.Sites = cloneSites(payload.Sites)
	payload.Tabs = cloneTabs(payload.Tabs)
	payload.Config = cloneConfig(payload.Config)
	payload.LocalFiles = cloneSlice(payload.LocalFiles)
	payload.RemoteFiles = cloneSlice(payload.RemoteFiles)
	payload.Transfers = cloneSlice(payload.Transfers)
	payload.Logs = cloneSlice(payload.Logs)
	return payload
}

func cloneSlice[T any](items []T) []T {
	cloned := make([]T, len(items))
	copy(cloned, items)
	return cloned
}
