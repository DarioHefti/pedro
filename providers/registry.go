package providers

import (
	"pedro/providers/azure"
	"pedro/providers/azure_apikey"
	"pedro/providers/openai"
)

func RegisterProviders(factory *Factory) {
	factory.Register(ProviderAzure, ProviderDescriptor{
		ID:   ProviderAzure,
		Name: "Azure OpenAI (Login)",
	}, azure.Build, azure.ParseConfig)
	factory.Register(ProviderAzureAPIKey, ProviderDescriptor{
		ID:   ProviderAzureAPIKey,
		Name: "Azure OpenAI (API Key)",
	}, azure_apikey.Build, azure_apikey.ParseConfig)
	factory.Register(ProviderOpenAI, ProviderDescriptor{
		ID:   ProviderOpenAI,
		Name: "OpenAI",
	}, openai.Build, openai.ParseConfig)
}
