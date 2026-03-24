package providers

import (
	"pedro/providers/azure"
	"pedro/providers/openai"
)

func RegisterProviders(factory *Factory) {
	factory.Register(ProviderAzure, azure.Builder{}.Build, azure.ParseConfig)
	factory.Register(ProviderOpenAI, openai.Builder{}.Build, openai.ParseConfig)
}
