package service

import (
	"context"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
)

func TestSettingsUpdatePreservesWriteOnlySecrets(t *testing.T) {
	current := config.DefaultConfig()
	current.AI.Provider.APIKey = "existing-secret"
	current.Password = "account-secret"
	service := NewSettingsService(&current)
	var saved *config.Config
	service.save = func(value *config.Config) error {
		copy := *value
		saved = &copy
		return nil
	}

	view, err := service.Update(context.Background(), SettingsUpdate{
		AIEnabled: true, AIProviderName: "Local", AIBaseURL: "http://127.0.0.1:11434/v1/",
		AIModel: "model", AITemperature: 0.5, AIMaxOutputTokens: 2048,
		AIRequestTimeout: 30, AIMaxSearchRounds: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil || saved.AI.Provider.APIKey != "existing-secret" || saved.Password != "account-secret" {
		t.Fatalf("write-only secrets were not preserved: %+v", saved)
	}
	if view.AIBaseURL != "http://127.0.0.1:11434/v1" || !view.AIAPIKeyConfigured || !view.RestartRequired {
		t.Fatalf("view = %+v", view)
	}
	if view.DatabaseType != "sqlite3" {
		t.Fatalf("database type = %q", view.DatabaseType)
	}
}

func TestSettingsUpdateValidatesProviderLimits(t *testing.T) {
	current := config.DefaultConfig()
	service := NewSettingsService(&current)
	_, err := service.Update(context.Background(), SettingsUpdate{
		AIProviderName: "DeepSeek", AIBaseURL: "javascript:alert(1)", AIModel: "deepseek-chat",
		AITemperature: 0.2, AIMaxOutputTokens: 4096, AIRequestTimeout: 120, AIMaxSearchRounds: 5,
	})
	if err == nil {
		t.Fatal("unsafe base URL unexpectedly succeeded")
	}
}

func TestSettingsManageMultipleAIProvidersAndPreserveKeys(t *testing.T) {
	current := config.DefaultConfig()
	current.AI.Provider.APIKey = "deepseek-secret"
	service := NewSettingsService(&current)
	service.save = func(*config.Config) error { return nil }
	view, err := service.CreateAIProvider(context.Background(), AIProviderSettingsUpdate{
		Name: "Local Ollama", BaseURL: "http://127.0.0.1:11434/v1", Model: "qwen3",
		Temperature: 0.3, MaxOutputTokens: 8192, RequestTimeout: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.AIProviders) != 2 || view.AIProviders[1].ID != "local-ollama" || view.AIActiveProvider != "deepseek" {
		t.Fatalf("providers after create = %+v", view)
	}
	view, err = service.ActivateAIProvider(context.Background(), "local-ollama")
	if err != nil || view.AIActiveProvider != "local-ollama" || view.AIProviderName != "Local Ollama" {
		t.Fatalf("activate provider = %+v, %v", view, err)
	}
	_, err = service.UpdateAIProvider(context.Background(), "deepseek", AIProviderSettingsUpdate{
		Name: "DeepSeek Updated", BaseURL: "https://api.deepseek.com", Model: "deepseek-reasoner",
		Temperature: 0.1, MaxOutputTokens: 4096, RequestTimeout: 120,
	})
	if err != nil {
		t.Fatal(err)
	}
	if current.AI.Providers[0].APIKey != "deepseek-secret" {
		t.Fatal("updating a provider without an API key erased its existing key")
	}
	view, err = service.DeleteAIProvider(context.Background(), "deepseek")
	if err != nil || len(view.AIProviders) != 1 || view.AIProviders[0].ID != "local-ollama" {
		t.Fatalf("delete provider = %+v, %v", view, err)
	}
	if _, err := service.DeleteAIProvider(context.Background(), "local-ollama"); err == nil {
		t.Fatal("deleting the final provider unexpectedly succeeded")
	}
}
