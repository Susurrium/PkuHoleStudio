package server

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/Susurrium/PkuHoleStudio/internal/archive"

	"github.com/gin-gonic/gin"
)

const toolkitBridgeProtocolHeader = "X-PkuHole-Toolkit"

func apiCreateBridgeDeviceRequest(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		Name          string `json:"name"`
		PublicKeySPKI string `json:"public_key_spki"`
	}
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireToolkitBridge(c) {
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		created, err := dependencies.Bridge.CreateDeviceRequest(body.Name, body.PublicKeySPKI)
		if err != nil {
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "too many") {
				status = http.StatusTooManyRequests
			}
			apiFailure(c, status, "bridge_device_request_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusCreated, created)
	}
}

func apiBridgeDeviceRequest(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireToolkitBridge(c) {
			return
		}
		request, ok := dependencies.Bridge.DeviceRequest(c.Param("token"))
		if !ok {
			apiFailure(c, http.StatusNotFound, "bridge_device_request_not_found", "device request expired or was not found", nil)
			return
		}
		apiRespond(c, http.StatusOK, request)
	}
}

func apiBridgeDeviceRequests(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireStudioBridgeBrowser(c) {
			return
		}
		apiRespond(c, http.StatusOK, dependencies.Bridge.PendingDeviceRequests())
	}
}

func apiApproveBridgeDeviceRequest(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireStudioBridgeBrowser(c) {
			return
		}
		request, err := dependencies.Bridge.ApproveDeviceRequest(c.Param("token"))
		if errors.Is(err, os.ErrNotExist) {
			apiFailure(c, http.StatusNotFound, "bridge_device_request_not_found", "device request expired or was not found", nil)
			return
		}
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "bridge_device_approval_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, request)
	}
}

func apiRejectBridgeDeviceRequest(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireStudioBridgeBrowser(c) {
			return
		}
		if !dependencies.Bridge.RejectDeviceRequest(c.Param("token")) {
			apiFailure(c, http.StatusNotFound, "bridge_device_request_not_found", "device request expired or was not found", nil)
			return
		}
		apiRespond(c, http.StatusOK, gin.H{"status": "rejected"})
	}
}

func apiBridgeDevices(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireStudioBridgeBrowser(c) {
			return
		}
		apiRespond(c, http.StatusOK, dependencies.Bridge.Devices())
	}
}

func apiRevokeBridgeDevice(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireStudioBridgeBrowser(c) {
			return
		}
		if err := dependencies.Bridge.RevokeDevice(c.Param("id")); errors.Is(err, os.ErrNotExist) {
			apiFailure(c, http.StatusNotFound, "bridge_device_not_found", "trusted device was not found", nil)
			return
		} else if err != nil {
			apiFailure(c, http.StatusInternalServerError, "bridge_device_revoke_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, gin.H{"status": "revoked"})
	}
}

func apiRevokeOwnBridgeDevice(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireToolkitBridge(c) {
			return
		}
		var body BridgeDeviceRevokeRequest
		if !decodeAPIJSON(c, &body) {
			return
		}
		if body.DeviceID != c.Param("id") {
			apiFailure(c, http.StatusBadRequest, "bridge_device_mismatch", "device id does not match the request path", nil)
			return
		}
		if err := dependencies.Bridge.RevokeTrustedDevice(body); errors.Is(err, os.ErrNotExist) {
			apiFailure(c, http.StatusNotFound, "bridge_device_not_found", "trusted device was not found", nil)
			return
		} else if err != nil {
			apiFailure(c, http.StatusForbidden, "bridge_device_revoke_rejected", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, gin.H{"status": "revoked"})
	}
}

func apiCreateBridgeChallenge(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		DeviceID string `json:"device_id"`
	}
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireToolkitBridge(c) {
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		challenge, err := dependencies.Bridge.CreateChallenge(body.DeviceID)
		if errors.Is(err, os.ErrNotExist) {
			apiFailure(c, http.StatusNotFound, "bridge_device_not_found", "trusted device was not found or was revoked", nil)
			return
		}
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "bridge_challenge_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusCreated, challenge)
	}
}

func apiCreateBridgeTransfer(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireToolkitBridge(c) {
			return
		}
		var body BridgeTransferRequest
		if !decodeAPIJSON(c, &body) {
			return
		}
		transfer, err := dependencies.Bridge.CreateTrustedTransfer(body)
		if errors.Is(err, os.ErrNotExist) {
			apiFailure(c, http.StatusNotFound, "bridge_device_not_found", "trusted device was not found or was revoked", nil)
			return
		}
		if err != nil {
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "signature") || strings.Contains(err.Error(), "challenge") {
				status = http.StatusForbidden
			}
			if strings.Contains(err.Error(), "too many") {
				status = http.StatusTooManyRequests
			}
			apiFailure(c, status, "bridge_transfer_rejected", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusCreated, transfer)
	}
}

func apiUploadTrustedBridgeArchive(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireToolkitBridge(c) {
			return
		}
		ticket := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
		if ticket == "" {
			apiFailure(c, http.StatusUnauthorized, "bridge_ticket_required", "a transfer upload ticket is required", nil)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, archive.MaxArchiveBytes+(1<<20))
		file, _, err := c.Request.FormFile("file")
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "multipart field file is required", nil)
			return
		}
		defer file.Close()
		transfer, err := dependencies.Bridge.UploadTrusted(c.Request.Context(), c.Param("id"), ticket, file)
		if errors.Is(err, os.ErrNotExist) {
			apiFailure(c, http.StatusNotFound, "bridge_transfer_not_found", "transfer expired, was revoked, or has an invalid ticket", nil)
			return
		}
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_archive", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusAccepted, transfer)
	}
}

func apiBridgeTransfers(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireStudioBridgeBrowser(c) {
			return
		}
		apiRespond(c, http.StatusOK, dependencies.Bridge.TrustedTransfers())
	}
}

func apiConfirmTrustedBridgeTransfer(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireStudioBridgeBrowser(c) {
			return
		}
		transfer, err := dependencies.Bridge.ConfirmTrusted(c.Request.Context(), c.Param("id"))
		if errors.Is(err, os.ErrNotExist) {
			apiFailure(c, http.StatusNotFound, "bridge_transfer_not_found", "transfer expired or was not found", nil)
			return
		}
		if err != nil {
			apiFailure(c, http.StatusConflict, "bridge_transfer_not_ready", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusAccepted, transfer)
	}
}

func apiCancelTrustedBridgeTransfer(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dependencies.Bridge == nil || !requireStudioBridgeBrowser(c) {
			return
		}
		if !dependencies.Bridge.CancelTrustedTransfer(c.Param("id")) {
			apiFailure(c, http.StatusConflict, "bridge_transfer_not_cancellable", "transfer was not found or is currently active", nil)
			return
		}
		apiRespond(c, http.StatusOK, gin.H{"status": "cancelled"})
	}
}

func requireStudioBridgeBrowser(c *gin.Context) bool {
	return requireBridgeHost(c) && requireStudioBrowser(c)
}

func requireToolkitBridge(c *gin.Context) bool {
	if !requireBridgeHost(c) || !requireLoopback(c) {
		return false
	}
	if strings.TrimSpace(c.GetHeader(toolkitBridgeProtocolHeader)) != "2" {
		apiFailure(c, http.StatusForbidden, "toolkit_bridge_required", "this endpoint requires PkuHoleToolkit bridge protocol 2", nil)
		return false
	}
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin == "" || origin == "null" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		apiFailure(c, http.StatusForbidden, "toolkit_origin_required", "the bridge request origin is invalid", nil)
		return false
	}
	// Privileged userscript managers may use an extension origin or omit it.
	// Ordinary web origins are accepted only for the official Treehole host.
	if (parsed.Scheme == "http" || parsed.Scheme == "https") && !strings.EqualFold(parsed.Hostname(), "treehole.pku.edu.cn") {
		apiFailure(c, http.StatusForbidden, "toolkit_origin_required", "the bridge request must come from PkuHoleToolkit", nil)
		return false
	}
	return true
}

func requireBridgeHost(c *gin.Context) bool {
	host := strings.TrimSpace(c.Request.Host)
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	} else if strings.Contains(host, ":") && !strings.EqualFold(host, "localhost") {
		apiFailure(c, http.StatusForbidden, "local_host_required", "the bridge host must be loopback", nil)
		return false
	}
	host = strings.Trim(host, "[]")
	ip := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		apiFailure(c, http.StatusForbidden, "local_host_required", "the bridge host must be loopback", nil)
		return false
	}
	return true
}
