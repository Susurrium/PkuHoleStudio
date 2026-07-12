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
