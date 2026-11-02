export namespace apperr {
	
	export class PublicError {
	    code: string;
	    message: string;
	    retryAfterSeconds?: number;
	
	    static createFrom(source: any = {}) {
	        return new PublicError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	        this.retryAfterSeconds = source["retryAfterSeconds"];
	    }
	}

}

export namespace appinfo {
	
	export class Info {
	    name: string;
	    version: string;
	    goVersion: string;
	    platform: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.goVersion = source["goVersion"];
	        this.platform = source["platform"];
	    }
	}

}

export namespace images {
	
	export class Author {
	    name: string;
	    url: string;
	    credit: string;
	
	    static createFrom(source: any = {}) {
	        return new Author(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.credit = source["credit"];
	    }
	}
	export class License {
	    status: string;
	    name: string;
	    url: string;
	    notice: string;
	
	    static createFrom(source: any = {}) {
	        return new License(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.notice = source["notice"];
	    }
	}
	export class Image {
	    provider: string;
	    providerId: string;
	    pageUrl: string;
	    thumbnailUrl: string;
	    previewUrl: string;
	    downloadUrl: string;
	    width: number;
	    height: number;
	    mimeType: string;
	    fileSize: number;
	    author: Author;
	    sourceUrl: string;
	    license: License;
	
	    static createFrom(source: any = {}) {
	        return new Image(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.providerId = source["providerId"];
	        this.pageUrl = source["pageUrl"];
	        this.thumbnailUrl = source["thumbnailUrl"];
	        this.previewUrl = source["previewUrl"];
	        this.downloadUrl = source["downloadUrl"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.mimeType = source["mimeType"];
	        this.fileSize = source["fileSize"];
	        this.author = this.convertValues(source["author"], Author);
	        this.sourceUrl = source["sourceUrl"];
	        this.license = this.convertValues(source["license"], License);
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

export namespace main {
	
	export class CollectionInfo {
	    id: number;
	    name: string;
	    images: number;
	
	    static createFrom(source: any = {}) {
	        return new CollectionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.images = source["images"];
	    }
	}
	export class DatabaseStatus {
	    path: string;
	    schemaVersion: number;
	
	    static createFrom(source: any = {}) {
	        return new DatabaseStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.schemaVersion = source["schemaVersion"];
	    }
	}
	export class FetchedStats {
	    images: number;
	    bytes: number;
	    maxBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new FetchedStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.images = source["images"];
	        this.bytes = source["bytes"];
	        this.maxBytes = source["maxBytes"];
	    }
	}
	export class LibraryItem {
	    id: number;
	    image: images.Image;
	    fileUrl: string;
	    addedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new LibraryItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.image = this.convertValues(source["image"], images.Image);
	        this.fileUrl = source["fileUrl"];
	        this.addedAt = source["addedAt"];
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
	export class LibraryPage {
	    items: LibraryItem[];
	    hasMore: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LibraryPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], LibraryItem);
	        this.hasMore = source["hasMore"];
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
	export class LibraryStats {
	    images: number;
	    bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new LibraryStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.images = source["images"];
	        this.bytes = source["bytes"];
	    }
	}
	export class RotationSettings {
	    enabled: boolean;
	    interval: string;
	    collectionId: number;
	    savedSearchId: number;
	    order: string;
	
	    static createFrom(source: any = {}) {
	        return new RotationSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.interval = source["interval"];
	        this.collectionId = source["collectionId"];
	        this.savedSearchId = source["savedSearchId"];
	        this.order = source["order"];
	    }
	}
	export class TimerStatus {
	    installed: boolean;
	    active: boolean;
	    nextRun: string;
	    outdated: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new TimerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installed = source["installed"];
	        this.active = source["active"];
	        this.nextRun = source["nextRun"];
	        this.outdated = source["outdated"];
	        this.error = source["error"];
	    }
	}
	export class RotationState {
	    settings: RotationSettings;
	    intervals: string[];
	    timer: TimerStatus;
	    lastImageId: number;
	    lastChange: string;
	    varietyRunning: boolean;
	    sourceImages: number;
	
	    static createFrom(source: any = {}) {
	        return new RotationState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.settings = this.convertValues(source["settings"], RotationSettings);
	        this.intervals = source["intervals"];
	        this.timer = this.convertValues(source["timer"], TimerStatus);
	        this.lastImageId = source["lastImageId"];
	        this.lastChange = source["lastChange"];
	        this.varietyRunning = source["varietyRunning"];
	        this.sourceImages = source["sourceImages"];
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
	export class SavedSearch {
	    id: number;
	    name: string;
	    query: searches.Query;
	
	    static createFrom(source: any = {}) {
	        return new SavedSearch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.query = this.convertValues(source["query"], searches.Query);
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
	export class SourceStatus {
	    providerId: string;
	    name: string;
	    page: number;
	    hasMore: boolean;
	    count: number;
	    error?: apperr.PublicError;
	
	    static createFrom(source: any = {}) {
	        return new SourceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.providerId = source["providerId"];
	        this.name = source["name"];
	        this.page = source["page"];
	        this.hasMore = source["hasMore"];
	        this.count = source["count"];
	        this.error = this.convertValues(source["error"], apperr.PublicError);
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
	export class SearchResult {
	    images: images.Image[];
	    sources: SourceStatus[];
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.images = this.convertValues(source["images"], images.Image);
	        this.sources = this.convertValues(source["sources"], SourceStatus);
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
	
	export class Status {
	    app: appinfo.Info;
	    database: DatabaseStatus;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.app = this.convertValues(source["app"], appinfo.Info);
	        this.database = this.convertValues(source["database"], DatabaseStatus);
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
	
	export class WallpaperResult {
	    method: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new WallpaperResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.method = source["method"];
	        this.message = source["message"];
	    }
	}

}

export namespace providers {
	
	export class Query {
	    text: string;
	    exclude: string[];
	    page: number;
	    minWidth: number;
	    minHeight: number;
	    orientation: string;
	
	    static createFrom(source: any = {}) {
	        return new Query(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.exclude = source["exclude"];
	        this.page = source["page"];
	        this.minWidth = source["minWidth"];
	        this.minHeight = source["minHeight"];
	        this.orientation = source["orientation"];
	    }
	}

}

export namespace search {
	
	export class ProviderInfo {
	    id: string;
	    name: string;
	    contentWarning?: string;
	    enabledByDefault: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProviderInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.contentWarning = source["contentWarning"];
	        this.enabledByDefault = source["enabledByDefault"];
	    }
	}
	export class SourcePage {
	    providerId: string;
	    page: number;
	
	    static createFrom(source: any = {}) {
	        return new SourcePage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.providerId = source["providerId"];
	        this.page = source["page"];
	    }
	}

}

export namespace searches {
	
	export class Query {
	    text: string;
	    providerIds: string[];
	    minWidth: number;
	    minHeight: number;
	    orientation: string;
	
	    static createFrom(source: any = {}) {
	        return new Query(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.providerIds = source["providerIds"];
	        this.minWidth = source["minWidth"];
	        this.minHeight = source["minHeight"];
	        this.orientation = source["orientation"];
	    }
	}

}

export namespace settings {
	
	export class SearchFilters {
	    providerIds: string[];
	    minWidth: number;
	    minHeight: number;
	    orientation: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchFilters(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.providerIds = source["providerIds"];
	        this.minWidth = source["minWidth"];
	        this.minHeight = source["minHeight"];
	        this.orientation = source["orientation"];
	    }
	}

}

