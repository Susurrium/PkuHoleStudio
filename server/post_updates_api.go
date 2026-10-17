package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"

	"github.com/Susurrium/PkuHoleStudio/internal/jobs"
	"github.com/Susurrium/PkuHoleStudio/internal/service"
	"github.com/gin-gonic/gin"
)

var pidUpdateJobMu sync.Mutex

type pidUpdateRequest struct {
	Account       string  `json:"account"`
	PIDs          []int32 `json:"pids"`
	Comments      bool    `json:"include_comments"`
	Media         bool    `json:"include_media"`
	PreserveMedia bool    `json:"preserve_media_options"`
}

func createOrReusePIDJob(ctx context.Context, deps Dependencies, payload any, total int) (jobs.Job, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return jobs.Job{}, err
	}
	var request pidUpdateRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return jobs.Job{}, err
	}
	if len(request.PIDs) == 0 || len(request.PIDs) > 2000 {
		return jobs.Job{}, errors.New("请选择 1 至 2000 个 PID")
	}
	seen := map[int32]bool{}
	for _, pid := range request.PIDs {
		if pid <= 0 || seen[pid] {
			return jobs.Job{}, errors.New("PID 必须是互不重复的正整数")
		}
		seen[pid] = true
	}
	pidUpdateJobMu.Lock()
	defer pidUpdateJobMu.Unlock()
	if len(request.PIDs) == 1 && !request.PreserveMedia {
		rows, err := deps.Jobs.List(ctx, 0)
		if err != nil {
			return jobs.Job{}, err
		}
		for _, job := range rows {
			if job.Type != jobs.TypeSyncPIDs || (job.Status != jobs.StatusRunning && job.Status != jobs.StatusQueued) {
				continue
			}
			var current pidUpdateRequest
			if err := json.Unmarshal(job.Payload, &current); err != nil {
				return jobs.Job{}, err
			}
			if current.Account != request.Account || current.PreserveMedia || (request.Comments && !current.Comments) || (request.Media && !current.Media) {
				continue
			}
			for _, pid := range current.PIDs {
				if pid == request.PIDs[0] {
					return job, nil
				}
			}
		}
	}
	return deps.Jobs.Create(ctx, jobs.CreateRequest{Type: jobs.TypeSyncPIDs, Payload: payload, TotalItems: total})
}

func apiPostUpdateSettings(deps Dependencies, write bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.PostUpdates == nil {
			apiFailure(c, 503, "capability_unavailable", "更新服务不可用", nil)
			return
		}
		if !write {
			state, err := deps.PostUpdates.Status(c.Request.Context())
			if err != nil {
				apiFailure(c, 500, "update_settings_failed", err.Error(), nil)
				return
			}
			apiRespond(c, 200, state)
			return
		}
		if !requireStudioBrowser(c) {
			return
		}
		var settings service.PostUpdateSettings
		if !decodeAPIJSON(c, &settings) {
			return
		}
		state, err := deps.PostUpdates.Configure(c.Request.Context(), settings)
		if err != nil {
			apiFailure(c, 400, "update_settings_rejected", err.Error(), nil)
			return
		}
		apiRespond(c, 200, state)
	}
}

func apiRefreshSavedPost(deps Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireStudioBrowser(c) {
			return
		}
		if deps.PostUpdates == nil {
			apiFailure(c, 503, "capability_unavailable", "更新服务不可用", nil)
			return
		}
		pid, err := strconv.ParseInt(c.Param("pid"), 10, 32)
		if err != nil || pid <= 0 {
			apiFailure(c, 400, "invalid_input", "PID 必须为正整数", nil)
			return
		}
		var body struct {
			Full bool `json:"full"`
		}
		if !decodeAPIJSON(c, &body) {
			return
		}
		if body.Full {
			if deps.Jobs == nil || deps.Repository == nil {
				apiFailure(c, 503, "capability_unavailable", "任务服务不可用", nil)
				return
			}
			visible, err := deps.Repository.GetVisiblePostIDs([]int32{int32(pid)})
			if err != nil || !visible[int32(pid)] {
				apiFailure(c, 400, "not_saved", "请先保存此洞", nil)
				return
			}
			media, err := deps.Repository.SavedMediaPreference(c.Request.Context(), int32(pid))
			if err != nil {
				apiFailure(c, 500, "refresh_failed", err.Error(), nil)
				return
			}
			state, err := deps.PostUpdates.Status(c.Request.Context())
			if err != nil || state.Account == "" {
				apiFailure(c, 401, "online_login_required", "请先登录树洞", nil)
				return
			}
			job, err := createOrReusePIDJob(c.Request.Context(), deps, map[string]any{"pids": []int32{int32(pid)}, "include_comments": true, "include_media": media, "account": state.Account}, 1)
			if err != nil {
				apiFailure(c, 400, "job_rejected", err.Error(), nil)
				return
			}
			apiRespond(c, http.StatusAccepted, toPublicJob(job))
			return
		}
		result, err := deps.PostUpdates.RefreshSaved(c.Request.Context(), int32(pid), false)
		if err != nil {
			apiFailure(c, 502, "refresh_failed", err.Error(), nil)
			return
		}
		apiRespond(c, 200, result)
	}
}
