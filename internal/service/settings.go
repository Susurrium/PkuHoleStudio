package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
)

type SettingsView struct {
	DatabaseType       string               `json:"database_type"`
	DatabaseFile       string               `json:"database_file,omitempty"`
	AIEnabled          bool                 `json:"ai_enabled"`
	AILiveSearch       bool                 `json:"ai_live_search"`
	AIProviderName     string               `json:"ai_provider_name"`
	AIBaseURL          string               `json:"ai_base_url"`
	AIModel            string               `json:"ai_model"`
	AITemperature      float64              `json:"ai_temperature"`
	AIMaxOutputTokens  int                  `json:"ai_max_output_tokens"`
	AIRequestTimeout   int                  `json:"ai_request_timeout_seconds"`
	AIMaxSearchRounds  int                  `json:"ai_max_search_rounds"`
	AIAPIKeyConfigured bool                 `json:"ai_api_key_configured"`
	RestartRequired    bool                 `json:"restart_required"`
	AIActiveProvider   string               `json:"ai_active_provider"`
	AIProviders        []AIProviderSettings `json:"ai_providers"`
}

type AIProviderSettings struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	BaseURL          string  `json:"base_url"`
	Model            string  `json:"model"`
	Temperature      float64 `json:"temperature"`
	MaxOutputTokens  int     `json:"max_output_tokens"`
	RequestTimeout   int     `json:"request_timeout_seconds"`
	APIKeyConfigured bool    `json:"api_key_configured"`
	Active           bool    `json:"active"`
}

type AIProviderSettingsUpdate struct {
	Name            string  `json:"name"`
	BaseURL         string  `json:"base_url"`
	Model           string  `json:"model"`
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"max_output_tokens"`
	RequestTimeout  int     `json:"request_timeout_seconds"`
	APIKey          string  `json:"api_key,omitempty"`
	ClearAPIKey     bool    `json:"clear_api_key,omitempty"`
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
	next.AI.Providers = append([]config.AIProviderConfig(nil), s.config.AI.Providers...)
	config.NormalizeAIProviders(&next.AI)
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
	for index := range next.AI.Providers {
		if next.AI.Providers[index].ID == next.AI.ActiveProvider {
			next.AI.Providers[index] = next.AI.Provider
			break
		}
	}
	if err := s.save(&next); err != nil {
		return SettingsView{}, err
	}
	*s.config = next
	return settingsView(s.config, true), nil
}

func (s *SettingsService) CreateAIProvider(ctx context.Context, update AIProviderSettingsUpdate) (SettingsView, error) {
	return s.mutateAIProvider(ctx, "", update, false)
}

func (s *SettingsService) UpdateAIProvider(ctx context.Context, id string, update AIProviderSettingsUpdate) (SettingsView, error) {
	return s.mutateAIProvider(ctx, strings.TrimSpace(id), update, true)
}

func (s *SettingsService) mutateAIProvider(ctx context.Context, id string, update AIProviderSettingsUpdate, existing bool) (SettingsView, error) {
	if err := contextError(ctx); err != nil {
		return SettingsView{}, err
	}
	if err := validateAIProviderUpdate(update); err != nil {
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
	next.AI.Providers = append([]config.AIProviderConfig(nil), s.config.AI.Providers...)
	config.NormalizeAIProviders(&next.AI)
	index := -1
	for candidate := range next.AI.Providers {
		if next.AI.Providers[candidate].ID == id {
			index = candidate
			break
		}
	}
	if existing && index < 0 {
		return SettingsView{}, errors.New("AI provider was not found")
	}
	if !existing {
		if len(next.AI.Providers) >= 20 {
			return SettingsView{}, errors.New("at most 20 AI providers may be configured")
		}
		id = uniqueProviderID(update.Name, next.AI.Providers)
		next.AI.Providers = append(next.AI.Providers, config.AIProviderConfig{ID: id})
		index = len(next.AI.Providers) - 1
	}
	provider := next.AI.Providers[index]
	provider.Name = strings.TrimSpace(update.Name)
	provider.BaseURL = strings.TrimRight(strings.TrimSpace(update.BaseURL), "/")
	provider.Model = strings.TrimSpace(update.Model)
	provider.Temperature = update.Temperature
	provider.MaxOutputTokens = update.MaxOutputTokens
	provider.RequestTimeout = update.RequestTimeout
	if update.ClearAPIKey {
		provider.APIKey = ""
	} else if strings.TrimSpace(update.APIKey) != "" {
		provider.APIKey = strings.TrimSpace(update.APIKey)
	}
	next.AI.Providers[index] = provider
	if next.AI.ActiveProvider == provider.ID {
		next.AI.Provider = provider
	}
	if err := s.save(&next); err != nil {
		return SettingsView{}, err
	}
	*s.config = next
	return settingsView(s.config, true), nil
}

func (s *SettingsService) ActivateAIProvider(ctx context.Context, id string) (SettingsView, error) {
	if err := contextError(ctx); err != nil {
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
	next.AI.Providers = append([]config.AIProviderConfig(nil), s.config.AI.Providers...)
	config.NormalizeAIProviders(&next.AI)
	found := false
	for _, provider := range next.AI.Providers {
		if provider.ID == id {
			next.AI.ActiveProvider = id
			next.AI.Provider = provider
			found = true
			break
		}
	}
	if !found {
		return SettingsView{}, errors.New("AI provider was not found")
	}
	if err := s.save(&next); err != nil {
		return SettingsView{}, err
	}
	*s.config = next
	return settingsView(s.config, true), nil
}

func (s *SettingsService) DeleteAIProvider(ctx context.Context, id string) (SettingsView, error) {
	if err := contextError(ctx); err != nil {
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
	next.AI.Providers = append([]config.AIProviderConfig(nil), s.config.AI.Providers...)
	config.NormalizeAIProviders(&next.AI)
	if len(next.AI.Providers) <= 1 {
		return SettingsView{}, errors.New("the last AI provider cannot be deleted")
	}
	filtered := make([]config.AIProviderConfig, 0, len(next.AI.Providers)-1)
	found := false
	for _, provider := range next.AI.Providers {
		if provider.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, provider)
	}
	if !found {
		return SettingsView{}, errors.New("AI provider was not found")
	}
	next.AI.Providers = filtered
	if next.AI.ActiveProvider == id {
		next.AI.ActiveProvider = filtered[0].ID
		next.AI.Provider = filtered[0]
	}
	if err := s.save(&next); err != nil {
		return SettingsView{}, err
	}
	*s.config = next
	return settingsView(s.config, true), nil
}

func settingsView(current *config.Config, restartRequired bool) SettingsView {
	config.NormalizeAIProviders(&current.AI)
	view := SettingsView{
		DatabaseType: current.Database.Type, DatabaseFile: current.Database.DBFile,
		AIEnabled: current.AI.Enabled, AILiveSearch: current.AI.AllowLiveSearch,
		AIProviderName: current.AI.Provider.Name, AIBaseURL: current.AI.Provider.BaseURL,
		AIModel: current.AI.Provider.Model, AITemperature: current.AI.Provider.Temperature,
		AIMaxOutputTokens: current.AI.Provider.MaxOutputTokens, AIRequestTimeout: current.AI.Provider.RequestTimeout,
		AIMaxSearchRounds: current.AI.MaxSearchRounds, AIAPIKeyConfigured: current.AI.Provider.APIKey != "",
		RestartRequired:  restartRequired,
		AIActiveProvider: current.AI.ActiveProvider,
	}
	for _, provider := range current.AI.Providers {
		view.AIProviders = append(view.AIProviders, AIProviderSettings{ID: provider.ID, Name: provider.Name, BaseURL: provider.BaseURL, Model: provider.Model, Temperature: provider.Temperature, MaxOutputTokens: provider.MaxOutputTokens, RequestTimeout: provider.RequestTimeout, APIKeyConfigured: provider.APIKey != "", Active: provider.ID == current.AI.ActiveProvider})
	}
	return view
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

func validateAIProviderUpdate(update AIProviderSettingsUpdate) error {
	return validateSettingsUpdate(SettingsUpdate{AIProviderName: update.Name, AIBaseURL: update.BaseURL, AIModel: update.Model, AITemperature: update.Temperature, AIMaxOutputTokens: update.MaxOutputTokens, AIRequestTimeout: update.RequestTimeout, AIMaxSearchRounds: 1})
}

var providerIDCharacters = regexp.MustCompile(`[^a-z0-9]+`)

func uniqueProviderID(name string, providers []config.AIProviderConfig) string {
	base := strings.Trim(providerIDCharacters.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-"), "-")
	if base == "" {
		base = "provider"
	}
	used := make(map[string]bool, len(providers))
	for _, provider := range providers {
		used[provider.ID] = true
	}
	if !used[base] {
		return base
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s-%d", base, suffix)
		if !used[candidate] {
			return candidate
		}
	}
}
