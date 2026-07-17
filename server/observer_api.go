package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Susurrium/PkuHoleStudio/internal/service"

	"github.com/gin-gonic/gin"
)

func requireObserver(c *gin.Context, dependencies Dependencies) bool {
	if dependencies.Observer != nil {
		return true
	}
	apiFailure(c, http.StatusServiceUnavailable, "capability_unavailable", "Observer service is unavailable", nil)
	return false
}

func apiObserverSettings(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireSettings(c, dependencies) {
			return
		}
		view, err := dependencies.Settings.GetObserver(c.Request.Context())
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "settings_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, view)
	}
}

func apiUpdateObserverSettings(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) || !requireSettings(c, dependencies) {
			return
		}
		var update service.ObserverSettingsUpdate
		if !decodeAPIJSON(c, &update) {
			return
		}
		view, err := dependencies.Settings.UpdateObserver(c.Request.Context(), update)
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "invalid_settings", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, view)
	}
}

func apiTestObserverSettings(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) || !requireObserver(c, dependencies) {
			return
		}
		result, err := dependencies.Observer.Test(c.Request.Context())
		if err != nil {
			apiFailure(c, http.StatusBadGateway, "observer_unavailable", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, result)
	}
}

func apiObserverStatus(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireObserver(c, dependencies) {
			return
		}
		result, _ := dependencies.Observer.Status(c.Request.Context())
		apiRespond(c, http.StatusOK, result)
	}
}

func apiObserverSync(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) || !requireObserver(c, dependencies) {
			return
		}
		result, err := dependencies.Observer.Sync(c.Request.Context())
		if err != nil {
			apiFailure(c, http.StatusBadGateway, "observer_sync_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, result)
	}
}

func apiObserverRemoved(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireObserver(c, dependencies) {
			return
		}
		cursor, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("cursor", "0")))
		if err != nil || cursor < 0 {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "cursor must be a non-negative integer", gin.H{"field": "cursor"})
			return
		}
		limit, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "25")))
		if err != nil || limit < 1 || limit > 100 {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "limit must be between 1 and 100", gin.H{"field": "limit"})
			return
		}
		page, err := dependencies.Observer.Removed(c.Request.Context(), c.Query("state"), c.Query("query"), cursor, limit)
		if err != nil {
			apiFailure(c, http.StatusBadRequest, "removed_query_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, page)
	}
}

func apiObserverRemovedDetail(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireObserver(c, dependencies) {
			return
		}
		pid, err := strconv.ParseInt(c.Param("pid"), 10, 32)
		if err != nil || pid <= 0 {
			apiFailure(c, http.StatusBadRequest, "invalid_input", "pid must be a positive integer", gin.H{"field": "pid"})
			return
		}
		detail, found, err := dependencies.Observer.RemovedDetail(c.Request.Context(), int32(pid))
		if err != nil {
			apiFailure(c, http.StatusInternalServerError, "removed_detail_failed", err.Error(), nil)
			return
		}
		if !found {
			apiFailure(c, http.StatusNotFound, "not_found", "removed post was not found", nil)
			return
		}
		apiRespond(c, http.StatusOK, detail)
	}
}

func apiObserverSubmitChallenge(dependencies Dependencies) gin.HandlerFunc {
	type request struct {
		Code string `json:"code"`
	}
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) || !requireObserver(c, dependencies) {
			return
		}
		var body request
		if !decodeAPIJSON(c, &body) {
			return
		}
		status, err := dependencies.Observer.SubmitChallenge(c.Request.Context(), body.Code)
		if err != nil {
			apiFailure(c, http.StatusBadGateway, "observer_auth_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, status)
	}
}

func apiObserverResendChallenge(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) || !requireObserver(c, dependencies) {
			return
		}
		status, err := dependencies.Observer.ResendChallenge(c.Request.Context())
		if err != nil {
			apiFailure(c, http.StatusBadGateway, "observer_auth_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, status)
	}
}

func apiObserverRetryAuth(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) || !requireObserver(c, dependencies) {
			return
		}
		status, err := dependencies.Observer.RetryAuth(c.Request.Context())
		if err != nil {
			apiFailure(c, http.StatusBadGateway, "observer_auth_failed", err.Error(), nil)
			return
		}
		apiRespond(c, http.StatusOK, status)
	}
}
