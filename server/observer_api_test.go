package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
	"github.com/Susurrium/PkuHoleStudio/internal/service"

	"github.com/gin-gonic/gin"
)

func TestObserverSettingsAPINeverReturnsToken(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Observer.Enabled = true
	cfg.Observer.BaseURL = "https://observer.example"
	cfg.Observer.APIToken = "must-not-leak"
	router := gin.New()
	registerAPIV1(router.Group("/api/v1"), Dependencies{Settings: service.NewSettingsService(&cfg)})
	response := performRequest(router, http.MethodGet, "/api/v1/settings/observer", nil, "")
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "must-not-leak") || strings.Contains(response.Body.String(), `"api_token":`) {
		t.Fatalf("secret leaked: %s", response.Body.String())
	}
	var envelope struct {
		Data service.ObserverSettingsView `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.Data.APITokenConfigured {
		t.Fatalf("settings=%+v", envelope.Data)
	}
}

func TestExportCanExplicitlyRequireObserverOrBypassIt(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()
	required := performRequest(router, http.MethodPost, "/api/v1/exports/jobs", strings.NewReader(`{"format":"treehole-v2","pids":[8133824],"include_comments":true,"sync_observer_before_export":true}`), "application/json")
	if required.Code != http.StatusServiceUnavailable || !strings.Contains(required.Body.String(), `"code":"observer_sync_failed"`) || !strings.Contains(required.Body.String(), `"can_export_local_snapshot":true`) {
		t.Fatalf("required status=%d body=%s", required.Code, required.Body.String())
	}
	bypass := performRequest(router, http.MethodPost, "/api/v1/exports/jobs", strings.NewReader(`{"format":"treehole-v2","pids":[8133824],"include_comments":true,"sync_observer_before_export":false}`), "application/json")
	if bypass.Code != http.StatusAccepted {
		t.Fatalf("bypass status=%d body=%s", bypass.Code, bypass.Body.String())
	}
}
