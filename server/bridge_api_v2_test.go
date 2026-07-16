package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIV1TrustedBridgeApprovesADeviceWithoutSharingPrivateKeys(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()
	_, publicSPKI := testBridgeDeviceKey(t)
	encoded, _ := json.Marshal(map[string]string{"name": "Test Toolkit", "public_key_spki": publicSPKI})
	create := httptest.NewRequest(http.MethodPost, "/api/v1/bridge/device-requests", bytes.NewReader(encoded))
	create.Host = "127.0.0.1:8080"
	create.RemoteAddr = "127.0.0.1:54321"
	create.Header.Set("Content-Type", "application/json")
	create.Header.Set(toolkitBridgeProtocolHeader, "2")
	create.Header.Set("Origin", "https://treehole.pku.edu.cn")
	created := httptest.NewRecorder()
	router.ServeHTTP(created, create)
	var response struct {
		Data BridgeDeviceRequest `json:"data"`
	}
	if created.Code != http.StatusCreated || json.Unmarshal(created.Body.Bytes(), &response) != nil || response.Data.Token == "" || response.Data.VerificationCode == "" {
		t.Fatalf("create device request = %d %s", created.Code, created.Body.String())
	}
	if bytes.Contains(created.Body.Bytes(), []byte(publicSPKI)) {
		t.Fatal("device request response leaked the stored public key")
	}

	approve := httptest.NewRequest(http.MethodPost, "/api/v1/bridge/device-requests/"+response.Data.Token+"/approve", nil)
	approve.Host = "127.0.0.1:8080"
	approve.RemoteAddr = "127.0.0.1:54321"
	approve.Header.Set("Origin", "http://127.0.0.1:8080")
	approved := httptest.NewRecorder()
	router.ServeHTTP(approved, approve)
	if approved.Code != http.StatusOK || !bytes.Contains(approved.Body.Bytes(), []byte(`"status":"approved"`)) || !bytes.Contains(approved.Body.Bytes(), []byte(`"device_id"`)) {
		t.Fatalf("approve device request = %d %s", approved.Code, approved.Body.String())
	}

	poll := httptest.NewRequest(http.MethodGet, "/api/v1/bridge/device-requests/"+response.Data.Token, nil)
	poll.Host = "127.0.0.1:8080"
	poll.RemoteAddr = "127.0.0.1:54321"
	poll.Header.Set(toolkitBridgeProtocolHeader, "2")
	polled := httptest.NewRecorder()
	router.ServeHTTP(polled, poll)
	if polled.Code != http.StatusOK || !bytes.Contains(polled.Body.Bytes(), []byte(`"instance_id"`)) {
		t.Fatalf("poll approved device request = %d %s", polled.Code, polled.Body.String())
	}
}

func TestAPIV1TrustedBridgeRequiresProtocolHeaderAndLoopbackHost(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()
	for _, test := range []struct {
		name       string
		host       string
		remoteAddr string
		header     string
		origin     string
	}{
		{name: "missing protocol header", host: "127.0.0.1:8080"},
		{name: "foreign host", host: "example.com:8080", header: "2"},
		{name: "foreign remote address", host: "127.0.0.1:8080", remoteAddr: "192.0.2.10:54321", header: "2"},
		{name: "foreign browser origin", host: "127.0.0.1:8080", header: "2", origin: "https://example.com"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/bridge/device-requests", bytes.NewReader([]byte(`{}`)))
			request.Host = test.host
			request.RemoteAddr = test.remoteAddr
			if request.RemoteAddr == "" {
				request.RemoteAddr = "127.0.0.1:54321"
			}
			request.Header.Set("Content-Type", "application/json")
			if test.header != "" {
				request.Header.Set(toolkitBridgeProtocolHeader, test.header)
			}
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestLoopbackBrowserOriginPolicy(t *testing.T) {
	for origin, want := range map[string]bool{
		"http://127.0.0.1:8080": true,
		"http://localhost:5173": true,
		"http://[::1]:8080":     true,
		"https://example.com":   false,
		"null":                  false,
	} {
		if got := isLoopbackBrowserOrigin(origin); got != want {
			t.Errorf("isLoopbackBrowserOrigin(%q) = %v, want %v", origin, got, want)
		}
	}
}
