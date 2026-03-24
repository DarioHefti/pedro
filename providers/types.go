package providers

import (
	"pedro/providers/azure"
	"pedro/providers/openai"
	"pedro/shared"
)

type Message = shared.Message
type LLMClient = shared.LLMClient
type Config = shared.Config
type Settings = shared.Settings
type AuthStatus = shared.AuthStatus

type AzureConfig = azure.AzureConfig
type AzureProvider = azure.Provider
type OpenAIConfig = openai.OpenAIConfig

// SetAuthRecordCallbacks wires up persistence for Azure auth records
func SetAuthRecordCallbacks(save func(string) error, load func() (string, error)) {
	azure.SaveAuthRecord = save
	azure.LoadAuthRecord = load
}
