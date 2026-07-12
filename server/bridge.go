package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/archive"
	"github.com/Susurrium/PkuHoleStudio/internal/jobs"
)

const bridgePairingLifetime = 5 * time.Minute

type BridgePairing struct {
	Token     string                   `json:"token"`
	Code      string                   `json:"code,omitempty"`
	Status    string                   `json:"status"`
	ExpiresAt time.Time                `json:"expires_at"`
	Filename  string                   `json:"filename,omitempty"`
	Preflight *archive.PreflightReport `json:"preflight,omitempty"`
	Job       *publicJob               `json:"job,omitempty"`
	path      string
	size      int64
}

type BridgeManager struct {
	mu       sync.Mutex
	dataDir  string
	archive  serviceArchive
	jobs     *jobs.Manager
	pairings map[string]*BridgePairing
	now      func() time.Time
}

type serviceArchive interface {
	Preflight(context.Context, io.ReaderAt, int64) (archive.PreflightReport, error)
}

func NewBridgeManager(dataDir string, archiveService serviceArchive, jobManager *jobs.Manager) *BridgeManager {
	return &BridgeManager{dataDir: dataDir, archive: archiveService, jobs: jobManager, pairings: make(map[string]*BridgePairing), now: time.Now}
}

func (m *BridgeManager) Create(codePrefix string) (BridgePairing, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return BridgePairing{}, err
	}
	token := hex.EncodeToString(raw[:])
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked()
	pairing := &BridgePairing{Token: token, Code: codePrefix + token, Status: "waiting_upload", ExpiresAt: m.now().Add(bridgePairingLifetime)}
	m.pairings[token] = pairing
	return clonePairing(pairing), nil
}

func (m *BridgeManager) Get(token string) (BridgePairing, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupLocked()
	pairing, ok := m.pairings[token]
	if !ok {
		return BridgePairing{}, false
	}
	return clonePairing(pairing), true
}

func (m *BridgeManager) Upload(ctx context.Context, token, filename string, source io.Reader) (BridgePairing, error) {
	m.mu.Lock()
	m.cleanupLocked()
	pairing, ok := m.pairings[token]
	if !ok {
		m.mu.Unlock()
		return BridgePairing{}, os.ErrNotExist
	}
	if pairing.Status != "waiting_upload" {
		m.mu.Unlock()
		return BridgePairing{}, errors.New("pairing has already received an archive")
	}
	pairing.Status = "uploading"
	m.mu.Unlock()

	dir := filepath.Join(m.dataDir, "imports", "bridge")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		m.restoreWaiting(token)
		return BridgePairing{}, err
	}
	staged, err := os.CreateTemp(dir, "bridge-*.treehole.zip")
	if err != nil {
		m.restoreWaiting(token)
		return BridgePairing{}, err
	}
	path := staged.Name()
	keep := false
	defer func() {
		_ = staged.Close()
		if !keep {
			_ = os.Remove(path)
		}
	}()
	written, err := io.Copy(staged, io.LimitReader(source, archive.MaxArchiveBytes+1))
	if err != nil || written <= 0 || written > archive.MaxArchiveBytes {
		m.restoreWaiting(token)
		return BridgePairing{}, errors.New("archive is empty, unreadable, or too large")
	}
	if err := staged.Sync(); err != nil {
		m.restoreWaiting(token)
		return BridgePairing{}, err
	}
	if _, err := staged.Seek(0, io.SeekStart); err != nil {
		m.restoreWaiting(token)
		return BridgePairing{}, err
	}
	preflight, err := m.archive.Preflight(ctx, staged, written)
	if err != nil {
		m.restoreWaiting(token)
		return BridgePairing{}, err
	}
	if preflight.Counts.ValidItems == 0 {
		m.restoreWaiting(token)
		return BridgePairing{}, errors.New("archive contains no valid items")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	pairing, ok = m.pairings[token]
	if !ok || !m.now().Before(pairing.ExpiresAt) {
		return BridgePairing{}, os.ErrNotExist
	}
	pairing.Status = "awaiting_confirmation"
	pairing.Filename = filepath.Base(filename)
	pairing.Preflight = &preflight
	pairing.path = path
	pairing.size = written
	keep = true
	return clonePairing(pairing), nil
}

func (m *BridgeManager) Confirm(ctx context.Context, token string) (BridgePairing, error) {
	m.mu.Lock()
	m.cleanupLocked()
	pairing, ok := m.pairings[token]
	if !ok {
		m.mu.Unlock()
		return BridgePairing{}, os.ErrNotExist
	}
	if pairing.Status != "awaiting_confirmation" {
		m.mu.Unlock()
		return BridgePairing{}, errors.New("pairing is not awaiting confirmation")
	}
	path, size := pairing.path, pairing.size
	pairing.Status = "confirming"
	m.mu.Unlock()

	absolutePath, _ := filepath.Abs(path)
	job, err := m.jobs.Create(ctx, jobs.CreateRequest{Type: jobs.TypeImportArchive, Payload: map[string]any{"path": absolutePath, "size": size}, TotalItems: 1})
	if err != nil {
		m.mu.Lock()
		if current := m.pairings[token]; current != nil && current.Status == "confirming" {
			current.Status = "awaiting_confirmation"
		}
		m.mu.Unlock()
		return BridgePairing{}, err
	}
	public := toPublicJob(job)
	m.mu.Lock()
	defer m.mu.Unlock()
	pairing, ok = m.pairings[token]
	if !ok {
		return BridgePairing{}, os.ErrNotExist
	}
	pairing.Status = "queued"
	pairing.Job = &public
	pairing.path = ""
	return clonePairing(pairing), nil
}

func (m *BridgeManager) Cancel(token string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	pairing, ok := m.pairings[token]
	if !ok {
		return false
	}
	if pairing.path != "" {
		_ = os.Remove(pairing.path)
	}
	delete(m.pairings, token)
	return true
}

func (m *BridgeManager) restoreWaiting(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pairing := m.pairings[token]; pairing != nil {
		pairing.Status = "waiting_upload"
	}
}

func (m *BridgeManager) cleanupLocked() {
	now := m.now()
	for token, pairing := range m.pairings {
		if !now.Before(pairing.ExpiresAt) {
			if pairing.path != "" {
				_ = os.Remove(pairing.path)
			}
			delete(m.pairings, token)
		}
	}
}

func clonePairing(pairing *BridgePairing) BridgePairing {
	clone := *pairing
	if pairing.Preflight != nil {
		preflight := *pairing.Preflight
		clone.Preflight = &preflight
	}
	if pairing.Job != nil {
		job := *pairing.Job
		clone.Job = &job
	}
	clone.path = ""
	clone.size = 0
	return clone
}
