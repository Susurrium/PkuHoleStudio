package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/archive"
)

func TestCapabilitiesExposeNativeArchiveContract(t *testing.T) {
	_, router, cleanup := setupTestEnv(t)
	defer cleanup()
	response := performRequest(router, http.MethodGet, "/api/v1/capabilities", nil, "")
	if response.Code != http.StatusOK {
		t.Fatalf("capabilities = %d %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data struct {
			ArchiveContract archive.ArchiveContractCapabilities `json:"archive_contract"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	contract := envelope.Data.ArchiveContract
	if contract.WriteSpecVersion != archive.ArchiveSpecVersion || contract.Extensions[archive.ArchiveExtensionMedia] != 1 || contract.MaxArchiveBytes != archive.MaxArchiveBytes {
		t.Fatalf("archive contract = %+v", contract)
	}
}
