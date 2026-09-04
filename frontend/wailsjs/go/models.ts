export namespace model {
	
	export class LogItem {
	    id: string;
	    message: string;
	    status: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new LogItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.message = source["message"];
	        this.status = source["status"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class TransferItem {
	    id: string;
	    direction: string;
	    name: string;
	    progress: number;
	    speedBps: number;
	    status: string;
	    attempt: number;
	    maxAttempts: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new TransferItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.direction = source["direction"];
	        this.name = source["name"];
	        this.progress = source["progress"];
	        this.speedBps = source["speedBps"];
	        this.status = source["status"];
	        this.attempt = source["attempt"];
	        this.maxAttempts = source["maxAttempts"];
	        this.error = source["error"];
	    }
	}
	export class FileEntry {
	    name: string;
	    path: string;
	    size: number;
	    modified: string;
	    isDir: boolean;
	    side: string;
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.modified = source["modified"];
	        this.isDir = source["isDir"];
	        this.side = source["side"];
	    }
	}
	export class Config {
	    windowWidth: number;
	    windowHeight: number;
	    windowX: number;
	    windowY: number;
	    lastActiveTab: string;
	    restoreTabsOnStart: boolean;
	    closeTerminalTabOnDisconnect: boolean;
	    showHiddenFiles: boolean;
	    showTrayIcon: boolean;
	    rememberWindowPosition: boolean;
	    telnetLocalEcho: boolean;
	    restServerEnabled: boolean;
	    restServerPort: number;
	    restServerAllowlist: string[];
	    fontScale: string;
	    language: string;
	    theme: string;
	    siteFolders: string[];
	    transferRetryCount: number;
	    transferConflictStrategy: string;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.windowWidth = source["windowWidth"];
	        this.windowHeight = source["windowHeight"];
	        this.windowX = source["windowX"];
	        this.windowY = source["windowY"];
	        this.lastActiveTab = source["lastActiveTab"];
	        this.restoreTabsOnStart = source["restoreTabsOnStart"];
	        this.closeTerminalTabOnDisconnect = source["closeTerminalTabOnDisconnect"];
	        this.showHiddenFiles = source["showHiddenFiles"];
	        this.showTrayIcon = source["showTrayIcon"];
	        this.rememberWindowPosition = source["rememberWindowPosition"];
	        this.telnetLocalEcho = source["telnetLocalEcho"];
	        this.restServerEnabled = source["restServerEnabled"];
	        this.restServerPort = source["restServerPort"];
	        this.restServerAllowlist = source["restServerAllowlist"];
	        this.fontScale = source["fontScale"];
	        this.language = source["language"];
	        this.theme = source["theme"];
	        this.siteFolders = source["siteFolders"];
	        this.transferRetryCount = source["transferRetryCount"];
	        this.transferConflictStrategy = source["transferConflictStrategy"];
	    }
	}
	export class Tab {
	    id: string;
	    siteId: string;
	    title: string;
	    mode: string;
	    protocol: string;
	    host: string;
	    port: number;
	    username: string;
	    password: string;
	    ppkPath: string;
	    ppkPassphrase: string;
	    localPath: string;
	    remotePath: string;
	    sessionId: string;
	    connected: boolean;
	    hidden: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Tab(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.siteId = source["siteId"];
	        this.title = source["title"];
	        this.mode = source["mode"];
	        this.protocol = source["protocol"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.ppkPath = source["ppkPath"];
	        this.ppkPassphrase = source["ppkPassphrase"];
	        this.localPath = source["localPath"];
	        this.remotePath = source["remotePath"];
	        this.sessionId = source["sessionId"];
	        this.connected = source["connected"];
	        this.hidden = source["hidden"];
	    }
	}
	export class Site {
	    id: string;
	    name: string;
	    folder?: string;
	    protocol: string;
	    protocolLabel?: string;
	    supportedModes?: string[];
	    primaryFileProtocol?: string;
	    primaryTerminalProtocol?: string;
	    host: string;
	    port: number;
	    username: string;
	    password: string;
	    ppkPath: string;
	    ppkPassphrase: string;
	    localPath: string;
	    remotePath: string;
	    lastUsedAt: string;
	    tags: string[];
	    favorite: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Site(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.folder = source["folder"];
	        this.protocol = source["protocol"];
	        this.protocolLabel = source["protocolLabel"];
	        this.supportedModes = source["supportedModes"];
	        this.primaryFileProtocol = source["primaryFileProtocol"];
	        this.primaryTerminalProtocol = source["primaryTerminalProtocol"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.ppkPath = source["ppkPath"];
	        this.ppkPassphrase = source["ppkPassphrase"];
	        this.localPath = source["localPath"];
	        this.remotePath = source["remotePath"];
	        this.lastUsedAt = source["lastUsedAt"];
	        this.tags = source["tags"];
	        this.favorite = source["favorite"];
	    }
	}
	export class FileComparison {
	    relativePath: string;
	    localExists: boolean;
	    remoteExists: boolean;
	    localSize: number;
	    remoteSize: number;
	    localModified: string;
	    remoteModified: string;
	    localDirectory: boolean;
	    remoteDirectory: boolean;
	    status: string;

	    static createFrom(source: any = {}) {
	        return new FileComparison(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.relativePath = source["relativePath"];
	        this.localExists = source["localExists"];
	        this.remoteExists = source["remoteExists"];
	        this.localSize = source["localSize"];
	        this.remoteSize = source["remoteSize"];
	        this.localModified = source["localModified"];
	        this.remoteModified = source["remoteModified"];
	        this.localDirectory = source["localDirectory"];
	        this.remoteDirectory = source["remoteDirectory"];
	        this.status = source["status"];
	    }
	}
	export class BootstrapPayload {
	    sites: Site[];
	    tabs: Tab[];
	    config: Config;
	    defaultLocalPath: string;
	    localFiles: FileEntry[];
	    remoteFiles: FileEntry[];
	    transfers: TransferItem[];
	    logs: LogItem[];
	
	    static createFrom(source: any = {}) {
	        return new BootstrapPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sites = this.convertValues(source["sites"], Site);
	        this.tabs = this.convertValues(source["tabs"], Tab);
	        this.config = this.convertValues(source["config"], Config);
	        this.defaultLocalPath = source["defaultLocalPath"];
	        this.localFiles = this.convertValues(source["localFiles"], FileEntry);
	        this.remoteFiles = this.convertValues(source["remoteFiles"], FileEntry);
	        this.transfers = this.convertValues(source["transfers"], TransferItem);
	        this.logs = this.convertValues(source["logs"], LogItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class HostTrustPrompt {
	    host: string;
	    port: number;
	    hostPattern: string;
	    keyType: string;
	    fingerprintSHA256: string;
	    authorizedKey: string;
	    replacesExisting: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HostTrustPrompt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.hostPattern = source["hostPattern"];
	        this.keyType = source["keyType"];
	        this.fingerprintSHA256 = source["fingerprintSHA256"];
	        this.authorizedKey = source["authorizedKey"];
	        this.replacesExisting = source["replacesExisting"];
	    }
	}
	
	export class RESTServerStatus {
	    enabled: boolean;
	    running: boolean;
	    baseURL: string;
	    mcpURL: string;
	    port: number;
	    attached: boolean;
	    allowlist: string[];
	
	    static createFrom(source: any = {}) {
	        return new RESTServerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.running = source["running"];
	        this.baseURL = source["baseURL"];
	        this.mcpURL = source["mcpURL"];
	        this.port = source["port"];
	        this.attached = source["attached"];
	        this.allowlist = source["allowlist"];
	    }
	}
	export class UpdateActionResult {
	    downloaded: boolean;

	    static createFrom(source: any = {}) {
	        return new UpdateActionResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.downloaded = source["downloaded"];
	    }
	}
	export class UpdateCheckResult {
	    currentVersion: string;
	    latestVersion: string;
	    latestTag: string;
	    updateAvailable: boolean;
	    canDownload: boolean;
	    assetName: string;

	    static createFrom(source: any = {}) {
	        return new UpdateCheckResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.latestTag = source["latestTag"];
	        this.updateAvailable = source["updateAvailable"];
	        this.canDownload = source["canDownload"];
	        this.assetName = source["assetName"];
	    }
	}

	export class SiteLibraryMutationResult {
	    sites: Site[];
	    config: Config;
	
	    static createFrom(source: any = {}) {
	        return new SiteLibraryMutationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sites = this.convertValues(source["sites"], Site);
	        this.config = this.convertValues(source["config"], Config);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}
