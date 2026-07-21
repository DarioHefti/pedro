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
	export class LLMDetailsEntry {
	    ID: number;
	    ConversationID: number;
	    ConversationTitle: string;
	    // Go type: time
	    CreatedAt: any;
	    RequestCount: number;
	    Model: string;
	    Messages: string;
	
	    static createFrom(source: any = {}) {
	        return new LLMDetailsEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.ConversationID = source["ConversationID"];
	        this.ConversationTitle = source["ConversationTitle"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.RequestCount = source["RequestCount"];
	        this.Model = source["Model"];
	        this.Messages = source["Messages"];
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
	export class LifetimeStats {
	    totalRequests: number;
	    totalTokens: number;
	
	    static createFrom(source: any = {}) {
	        return new LifetimeStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalRequests = source["totalRequests"];
	        this.totalTokens = source["totalTokens"];
	    }
	}
	export class Message {
	    ID: number;
	    ConversationID: number;
	    Role: string;
	    Content: string;
	    Attachments: string;
	    ToolCalls: string;
	    ToolCallID: string;
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
	        this.ToolCallID = source["ToolCallID"];
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
	export class RequestCounts {
	    perChat: number;
	    perChatTokens: number;
	    global: number;
	
	    static createFrom(source: any = {}) {
	        return new RequestCounts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.perChat = source["perChat"];
	        this.perChatTokens = source["perChatTokens"];
	        this.global = source["global"];
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
	    Importance: number;
	    Source: string;
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
	        this.Importance = source["Importance"];
	        this.Source = source["Source"];
	        this.UpdatedAt = source["UpdatedAt"];
	    }
	}

}

