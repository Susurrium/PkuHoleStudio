package service

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
)

type SettingsView struct {
	DatabaseType       string  `json:"database_type"`
	DatabaseFile       string  `json:"database_file,omitempty"`
	AIEnabled          bool    `json:"ai_enabled"`
	AILiveSearch       bool    `json:"ai_live_search"`
	AIProviderName     string  `json:"ai_provider_name"`
	AIBaseURL          string  `json:"ai_base_url"`
	AIModel            string  `json:"ai_model"`
	AITemperature      float64 `json:"ai_temperature"`
	AIMaxOutputTokens  int     `json:"ai_max_output_tokens"`
	AIRequestTimeout   int     `json:"ai_request_timeout_seconds"`
	AIMaxSearchRounds  int     `json:"ai_max_search_rounds"`
	AIAPIKeyConfigured bool    `json:"ai_api_key_configured"`
	RestartRequired    bool    `json:"restart_required"`
}

type SettingsUpdate struct {
	AIEnabled         bool    `json:"ai_enabled"`
	AILiveSearch      bool    `json:"ai_live_search"`
	AIProviderName    string  `json:"ai_provider_name"`
	AIBaseURL         string  `json:"ai_base_url"`
	AIModel           string  `json:"ai_model"`
	AITemperature     float64 `json:"ai_temperature"`
	AIMaxOutputTokens int     `json:"ai_max_output_tokens"`
	AIRequestTimeout  int     `json:"ai_request_timeout_seconds"`
	AIMaxSearchRounds int     `json:"ai_max_search_rounds"`
	AIAPIKey          string  `json:"ai_api_key,omitempty"`
	ClearAIAPIKey     bool    `json:"clear_ai_api_key,omitempty"`
}

type SettingsService struct {
	mu     sync.Mutex
	config *config.Config
	save   func(*config.Config) error
}

func NewSettingsService(current *config.Config) *SettingsService {
	return &SettingsService{config: current, save: config.SaveConfig}
}

func (s *SettingsService) Get(ctx context.Context) (SettingsView, error) {
	if err := contextError(ctx); err != nil {
		return SettingsView{}, err
	}
	if s == nil {
		return SettingsView{}, errors.New("settings are unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.config == nil {
		return SettingsView{}, errors.New("settings are unavailable")
	}
	return settingsView(s.config, false), nil
}

func (s *SettingsService) Update(ctx context.Context, update SettingsUpdate) (SettingsView, error) {
	if err := contextError(ctx); err != nil {
		return SettingsView{}, err
	}
	if err := validateSettingsUpdate(update); err != nil {
		return SettingsView{}, err
	}
	if s == nil {
		return SettingsView{}, errors.New("settings are unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.config == nil || s.save == nil {
		return SettingsView{}, errors.New("settings are unavailable")
	}
	next := *s.config
	next.AI = s.config.AI
	next.AI.Enabled = update.AIEnabled
	next.AI.AllowLiveSearch = update.AILiveSearch
	next.AI.MaxSearchRounds = update.AIMaxSearchRounds
	next.AI.Provider.Name = strings.TrimSpace(update.AIProviderName)
	next.AI.Provider.BaseURL = strings.TrimRight(strings.TrimSpace(update.AIBaseURL), "/")
	next.AI.Provider.Model = strings.TrimSpace(update.AIModel)
	next.AI.Provider.Temperature = update.AITemperature
	next.AI.Provider.MaxOutputTokens = update.AIMaxOutputTokens
	next.AI.Provider.RequestTimeout = update.AIRequestTimeout
	if update.ClearAIAPIKey {
		next.AI.Provider.APIKey = ""
	} else if strings.TrimSpace(update.AIAPIKey) != "" {
		next.AI.Provider.APIKey = strings.TrimSpace(update.AIAPIKey)
	}
	if err := s.save(&next); err != nil {
		return SettingsView{}, err
	}
	*s.config = next
	return settingsView(s.config, true), nil
}

func settingsView(current *config.Config, restartRequired bool) SettingsView {
	return SettingsView{
		DatabaseType: current.Database.Type, DatabaseFile: current.Database.DBFile,
		AIEnabled: current.AI.Enabled, AILiveSearch: current.AI.AllowLiveSearch,
		AIProviderName: current.AI.Provider.Name, AIBaseURL: current.AI.Provider.BaseURL,
		AIModel: current.AI.Provider.Model, AITemperature: current.AI.Provider.Temperature,
		AIMaxOutputTokens: current.AI.Provider.MaxOutputTokens, AIRequestTimeout: current.AI.Provider.RequestTimeout,
		AIMaxSearchRounds: current.AI.MaxSearchRounds, AIAPIKeyConfigured: current.AI.Provider.APIKey != "",
		RestartRequired: restartRequired,
	}
}

func validateSettingsUpdate(update SettingsUpdate) error {
	parsed, err := url.Parse(strings.TrimSpace(update.AIBaseURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("AI base URL must be an absolute HTTP or HTTPS URL")
	}
	if strings.TrimSpace(update.AIProviderName) == "" || strings.TrimSpace(update.AIModel) == "" {
		return errors.New("AI provider name and model are required")
	}
	if update.AITemperature < 0 || update.AITemperature > 2 {
		return errors.New("AI temperature must be between 0 and 2")
	}
	if update.AIMaxOutputTokens < 1 || update.AIMaxOutputTokens > 1_000_000 {
		return errors.New("AI max output tokens must be between 1 and 1000000")
	}
	if update.AIRequestTimeout < 1 || update.AIRequestTimeout > 3600 {
		return errors.New("AI request timeout must be between 1 and 3600 seconds")
	}
	if update.AIMaxSearchRounds < 1 || update.AIMaxSearchRounds > 20 {
		return errors.New("AI max search rounds must be between 1 and 20")
	}
	return nil
}
