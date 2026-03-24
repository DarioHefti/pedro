# Azure SSO Authentication Implementation Plan

## Overview

Replace API key authentication with browser-based Azure SSO using Microsoft's official SDK. Users will only need to provide their Azure OpenAI endpoint URL and deployment name, then sign in via browser.

## User Requirements

| Decision | Choice |
|----------|--------|
| Auth method | SSO only (remove API key completely) |
| Token caching | Persistent (survives app restart) |
| Tenant ID | Not exposed (handled in browser) |
| Test Connection | Triggers sign-in if not authenticated |
| Auth failure mid-conversation | Show error, require manual re-sign-in |
| Sign out | Clears everything (tokens + endpoint + deployment) |

## Dependencies to Add

```
github.com/Azure/azure-sdk-for-go/sdk/azidentity
github.com/Azure/azure-sdk-for-go/sdk/azidentity/cache
github.com/openai/openai-go
```

## Files to Change

| File | Action | Description |
|------|--------|-------------|
| `go.mod` | Modify | Add Azure SDK dependencies |
| `azure_oauth.go` | **Create** | New OAuth client using official SDK |
| `azure.go` | **Delete** | Remove old API key client |
| `app.go` | Modify | Use new client, add SignIn/SignOut/IsAuthenticated |
| `interfaces.go` | Modify | Extend LLMClient interface |
| `frontend/src/SettingsModal.tsx` | Modify | Replace API key field with Sign In button |

## Implementation Details

### 1. `azure_oauth.go` - New OAuth Client

```go
type AzureOAuthClient struct {
    client             *openai.Client
    credential         *azidentity.InteractiveBrowserCredential
    endpoint           string
    deployment         string
    registry           *tools.Registry
    customSystemPrompt string
}

// NewAzureOAuthClient creates client (doesn't authenticate yet)
func NewAzureOAuthClient(endpoint, deployment string, registry *tools.Registry) (*AzureOAuthClient, error)

// SignIn triggers browser authentication, caches tokens
func (a *AzureOAuthClient) SignIn(ctx context.Context) error

// SignOut clears all cached tokens
func (a *AzureOAuthClient) SignOut() error

// IsAuthenticated checks if we have valid cached tokens
func (a *AzureOAuthClient) IsAuthenticated() bool

// Chat implements LLMClient interface (streaming + tool calling)
func (a *AzureOAuthClient) Chat(...) error

// SetCustomSystemPrompt implements LLMClient interface
func (a *AzureOAuthClient) SetCustomSystemPrompt(prompt string)
```

### 2. `app.go` - Backend Updates

**Remove:**
- `azure_api_key` handling
- API key parameters from SaveSettings/TestConnection

**Add methods:**
```go
func (a *App) SignIn() string           // Triggers browser auth
func (a *App) SignOut() error           // Clears tokens AND settings
func (a *App) IsAuthenticated() bool    // Check auth status
```

**Modify:**
- `SaveSettings(endpoint, deployment string)` - only 2 params
- `TestConnection()` - triggers sign-in if not authenticated first

### 3. `interfaces.go` - Interface Updates

```go
type LLMClient interface {
    Chat(ctx context.Context, messages []Message, imageDataURLs []string, onChunk func(string), onToolCall func(name, argsJSON string)) error
    SetCustomSystemPrompt(prompt string)
    SignIn(ctx context.Context) error
    SignOut() error
    IsAuthenticated() bool
}
```

### 4. Frontend Settings UI

**Remove:**
- API Key input field

**Keep:**
- Endpoint URL input
- Deployment Name input

**Add:**
- Auth status display ("Signed in" / "Not signed in")
- "Sign In with Azure" button (when not authenticated)
- "Sign Out" button (when authenticated)

**UI Layout:**
```
┌─────────────────────────────────────┐
│ Azure OpenAI Settings               │
├─────────────────────────────────────┤
│ Endpoint URL:                       │
│ [https://myco.openai.azure.com    ] │
│                                     │
│ Deployment Name:                    │
│ [gpt-4                            ] │
│                                     │
│ Status: ● Not signed in             │
│                                     │
│ [Sign In with Azure]  [Test]        │
│                                     │
│              [Save]   [Cancel]      │
└─────────────────────────────────────┘
```

When signed in:
```
│ Status: ● Signed in                 │
│                                     │
│ [Sign Out]            [Test]        │
```

## User Flow

### First Launch
1. User opens Settings
2. Enters endpoint URL (e.g., `https://mycompany.openai.azure.com`)
3. Enters deployment name (e.g., `gpt-4`)
4. Clicks "Sign In with Azure"
5. Browser opens → user logs in with Microsoft account
6. Tokens cached to OS keychain
7. Ready to chat!

### Subsequent Launches
1. App loads cached tokens automatically
2. No login needed (tokens refresh silently)
3. User can start chatting immediately

### Token Expiry/Revocation
1. SDK detects invalid token
2. Error shown to user
3. User must manually click "Sign In" again

### Sign Out
1. User clicks "Sign Out"
2. Cached tokens cleared
3. Endpoint and deployment settings cleared
4. User returns to initial setup state

## Technical Notes

- Uses `azure.WithEndpoint()` and `azure.WithTokenCredential()` from `openai-go`
- Persistent cache via `azidentity/cache.New()` (uses OS keychain)
- Streaming via `client.Chat.Completions.NewStreaming()`
- Tool calling simplified to work with new SDK
- If OS keychain unavailable, falls back to memory-only cache
