export namespace main {
	
	export class Conversation {
	    ID: number;
	    Title: string;
	    CreatedAt: string;
	    UpdatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Conversation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Title = source["Title"];
	        this.CreatedAt = source["CreatedAt"];
	        this.UpdatedAt = source["UpdatedAt"];
	    }
	}
	export class Message {
	    ID: number;
	    ConversationID: number;
	    Role: string;
	    Content: string;
	    Attachments: string;
	    ToolCalls: string;
	    CreatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.ConversationID = source["ConversationID"];
	        this.Role = source["Role"];
	        this.Content = source["Content"];
	        this.Attachments = source["Attachments"];
	        this.ToolCalls = source["ToolCalls"];
	        this.CreatedAt = source["CreatedAt"];
	    }
	}
	export class Persona {
	    ID: number;
	    Name: string;
	    Prompt: string;
	    CreatedAt: string;
	    UpdatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Persona(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.Prompt = source["Prompt"];
	        this.CreatedAt = source["CreatedAt"];
	        this.UpdatedAt = source["UpdatedAt"];
	    }
	}
	export class UpdateInfo {
	    available: boolean;
	    currentVersion: string;
	    latestVersion: string;
	    releaseURL: string;
	    assetName: string;
	    assetURL: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.releaseURL = source["releaseURL"];
	        this.assetName = source["assetName"];
	        this.assetURL = source["assetURL"];
	    }
	}

}

export namespace shared {
	
	export class MemoryRecord {
	    ID: number;
	    Key: string;
	    Value: string;
	    Category: string;
	    UpdatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new MemoryRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Key = source["Key"];
	        this.Value = source["Value"];
	        this.Category = source["Category"];
	        this.UpdatedAt = source["UpdatedAt"];
	    }
	}

}

