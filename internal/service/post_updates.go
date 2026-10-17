package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/client"
	"github.com/Susurrium/PkuHoleStudio/internal/models"
)

const MaxWatchedPosts = 20
const UpdateRoundBudget = 100

var errUpdateScheduleChanged = errors.New("更新设置已发生变化，本轮执行停止")

type PostUpdateRepository interface {
	Repository
	PostPresenceRepository
	ReadPostUpdateSchedule(context.Context, string) (string, error)
	WritePostUpdateSchedule(context.Context, string, string) error
	ComparePostUpdateSchedule(context.Context, string, string, string) (bool, error)
	SavedMediaPreference(context.Context, int32) (bool, error)
	SaveCrawlResultWithSource([]models.Post, []models.Comment, string, string) error
	GetMediaByPID(int32) ([]models.Media, error)
	RebuildReferences(context.Context) (int, error)
	GetPostAvailability(context.Context, int32) (models.PostAvailability, bool, error)
}

type WatchedPost struct {
	PID      int32  `json:"pid"`
	Page     int    `json:"page,omitempty"`
	Boundary int32  `json:"boundary,omitempty"`
	Head     int32  `json:"head,omitempty"`
	Phase    string `json:"phase,omitempty"`
	Error    string `json:"error,omitempty"`
}
type PostUpdateSettings struct {
	Enabled         bool    `json:"enabled"`
	IntervalMinutes int     `json:"interval_minutes"`
	PIDs            []int32 `json:"pids"`
}
type PostUpdateState struct {
	stored string
	PostUpdateSettings
	Account  string        `json:"account,omitempty"`
	NextRun  time.Time     `json:"next_run"`
	Targets  []WatchedPost `json:"targets"`
	Cursor   int           `json:"cursor"`
	Running  bool          `json:"running"`
	Error    string        `json:"error,omitempty"`
	Revision uint64        `json:"revision"`
}
type PostUpdateResult struct {
	PID          int32 `json:"pid"`
	Comments     int   `json:"comments"`
	CommentPages int   `json:"comment_pages"`
	Media        int   `json:"media"`
	MissingMedia int   `json:"missing_media"`
}
type updateFlight struct {
	comments bool
	media    bool
	account  string
	partial  bool
	done     chan struct{}
	result   PostUpdateResult
	err      error
	full     bool
}

type PartialPostUpdateError struct{ Err error }

func (e *PartialPostUpdateError) Error() string { return e.Err.Error() }
func (e *PartialPostUpdateError) Unwrap() error { return e.Err }
func (e *PartialPostUpdateError) Partial() bool { return true }

type PostUpdateService struct {
	posts       *PostService
	repo        PostUpdateRepository
	media       *MediaService
	account     func() (string, error)
	mu          sync.Mutex
	flights     map[int32]*updateFlight
	settingsMu  sync.Mutex
	roundMu     sync.Mutex
	roundCancel context.CancelFunc
	roundDone   chan struct{}
	revision    atomic.Uint64
	foreground  atomic.Int32
	cancel      context.CancelFunc
	done        chan struct{}
}

func NewPostUpdateService(posts *PostService, repo PostUpdateRepository, media *MediaService, account func() (string, error)) *PostUpdateService {
	return &PostUpdateService{posts: posts, repo: repo, media: media, account: account, flights: make(map[int32]*updateFlight)}
}

func (s *PostUpdateService) checkAccount(account string) error {
	current, err := s.account()
	if err != nil {
		return fmt.Errorf("%w: %v", client.ErrAccountChanged, err)
	}
	if current != account {
		return client.ErrAccountChanged
	}
	return nil
}

func (s *PostUpdateService) load(ctx context.Context, account string) (PostUpdateState, error) {
	state := PostUpdateState{PostUpdateSettings: PostUpdateSettings{IntervalMinutes: 60, PIDs: []int32{}}, Account: account, Targets: []WatchedPost{}}
	raw, err := s.repo.ReadPostUpdateSchedule(ctx, account)
	if err != nil {
		return state, err
	}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &state); err != nil {
			return state, fmt.Errorf("读取自动更新设置: %w", err)
		}
	}
	if state.Account != account {
		return state, errors.New("自动更新设置账号不匹配")
	}
	if state.IntervalMinutes != 30 && state.IntervalMinutes != 60 && state.IntervalMinutes != 120 {
		return state, errors.New("自动更新周期配置无效")
	}
	if len(state.Targets) > MaxWatchedPosts || len(state.PIDs) > MaxWatchedPosts || state.Cursor < 0 {
		return state, errors.New("自动更新目标配置无效")
	}
	state.Revision = max(state.Revision, s.revision.Load())
	state.stored = raw
	return state, nil
}
func (s *PostUpdateService) persist(ctx context.Context, state PostUpdateState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.repo.WritePostUpdateSchedule(ctx, state.Account, string(raw))
}

func (s *PostUpdateService) persistRound(ctx context.Context, state *PostUpdateState) error {
	state.Revision = max(state.Revision+1, s.revision.Load())
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	written, err := s.repo.ComparePostUpdateSchedule(ctx, state.Account, state.stored, string(raw))
	if err != nil {
		return err
	}
	if !written {
		return errUpdateScheduleChanged
	}
	state.stored = string(raw)
	return nil
}
func (s *PostUpdateService) Status(ctx context.Context) (PostUpdateState, error) {
	account, err := s.account()
	if err != nil {
		return PostUpdateState{PostUpdateSettings: PostUpdateSettings{IntervalMinutes: 60, PIDs: []int32{}}, Targets: []WatchedPost{}, Revision: s.revision.Load(), Error: err.Error()}, nil
	}
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	state, err := s.load(ctx, account)
	state.Running = s.roundCancel != nil
	return state, err
}

func (s *PostUpdateService) Configure(ctx context.Context, settings PostUpdateSettings) (PostUpdateState, error) {
	if settings.IntervalMinutes != 30 && settings.IntervalMinutes != 60 && settings.IntervalMinutes != 120 {
		return PostUpdateState{}, errors.New("更新周期只能为 30、60 或 120 分钟")
	}
	if len(settings.PIDs) > MaxWatchedPosts {
		return PostUpdateState{}, errors.New("持续更新最多选择 20 个树洞")
	}
	seen := map[int32]bool{}
	for _, pid := range settings.PIDs {
		if pid <= 0 || seen[pid] {
			return PostUpdateState{}, errors.New("PID 必须是互不重复的正整数")
		}
		seen[pid] = true
	}
	account, err := s.account()
	if err != nil {
		return PostUpdateState{}, err
	}
	ctx = client.WithRequestAccount(ctx, account)
	{
		s.settingsMu.Lock()
		cancel, done := s.roundCancel, s.roundDone
		s.settingsMu.Unlock()
		if cancel != nil {
			cancel()
			select {
			case <-done:
			case <-ctx.Done():
				return PostUpdateState{}, ctx.Err()
			}
		}
	}
	visible, err := s.repo.GetVisiblePostIDs(settings.PIDs)
	if err != nil {
		return PostUpdateState{}, err
	}
	for _, pid := range settings.PIDs {
		if !visible[pid] {
			return PostUpdateState{}, fmt.Errorf("请先保存树洞 #%d", pid)
		}
		if !settings.Enabled {
			continue
		}
		post, err := s.posts.RefreshPost(ctx, pid, SourceLive)
		if err != nil {
			return PostUpdateState{}, err
		}
		if post == nil || !bool(post.IsFollow) {
			return PostUpdateState{}, fmt.Errorf("树洞 #%d 当前未关注", pid)
		}
	}
	if err := s.checkAccount(account); err != nil {
		return PostUpdateState{}, err
	}
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	state, err := s.load(ctx, account)
	if err != nil {
		return state, err
	}
	if s.roundCancel != nil {
		return state, errors.New("正在执行更新，请等待本轮结束后修改设置")
	}
	old := map[int32]WatchedPost{}
	for _, target := range state.Targets {
		old[target.PID] = target
	}
	state.Targets = []WatchedPost{}
	for _, pid := range settings.PIDs {
		target := old[pid]
		target.PID = pid
		state.Targets = append(state.Targets, target)
	}
	state.PostUpdateSettings = settings
	state.Running = false
	state.Cursor = 0
	state.Error = ""
	// 保留已预订的下轮时间，反复开关或重启不能缩短更新间隔。
	if state.NextRun.IsZero() {
		state.NextRun = time.Now().UTC().Add(time.Duration(settings.IntervalMinutes) * time.Minute)
	}
	if len(settings.PIDs) == 0 {
		state.Enabled = false
	}
	return state, s.persist(ctx, state)
}

func (s *PostUpdateService) Capture(ctx context.Context, pid int32, comments, media bool) (PostUpdateResult, error) {
	availability, found, err := s.repo.GetPostAvailability(ctx, pid)
	if err != nil {
		return PostUpdateResult{}, err
	}
	if found && availability.State == "confirmed_unavailable" {
		return PostUpdateResult{}, errors.New("历史删除快照保持原样，可查看线上版本")
	}
	return s.refresh(ctx, pid, true, comments, media)
}
func (s *PostUpdateService) RefreshSaved(ctx context.Context, pid int32, full bool) (PostUpdateResult, error) {
	visible, err := s.repo.GetVisiblePostIDs([]int32{pid})
	if err != nil {
		return PostUpdateResult{}, err
	}
	if !visible[pid] {
		return PostUpdateResult{}, errors.New("此洞尚未保存到本地")
	}
	availability, found, err := s.repo.GetPostAvailability(ctx, pid)
	if err != nil {
		return PostUpdateResult{}, err
	}
	if found && availability.State == "confirmed_unavailable" {
		return PostUpdateResult{}, errors.New("历史删除快照保持原样，可查看线上版本")
	}
	media, err := s.repo.SavedMediaPreference(ctx, pid)
	if err != nil {
		return PostUpdateResult{}, err
	}
	return s.refresh(ctx, pid, full, true, full && media)
}

func (s *PostUpdateService) refresh(ctx context.Context, pid int32, full, comments, media bool) (PostUpdateResult, error) {
	s.foreground.Add(1)
	defer s.foreground.Add(-1)
	if pid <= 0 {
		return PostUpdateResult{}, errors.New("PID 必须为正整数")
	}
	account, err := s.account()
	if err != nil {
		return PostUpdateResult{}, fmt.Errorf("%w: %v", client.ErrAccountChanged, err)
	}
	if expected := client.RequestAccount(ctx); expected != "" && expected != account {
		return PostUpdateResult{}, client.ErrAccountChanged
	}
	ctx = client.WithRequestAccount(ctx, account)
	for {
		s.mu.Lock()
		flight := s.flights[pid]
		if flight == nil {
			flight = &updateFlight{done: make(chan struct{}), full: full, account: account, comments: comments, media: media}
			s.flights[pid] = flight
			s.mu.Unlock()
			flight.result, flight.err = s.fetch(ctx, account, pid, full, comments, media)
			s.mu.Lock()
			delete(s.flights, pid)
			close(flight.done)
			s.mu.Unlock()
			return flight.result, flight.err
		}
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return PostUpdateResult{}, ctx.Err()
		case <-flight.done:
		}
		if !flight.partial && flight.account == account && (!full || flight.full) && (!comments || flight.comments) && (!media || flight.media) {
			return flight.result, flight.err
		}
	}
}

func (s *PostUpdateService) fetch(ctx context.Context, account string, pid int32, full, comments, media bool) (PostUpdateResult, error) {
	result := PostUpdateResult{PID: pid}
	post, err := s.posts.RefreshPost(ctx, pid, SourceLive)
	if err != nil {
		return result, err
	}
	if post == nil || post.Pid != pid {
		return result, errors.New("线上正文 PID 不匹配")
	}
	var boundary int32
	if !full {
		known, err := s.repo.GetCommentsByPidCursor(pid, 0, 1, false)
		if err != nil {
			return result, err
		}
		if len(known) > 0 {
			boundary = known[0].Cid
		}
		s.settingsMu.Lock()
		state, loadErr := s.load(ctx, account)
		s.settingsMu.Unlock()
		if loadErr != nil {
			return result, loadErr
		}
		for _, target := range state.Targets {
			if target.PID == pid && target.Page > 0 {
				boundary = min(boundary, target.Boundary)
			}
		}
	}
	items := []models.Comment{}
	seen := map[int32]bool{}
	if comments {
		for page := 1; ; page++ {
			if page > 10000 {
				return result, errors.New("评论分页超过安全上限")
			}
			order := "desc"
			if full {
				order = "asc"
			}
			part, err := s.posts.Comments(ctx, pid, CommentQuery{Source: SourceLive, Sort: order, Cursor: int32(page - 1), Limit: 100})
			if err != nil {
				return result, err
			}
			result.CommentPages++
			stop, added := false, 0
			for _, item := range part.Items {
				if item.Pid != pid || item.Cid <= 0 {
					return result, errors.New("评论标识或所属树洞异常")
				}
				if item.Cid <= boundary {
					stop = true
				}
				if !seen[item.Cid] {
					seen[item.Cid] = true
					items = append(items, item)
					added++
				}
			}
			if !part.HasMore || stop {
				break
			}
			if added == 0 {
				return result, errors.New("评论分页没有取得新记录")
			}
		}
	}
	if err := s.checkAccount(account); err != nil {
		return result, err
	}
	// 新评论读取完整后一起提交，避免中途失败导致下一次误以为缺页已经保存。
	if err := s.repo.SaveCrawlResultWithSource([]models.Post{*post}, items, "explicit", "treehole-live"); err != nil {
		return result, err
	}
	result.Comments = len(items)
	s.revision.Add(1)
	if _, err := s.repo.RebuildReferences(ctx); err != nil {
		return result, &PartialPostUpdateError{Err: err}
	}
	if media {
		rows, err := s.repo.GetMediaByPID(pid)
		if err != nil {
			return result, err
		}
		var failures []error
		for _, row := range rows {
			if row.Status == "available" && row.Path != "" {
				result.Media++
				continue
			}
			if _, err := s.media.Repair(ctx, models.MediaRepairCandidate{Media: row, PID: pid}); err != nil {
				result.MissingMedia++
				failures = append(failures, err)
				if ctx.Err() != nil || IsPostUpdateInterruption(err) {
					break
				}
			} else {
				result.Media++
			}
		}
		if len(failures) > 0 {
			return result, &PartialPostUpdateError{Err: fmt.Errorf("正文与评论已更新，%d 张图片未保存: %w", result.MissingMedia, errors.Join(failures...))}
		}
	}
	return result, nil
}

func (s *PostUpdateService) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := s.account(); err == nil {
					if err := s.RunDue(ctx, time.Now().UTC()); err != nil && ctx.Err() == nil {
						log.Printf("[PostUpdates] %v", err)
					}
				}
			}
		}
	}()
}
func (s *PostUpdateService) Close() {
	if s.cancel != nil {
		s.cancel()
		<-s.done
	}
}

func (s *PostUpdateService) RunDue(ctx context.Context, now time.Time) error {
	if !s.roundMu.TryLock() {
		return nil
	}
	defer s.roundMu.Unlock()
	account, err := s.account()
	if err != nil {
		return err
	}
	s.settingsMu.Lock()
	state, err := s.load(ctx, account)
	if err != nil || !state.Enabled || state.NextRun.After(now) || len(state.Targets) == 0 {
		s.settingsMu.Unlock()
		return err
	}
	// 先持久化下轮时间，再开始请求；进程意外结束也不能提前执行下一轮。
	state.Running = true
	state.NextRun = now.Add(time.Duration(state.IntervalMinutes) * time.Minute)
	state.Error = ""
	if err := s.persistRound(ctx, &state); err != nil {
		s.settingsMu.Unlock()
		if errors.Is(err, errUpdateScheduleChanged) {
			return nil
		}
		return err
	}
	ctx, cancelRound := context.WithTimeout(ctx, 5*time.Minute)
	s.roundCancel = cancelRound
	s.roundDone = make(chan struct{})
	defer func() {
		cancelRound()
		s.settingsMu.Lock()
		s.roundCancel = nil
		close(s.roundDone)
		s.settingsMu.Unlock()
	}()
	s.settingsMu.Unlock()
	budget := client.NewRequestBudget(UpdateRoundBudget)
	ctx = client.WithRequestAccount(client.WithRequestBudget(ctx, budget), account)
	completed := map[int32]bool{}
	validated := map[int32]bool{}
	for steps := 0; steps < UpdateRoundBudget && budget.Remaining() > 0 && len(completed) < len(state.Targets); steps++ {
		for s.foreground.Load() > 0 && ctx.Err() == nil {
			timer := time.NewTimer(50 * time.Millisecond)
			select {
			case <-timer.C:
			case <-ctx.Done():
			}
			timer.Stop()
		}
		if ctx.Err() != nil {
			state.Error = ctx.Err().Error()
			break
		}
		index := state.Cursor % len(state.Targets)
		state.Cursor = (index + 1) % len(state.Targets)
		target := &state.Targets[index]
		if completed[target.PID] {
			continue
		}
		if err := s.checkAccount(account); err != nil {
			state.Error = err.Error()
			break
		}
		s.mu.Lock()
		if s.flights[target.PID] != nil {
			s.mu.Unlock()
			completed[target.PID] = true
			continue
		}
		flight := &updateFlight{done: make(chan struct{}), account: account, partial: true}
		s.flights[target.PID] = flight
		s.mu.Unlock()
		done, stepErr := s.watchStep(ctx, account, target, !validated[target.PID])
		validated[target.PID] = true
		flight.result.PID = target.PID
		flight.err = stepErr
		s.mu.Lock()
		delete(s.flights, target.PID)
		close(flight.done)
		s.mu.Unlock()
		if done {
			completed[target.PID] = true
		}
		if stepErr != nil {
			target.Error = stepErr.Error()
			completed[target.PID] = true
			if errors.Is(stepErr, client.ErrRequestBudget) || ctx.Err() != nil || IsPostUpdateInterruption(stepErr) {
				state.Error = stepErr.Error()
				break
			}
		}
		if err := s.persistRound(ctx, &state); err != nil {
			state.Error = err.Error()
			break
		}
	}
	kept := []WatchedPost{}
	state.PIDs = []int32{}
	for _, target := range state.Targets {
		if target.Phase != "unfollowed" {
			kept = append(kept, target)
			state.PIDs = append(state.PIDs, target.PID)
		}
	}
	state.Targets = kept
	state.Running = false
	if len(kept) == 0 {
		state.Enabled = false
		state.Cursor = 0
	}
	// 退出或取消时也保存已经完成的分页；数据库写入单独设定短超时。
	saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	err = s.persistRound(saveCtx, &state)
	if errors.Is(err, errUpdateScheduleChanged) {
		return nil
	}
	return err
}

func (s *PostUpdateService) watchStep(ctx context.Context, account string, target *WatchedPost, header bool) (bool, error) {
	visible, err := s.repo.GetVisiblePostIDs([]int32{target.PID})
	if err != nil {
		return true, err
	}
	if !visible[target.PID] {
		return true, errors.New("本地资料不存在，已暂停此洞更新")
	}
	availability, found, err := s.repo.GetPostAvailability(ctx, target.PID)
	if err != nil {
		return true, err
	}
	if found && availability.State == "confirmed_unavailable" {
		return true, errors.New("历史删除快照不自动更新")
	}
	if header {
		post, err := s.posts.RefreshPost(ctx, target.PID, SourceLive)
		if err != nil {
			return true, err
		}
		if post == nil || post.Pid != target.PID {
			return true, errors.New("线上正文 PID 不匹配")
		}
		if !bool(post.IsFollow) {
			target.Phase = "unfollowed"
			return true, nil
		}
		if err := s.checkAccount(account); err != nil {
			return true, err
		}
		if target.Page == 0 {
			known, err := s.repo.GetCommentsByPidCursor(target.PID, 0, 1, false)
			if err != nil {
				return true, err
			}
			if len(known) > 0 {
				target.Boundary = known[0].Cid
			}
			target.Page = 1
		}
		if err := s.repo.SaveCrawlResultWithSource([]models.Post{*post}, nil, "explicit", "treehole-live"); err != nil {
			return true, err
		}
		s.revision.Add(1)
		if target.Phase == "" || target.Phase == "post" {
			target.Phase = "comments"
		}
		target.Error = ""
		return false, nil
	}
	if target.Phase == "comments" {
		page, err := s.posts.Comments(ctx, target.PID, CommentQuery{Source: SourceLive, Sort: "desc", Cursor: int32(target.Page - 1), Limit: 100})
		if err != nil {
			return true, err
		}
		stop := !page.HasMore
		if len(page.Items) == 0 && page.HasMore {
			return true, errors.New("评论分页为空，更新已暂停")
		}
		for _, item := range page.Items {
			if item.Pid != target.PID || item.Cid <= 0 {
				return true, errors.New("评论标识异常")
			}
			target.Head = max(target.Head, item.Cid)
			if item.Cid <= target.Boundary {
				stop = true
			}
		}
		if err := s.checkAccount(account); err != nil {
			return true, err
		}
		if err := s.repo.SaveCrawlResultWithSource(nil, page.Items, "", ""); err != nil {
			return true, err
		}
		s.revision.Add(1)
		if !stop {
			target.Page++
			if target.Page > 10000 {
				return true, errors.New("评论分页超过安全上限")
			}
			return false, nil
		}
		if _, err := s.repo.RebuildReferences(ctx); err != nil {
			return true, err
		}
		target.Boundary = max(target.Boundary, target.Head)
		target.Page = 0
		target.Head = 0
		target.Phase = "media"
		return false, nil
	}
	media, err := s.repo.SavedMediaPreference(ctx, target.PID)
	if err != nil {
		return true, err
	}
	if media {
		rows, err := s.repo.GetMediaByPID(target.PID)
		if err != nil {
			return true, err
		}
		for _, row := range rows {
			if row.Status != "available" || row.Path == "" {
				if _, err := s.media.Repair(ctx, models.MediaRepairCandidate{Media: row, PID: target.PID}); err != nil {
					return true, err
				}
				s.revision.Add(1)
				return false, nil
			}
		}
	}
	target.Phase = "post"
	target.Error = ""
	return true, nil
}

func IsPostUpdateInterruption(err error) bool {
	if errors.Is(err, client.ErrAccountChanged) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var remote *client.RemoteAPIError
	if errors.As(err, &remote) {
		return remote.HTTPStatus == 401 || remote.HTTPStatus == 403 || remote.HTTPStatus == 429 || remote.HTTPStatus >= 500
	}
	var network net.Error
	return errors.As(err, &network) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF)
}
