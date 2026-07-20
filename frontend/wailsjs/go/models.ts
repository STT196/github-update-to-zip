export namespace gitdiff {
	
	export class CommitInfo {
	    sha: string;
	    message: string;
	    author: string;
	    date: string;
	
	    static createFrom(source: any = {}) {
	        return new CommitInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sha = source["sha"];
	        this.message = source["message"];
	        this.author = source["author"];
	        this.date = source["date"];
	    }
	}

}

export namespace main {
	
	export class SetRepoResult {
	    branch: string;
	    ok: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new SetRepoResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.ok = source["ok"];
	        this.error = source["error"];
	    }
	}
	export class ZipResult {
	    ok: boolean;
	    zip_path: string;
	    zip_name: string;
	    files: string[];
	    count: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ZipResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.zip_path = source["zip_path"];
	        this.zip_name = source["zip_name"];
	        this.files = source["files"];
	        this.count = source["count"];
	        this.error = source["error"];
	    }
	}

}

