package app

import "testing"

func TestNormalizeMCPVFSPathUsesMCPWorkspaceRoot(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty path", input: "", want: ""},
		{name: "root path", input: "/", want: ""},
		{name: "root URI", input: mcpVFSRootURI, want: ""},
		{name: "root URI with slash", input: mcpVFSRootURI + "/", want: ""},
		{name: "child URI", input: mcpVFSRootURI + "/notes/readme.txt", want: "notes/readme.txt"},
		{name: "relative child", input: "notes/readme.txt", want: "notes/readme.txt"},
		{name: "legacy root URI", input: "integterm-vfs://workspace/", wantErr: true},
		{name: "foreign URI path", input: "integterm-vfs://workspace/other/file.txt", wantErr: true},
		{name: "parent traversal", input: mcpVFSRootURI + "/../secret", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeMCPVFSPath(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q", test.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeMCPVFSPath(%q) returned error: %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("normalizeMCPVFSPath(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestMCPVFSURIUsesMCPWorkspaceRoot(t *testing.T) {
	if got := mcpVFSURI(""); got != mcpVFSRootURI {
		t.Fatalf("mcpVFSURI(\"\") = %q, want %q", got, mcpVFSRootURI)
	}
	want := mcpVFSRootURI + "/notes/readme.txt"
	if got := mcpVFSURI("notes/readme.txt"); got != want {
		t.Fatalf("mcpVFSURI(child) = %q, want %q", got, want)
	}
}

func TestParseMCPVFSLocation(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		kind       mcpVFSLocationKind
		path       string
		siteID     string
		remotePath string
		wantErr    bool
	}{
		{name: "RAM root", input: mcpVFSRootURI, kind: mcpVFSLocationRAM, path: ""},
		{name: "RAM file", input: "notes/readme.txt", kind: mcpVFSLocationRAM, path: "notes/readme.txt"},
		{name: "sites namespace", input: "sites", kind: mcpVFSLocationSites, path: "sites"},
		{name: "site root", input: mcpVFSRootURI + "/sites/site-1", kind: mcpVFSLocationSiteRoot, path: "sites/site-1", siteID: "site-1"},
		{name: "remote file", input: mcpVFSRootURI + "/sites/site-1/config/app.yml", kind: mcpVFSLocationRemote, path: "sites/site-1/config/app.yml", siteID: "site-1", remotePath: "config/app.yml"},
		{name: "parent traversal", input: mcpVFSRootURI + "/sites/site-1/../secret", wantErr: true},
		{name: "missing site id", input: "sites/", wantErr: false, kind: mcpVFSLocationSites, path: "sites"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseMCPVFSLocation(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q", test.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseMCPVFSLocation(%q) returned error: %v", test.input, err)
			}
			if got.kind != test.kind || got.path != test.path || got.siteID != test.siteID || got.remotePath != test.remotePath {
				t.Fatalf("parseMCPVFSLocation(%q) = %#v, want kind=%q path=%q siteID=%q remotePath=%q", test.input, got, test.kind, test.path, test.siteID, test.remotePath)
			}
		})
	}
}

func TestMCPRemotePathMapping(t *testing.T) {
	mount := mcpVFSRemoteMount{SiteID: "site-1", RootPath: "/srv/app"}
	if got := remotePathForMCPMount(mount, "config/app.yml"); got != "/srv/app/config/app.yml" {
		t.Fatalf("remotePathForMCPMount() = %q, want %q", got, "/srv/app/config/app.yml")
	}
	if got := remotePathForMCPMount(mount, ""); got != "/srv/app" {
		t.Fatalf("remotePathForMCPMount(root) = %q, want %q", got, "/srv/app")
	}

	virtualPath, err := virtualRemotePath(mount, "/srv/app/config/app.yml")
	if err != nil {
		t.Fatalf("virtualRemotePath() returned error: %v", err)
	}
	if virtualPath != "sites/site-1/config/app.yml" {
		t.Fatalf("virtualRemotePath() = %q, want %q", virtualPath, "sites/site-1/config/app.yml")
	}
	if _, err := virtualRemotePath(mount, "/srv/secret.txt"); err == nil {
		t.Fatal("expected a remote path outside the site root to be rejected")
	}
}
