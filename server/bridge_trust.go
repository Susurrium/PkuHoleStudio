package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/archive"
	"github.com/Susurrium/PkuHoleStudio/internal/jobs"
)

const (
	bridgeDeviceRequestLifetime = 10 * time.Minute
	bridgeChallengeLifetime     = 5 * time.Minute
	bridgeTransferLifetime      = 10 * time.Minute
	bridgeTrustedQueuedLifetime = time.Hour
	maxPendingDeviceRequests    = 10
	maxActiveBridgeTransfers    = 20
)

// BridgeDevice is a revocable, long-lived trust relationship with one
// userscript installation. Studio persists only the public key.
type BridgeDevice struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at,omitempty"`
	publicSPKI string
}

type BridgeDeviceRequest struct {
	Token            string    `json:"token"`
	Name             string    `json:"name"`
	VerificationCode string    `json:"verification_code"`
	Status           string    `json:"status"`
	ExpiresAt        time.Time `json:"expires_at"`
	DeviceID         string    `json:"device_id,omitempty"`
	InstanceID       string    `json:"instance_id,omitempty"`
	publicSPKI       string
}

type BridgeChallenge struct {
	Token      string    `json:"challenge"`
	DeviceID   string    `json:"device_id"`
	InstanceID string    `json:"instance_id"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type BridgeTransferRequest struct {
	DeviceID   string `json:"device_id"`
	InstanceID string `json:"instance_id"`
	Challenge  string `json:"challenge"`
	Filename   string `json:"filename"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
	Signature  string `json:"signature"`
}

type BridgeDeviceRevokeRequest struct {
	DeviceID   string `json:"device_id"`
	InstanceID string `json:"instance_id"`
	Challenge  string `json:"challenge"`
	Signature  string `json:"signature"`
}

type BridgeTransfer struct {
	ID           string                   `json:"id"`
	DeviceID     string                   `json:"device_id"`
	DeviceName   string                   `json:"device_name"`
	Filename     string                   `json:"filename"`
	Size         int64                    `json:"size"`
	SHA256       string                   `json:"sha256"`
	Status       string                   `json:"status"`
	CreatedAt    time.Time                `json:"created_at"`
	ExpiresAt    time.Time                `json:"expires_at"`
	UploadTicket string                   `json:"upload_ticket,omitempty"`
	Preflight    *archive.PreflightReport `json:"preflight,omitempty"`
	Job          *publicJob               `json:"job,omitempty"`
	path         string
	ticketHash   [32]byte
}

type bridgeDeviceFile struct {
	Version    int                  `json:"version"`
	InstanceID string               `json:"instance_id"`
	Devices    []bridgeStoredDevice `json:"devices"`
}

type bridgeStoredDevice struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	PublicSPKI string    `json:"public_key_spki"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at,omitempty"`
}

func (m *BridgeManager) initTrustedBridge() {
	m.devices = make(map[string]*BridgeDevice)
	m.deviceRequests = make(map[string]*BridgeDeviceRequest)
	m.challenges = make(map[string]*BridgeChallenge)
	m.transfers = make(map[string]*BridgeTransfer)
	encoded, err := os.ReadFile(m.bridgeDevicePath())
	if err == nil {
		var stored bridgeDeviceFile
		if json.Unmarshal(encoded, &stored) == nil && stored.Version == 1 {
			m.instanceID = strings.TrimSpace(stored.InstanceID)
			for _, candidate := range stored.Devices {
				if candidate.ID == "" || candidate.PublicSPKI == "" {
					continue
				}
				if _, err := parseBridgePublicKey(candidate.PublicSPKI); err != nil {
					continue
				}
				m.devices[candidate.ID] = &BridgeDevice{
					ID: candidate.ID, Name: candidate.Name, CreatedAt: candidate.CreatedAt,
					LastUsedAt: candidate.LastUsedAt, publicSPKI: candidate.PublicSPKI,
				}
			}
		}
	}
	if m.instanceID == "" {
		m.instanceID, _ = bridgeRandomHex(16)
	}
}

func (m *BridgeManager) bridgeDevicePath() string {
	return filepath.Join(m.dataDir, "bridge-devices.json")
}

func (m *BridgeManager) CreateDeviceRequest(name, publicSPKI string) (BridgeDeviceRequest, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Toolkit"
	}
	if len(name) > 80 {
		return BridgeDeviceRequest{}, errors.New("device name is too long")
	}
	if _, err := parseBridgePublicKey(publicSPKI); err != nil {
		return BridgeDeviceRequest{}, err
	}
	token, err := bridgeRandomHex(24)
	if err != nil {
		return BridgeDeviceRequest{}, err
	}
	code, err := bridgeVerificationCode()
	if err != nil {
		return BridgeDeviceRequest{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupTrustedLocked(m.now())
	pendingRequests := 0
	for _, existing := range m.deviceRequests {
		if existing.Status == "pending" {
			pendingRequests++
		}
	}
	if pendingRequests >= maxPendingDeviceRequests {
		return BridgeDeviceRequest{}, errors.New("too many pending device requests")
	}
	request := &BridgeDeviceRequest{
		Token: token, Name: name, VerificationCode: code, Status: "pending",
		ExpiresAt: m.now().Add(bridgeDeviceRequestLifetime), publicSPKI: publicSPKI,
	}
	m.deviceRequests[token] = request
	return cloneDeviceRequest(request), nil
}

func (m *BridgeManager) DeviceRequest(token string) (BridgeDeviceRequest, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupTrustedLocked(m.now())
	request, ok := m.deviceRequests[token]
	if !ok {
		return BridgeDeviceRequest{}, false
	}
	return cloneDeviceRequest(request), true
}

func (m *BridgeManager) PendingDeviceRequests() []BridgeDeviceRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupTrustedLocked(m.now())
	result := make([]BridgeDeviceRequest, 0, len(m.deviceRequests))
	for _, request := range m.deviceRequests {
		if request.Status == "pending" {
			result = append(result, cloneDeviceRequest(request))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ExpiresAt.Before(result[j].ExpiresAt) })
	return result
}

func (m *BridgeManager) ApproveDeviceRequest(token string) (BridgeDeviceRequest, error) {
	deviceID, err := bridgeRandomHex(16)
	if err != nil {
		return BridgeDeviceRequest{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupTrustedLocked(m.now())
	request := m.deviceRequests[token]
	if request == nil || request.Status != "pending" {
		return BridgeDeviceRequest{}, os.ErrNotExist
	}
	device := &BridgeDevice{
		ID: deviceID, Name: request.Name, CreatedAt: m.now(), publicSPKI: request.publicSPKI,
	}
	m.devices[deviceID] = device
	if err := m.persistBridgeDevicesLocked(); err != nil {
		delete(m.devices, deviceID)
		return BridgeDeviceRequest{}, err
	}
	request.Status = "approved"
	request.DeviceID = deviceID
	request.InstanceID = m.instanceID
	request.ExpiresAt = m.now().Add(5 * time.Minute)
	return cloneDeviceRequest(request), nil
}

func (m *BridgeManager) RejectDeviceRequest(token string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupTrustedLocked(m.now())
	request := m.deviceRequests[token]
	if request == nil || request.Status != "pending" {
		return false
	}
	request.Status = "rejected"
	request.ExpiresAt = m.now().Add(5 * time.Minute)
	return true
}

func (m *BridgeManager) Devices() []BridgeDevice {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]BridgeDevice, 0, len(m.devices))
	for _, device := range m.devices {
		result = append(result, cloneBridgeDevice(device))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result
}

func (m *BridgeManager) RevokeDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.revokeDeviceLocked(id)
}

func (m *BridgeManager) RevokeTrustedDevice(input BridgeDeviceRevokeRequest) error {
	m.mu.Lock()
	m.cleanupTrustedLocked(m.now())
	challenge := m.challenges[input.Challenge]
	device := m.devices[input.DeviceID]
	if challenge == nil || device == nil || challenge.DeviceID != input.DeviceID || challenge.InstanceID != input.InstanceID || input.InstanceID != m.instanceID {
		m.mu.Unlock()
		return errors.New("bridge challenge is invalid or expired")
	}
	delete(m.challenges, input.Challenge)
	publicSPKI := device.publicSPKI
	m.mu.Unlock()
	publicKey, err := parseBridgePublicKey(publicSPKI)
	if err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(input.Signature)
	if err != nil || len(signature) != 64 {
		return errors.New("device revoke signature is invalid")
	}
	digest := sha256.Sum256(bridgeDeviceRevokeMessage(input.DeviceID, input.InstanceID, input.Challenge))
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	if !ecdsa.Verify(publicKey, digest[:], r, s) {
		return errors.New("device revoke signature verification failed")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.revokeDeviceLocked(input.DeviceID)
}

func (m *BridgeManager) revokeDeviceLocked(id string) error {
	device := m.devices[id]
	if device == nil {
		return os.ErrNotExist
	}
	delete(m.devices, id)
	if err := m.persistBridgeDevicesLocked(); err != nil {
		m.devices[id] = device
		return err
	}
	for token, challenge := range m.challenges {
		if challenge.DeviceID == id {
			delete(m.challenges, token)
		}
	}
	for transferID, transfer := range m.transfers {
		if transfer.DeviceID != id || transfer.Status == "uploading" || transfer.Status == "confirming" {
			continue
		}
		if transfer.path != "" {
			_ = os.Remove(transfer.path)
		}
		delete(m.transfers, transferID)
	}
	return nil
}

func (m *BridgeManager) CreateChallenge(deviceID string) (BridgeChallenge, error) {
	token, err := bridgeRandomHex(32)
	if err != nil {
		return BridgeChallenge{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupTrustedLocked(m.now())
	if m.devices[deviceID] == nil {
		return BridgeChallenge{}, os.ErrNotExist
	}
	for key, challenge := range m.challenges {
		if challenge.DeviceID == deviceID {
			delete(m.challenges, key)
		}
	}
	challenge := &BridgeChallenge{
		Token: token, DeviceID: deviceID, InstanceID: m.instanceID,
		ExpiresAt: m.now().Add(bridgeChallengeLifetime),
	}
	m.challenges[token] = challenge
	return *challenge, nil
}

func (m *BridgeManager) CreateTrustedTransfer(input BridgeTransferRequest) (BridgeTransfer, error) {
	filename, err := normalizeBridgeFilename(input.Filename)
	if err != nil {
		return BridgeTransfer{}, err
	}
	hash := strings.ToLower(strings.TrimSpace(input.SHA256))
	if len(hash) != 64 {
		return BridgeTransfer{}, errors.New("archive SHA-256 is invalid")
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return BridgeTransfer{}, errors.New("archive SHA-256 is invalid")
	}
	if input.Size <= 0 || input.Size > archive.MaxArchiveBytes {
		return BridgeTransfer{}, errors.New("archive size is invalid")
	}
	m.mu.Lock()
	m.cleanupTrustedLocked(m.now())
	challenge := m.challenges[input.Challenge]
	device := m.devices[input.DeviceID]
	if challenge == nil || device == nil || challenge.DeviceID != input.DeviceID || challenge.InstanceID != input.InstanceID || input.InstanceID != m.instanceID {
		m.mu.Unlock()
		return BridgeTransfer{}, errors.New("bridge challenge is invalid or expired")
	}
	delete(m.challenges, input.Challenge)
	publicSPKI := device.publicSPKI
	deviceName := device.Name
	m.mu.Unlock()

	publicKey, err := parseBridgePublicKey(publicSPKI)
	if err != nil {
		return BridgeTransfer{}, err
	}
	signature, err := base64.StdEncoding.DecodeString(input.Signature)
	if err != nil || len(signature) != 64 {
		return BridgeTransfer{}, errors.New("transfer signature is invalid")
	}
	digest := sha256.Sum256(bridgeTransferMessage(input.DeviceID, input.InstanceID, input.Challenge, filename, input.Size, hash))
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	if !ecdsa.Verify(publicKey, digest[:], r, s) {
		return BridgeTransfer{}, errors.New("transfer signature verification failed")
	}
	transferID, err := bridgeRandomHex(16)
	if err != nil {
		return BridgeTransfer{}, err
	}
	ticket, err := bridgeRandomHex(32)
	if err != nil {
		return BridgeTransfer{}, err
	}
	ticketHash := sha256.Sum256([]byte(ticket))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupTrustedLocked(m.now())
	device = m.devices[input.DeviceID]
	if device == nil {
		return BridgeTransfer{}, os.ErrNotExist
	}
	activeTransfers := 0
	for _, existing := range m.transfers {
		if existing.Status != "queued" {
			activeTransfers++
		}
	}
	if activeTransfers >= maxActiveBridgeTransfers {
		return BridgeTransfer{}, errors.New("too many active bridge transfers")
	}
	transfer := &BridgeTransfer{
		ID: transferID, DeviceID: input.DeviceID, DeviceName: deviceName,
		Filename: filename, Size: input.Size, SHA256: hash, Status: "waiting_upload",
		CreatedAt: m.now(), ExpiresAt: m.now().Add(bridgeTransferLifetime), ticketHash: ticketHash,
	}
	m.transfers[transferID] = transfer
	device.LastUsedAt = m.now()
	if err := m.persistBridgeDevicesLocked(); err != nil {
		delete(m.transfers, transferID)
		return BridgeTransfer{}, err
	}
	result := cloneBridgeTransfer(transfer)
	result.UploadTicket = ticket
	return result, nil
}

func (m *BridgeManager) UploadTrusted(ctx context.Context, transferID, ticket string, source io.Reader) (BridgeTransfer, error) {
	digest := sha256.Sum256([]byte(ticket))
	m.mu.Lock()
	m.cleanupTrustedLocked(m.now())
	transfer := m.transfers[transferID]
	if transfer == nil || subtle.ConstantTimeCompare(transfer.ticketHash[:], digest[:]) != 1 {
		m.mu.Unlock()
		return BridgeTransfer{}, os.ErrNotExist
	}
	if transfer.Status != "waiting_upload" {
		m.mu.Unlock()
		return BridgeTransfer{}, errors.New("transfer has already received an archive")
	}
	transfer.Status = "uploading"
	transfer.ExpiresAt = m.now().Add(bridgeUploadLifetime)
	expectedSize, expectedHash := transfer.Size, transfer.SHA256
	m.mu.Unlock()

	dir := filepath.Join(m.dataDir, "imports", "staging")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		m.restoreTrustedWaiting(transferID)
		return BridgeTransfer{}, err
	}
	staged, err := os.CreateTemp(dir, "trusted-bridge-*.treehole.zip")
	if err != nil {
		m.restoreTrustedWaiting(transferID)
		return BridgeTransfer{}, err
	}
	path := staged.Name()
	keep := false
	defer func() {
		_ = staged.Close()
		if !keep {
			_ = os.Remove(path)
		}
	}()
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(staged, hasher), io.LimitReader(source, archive.MaxArchiveBytes+1))
	if copyErr != nil || written != expectedSize || written <= 0 || written > archive.MaxArchiveBytes {
		m.restoreTrustedWaiting(transferID)
		return BridgeTransfer{}, errors.New("archive size changed during upload")
	}
	if hex.EncodeToString(hasher.Sum(nil)) != expectedHash {
		m.restoreTrustedWaiting(transferID)
		return BridgeTransfer{}, errors.New("archive SHA-256 changed during upload")
	}
	if err := staged.Sync(); err != nil {
		m.restoreTrustedWaiting(transferID)
		return BridgeTransfer{}, err
	}
	if _, err := staged.Seek(0, io.SeekStart); err != nil {
		m.restoreTrustedWaiting(transferID)
		return BridgeTransfer{}, err
	}
	preflight, err := m.archive.Preflight(ctx, staged, written)
	if err != nil {
		m.restoreTrustedWaiting(transferID)
		return BridgeTransfer{}, err
	}
	if preflight.Counts.ValidItems == 0 {
		m.restoreTrustedWaiting(transferID)
		return BridgeTransfer{}, errors.New("archive contains no valid items")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	transfer = m.transfers[transferID]
	if transfer == nil || transfer.Status != "uploading" {
		return BridgeTransfer{}, os.ErrNotExist
	}
	// A device may be revoked while a large archive is still being streamed or
	// preflighted. Do not turn that in-flight upload into a confirmable import.
	if m.devices[transfer.DeviceID] == nil {
		delete(m.transfers, transferID)
		return BridgeTransfer{}, os.ErrNotExist
	}
	transfer.Status = "awaiting_confirmation"
	transfer.ExpiresAt = m.now().Add(bridgeConfirmationLifetime)
	transfer.Preflight = &preflight
	transfer.path = path
	transfer.ticketHash = [32]byte{}
	keep = true
	return cloneBridgeTransfer(transfer), nil
}

func (m *BridgeManager) ConfirmTrusted(ctx context.Context, transferID string) (BridgeTransfer, error) {
	m.mu.Lock()
	m.cleanupTrustedLocked(m.now())
	transfer := m.transfers[transferID]
	if transfer == nil {
		m.mu.Unlock()
		return BridgeTransfer{}, os.ErrNotExist
	}
	if transfer.Status != "awaiting_confirmation" {
		m.mu.Unlock()
		return BridgeTransfer{}, errors.New("transfer is not awaiting confirmation")
	}
	if m.jobs == nil {
		m.mu.Unlock()
		return BridgeTransfer{}, errors.New("job manager is unavailable")
	}
	path, size := transfer.path, transfer.Size
	transfer.Status = "confirming"
	m.mu.Unlock()

	absolutePath, _ := filepath.Abs(path)
	job, err := m.jobs.Create(ctx, jobs.CreateRequest{Type: jobs.TypeImportArchive, Payload: map[string]any{"path": absolutePath, "size": size}, TotalItems: 1})
	if err != nil {
		m.mu.Lock()
		if current := m.transfers[transferID]; current != nil && current.Status == "confirming" {
			current.Status = "awaiting_confirmation"
			current.ExpiresAt = m.now().Add(bridgeConfirmationLifetime)
		}
		m.mu.Unlock()
		return BridgeTransfer{}, err
	}
	public := toPublicJob(job)
	m.mu.Lock()
	defer m.mu.Unlock()
	transfer = m.transfers[transferID]
	if transfer == nil {
		return BridgeTransfer{}, os.ErrNotExist
	}
	transfer.Status = "queued"
	transfer.ExpiresAt = m.now().Add(bridgeTrustedQueuedLifetime)
	transfer.Job = &public
	transfer.path = ""
	return cloneBridgeTransfer(transfer), nil
}

func (m *BridgeManager) TrustedTransfers() []BridgeTransfer {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupTrustedLocked(m.now())
	result := make([]BridgeTransfer, 0, len(m.transfers))
	for _, transfer := range m.transfers {
		result = append(result, cloneBridgeTransfer(transfer))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result
}

func (m *BridgeManager) CancelTrustedTransfer(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	transfer := m.transfers[id]
	if transfer == nil || transfer.Status == "uploading" || transfer.Status == "confirming" {
		return false
	}
	if transfer.path != "" {
		_ = os.Remove(transfer.path)
	}
	delete(m.transfers, id)
	return true
}

func (m *BridgeManager) restoreTrustedWaiting(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if transfer := m.transfers[id]; transfer != nil && transfer.Status == "uploading" {
		transfer.Status = "waiting_upload"
		transfer.ExpiresAt = m.now().Add(bridgeTransferLifetime)
	}
}

func (m *BridgeManager) cleanupTrustedLocked(now time.Time) {
	for token, request := range m.deviceRequests {
		if !now.Before(request.ExpiresAt) {
			delete(m.deviceRequests, token)
		}
	}
	for token, challenge := range m.challenges {
		if !now.Before(challenge.ExpiresAt) {
			delete(m.challenges, token)
		}
	}
	for id, transfer := range m.transfers {
		if transfer.Status == "uploading" || transfer.Status == "confirming" {
			continue
		}
		if !now.Before(transfer.ExpiresAt) {
			if transfer.path != "" {
				_ = os.Remove(transfer.path)
			}
			delete(m.transfers, id)
		}
	}
}

func (m *BridgeManager) persistBridgeDevicesLocked() error {
	if m.instanceID == "" {
		instanceID, err := bridgeRandomHex(16)
		if err != nil {
			return err
		}
		m.instanceID = instanceID
	}
	stored := bridgeDeviceFile{Version: 1, InstanceID: m.instanceID, Devices: make([]bridgeStoredDevice, 0, len(m.devices))}
	for _, device := range m.devices {
		stored.Devices = append(stored.Devices, bridgeStoredDevice{
			ID: device.ID, Name: device.Name, PublicSPKI: device.publicSPKI,
			CreatedAt: device.CreatedAt, LastUsedAt: device.LastUsedAt,
		})
	}
	sort.Slice(stored.Devices, func(i, j int) bool { return stored.Devices[i].ID < stored.Devices[j].ID })
	encoded, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	path := m.bridgeDevicePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".bridge-devices-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(encoded); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	backup := path + ".bak"
	_ = os.Remove(backup)
	if err := os.Rename(path, backup); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		_ = os.Rename(backup, path)
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func parseBridgePublicKey(encoded string) (*ecdsa.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(raw) > 1024 {
		return nil, errors.New("device public key is invalid")
	}
	parsed, err := x509.ParsePKIXPublicKey(raw)
	if err != nil {
		return nil, errors.New("device public key is invalid")
	}
	publicKey, ok := parsed.(*ecdsa.PublicKey)
	if !ok || publicKey.Curve != elliptic.P256() {
		return nil, errors.New("device public key must use ECDSA P-256")
	}
	return publicKey, nil
}

func bridgeTransferMessage(deviceID, instanceID, challenge, filename string, size int64, hash string) []byte {
	return []byte(strings.Join([]string{
		"pkuhole-bridge-v2", deviceID, instanceID, challenge, filename,
		strconv.FormatInt(size, 10), hash,
	}, "\n"))
}

func bridgeDeviceRevokeMessage(deviceID, instanceID, challenge string) []byte {
	return []byte(strings.Join([]string{"pkuhole-bridge-v2-revoke", deviceID, instanceID, challenge}, "\n"))
}

func normalizeBridgeFilename(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 255 || strings.ContainsAny(value, "\r\n\x00") {
		return "", errors.New("archive filename is invalid")
	}
	value = filepath.Base(value)
	if value == "." || value == string(filepath.Separator) {
		return "", errors.New("archive filename is invalid")
	}
	return value, nil
}

func bridgeRandomHex(bytes int) (string, error) {
	raw := make([]byte, bytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func bridgeVerificationCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

func cloneDeviceRequest(request *BridgeDeviceRequest) BridgeDeviceRequest {
	return BridgeDeviceRequest{
		Token: request.Token, Name: request.Name, VerificationCode: request.VerificationCode,
		Status: request.Status, ExpiresAt: request.ExpiresAt,
		DeviceID: request.DeviceID, InstanceID: request.InstanceID,
	}
}

func cloneBridgeDevice(device *BridgeDevice) BridgeDevice {
	return BridgeDevice{ID: device.ID, Name: device.Name, CreatedAt: device.CreatedAt, LastUsedAt: device.LastUsedAt}
}

func cloneBridgeTransfer(transfer *BridgeTransfer) BridgeTransfer {
	clone := *transfer
	clone.path = ""
	clone.ticketHash = [32]byte{}
	clone.UploadTicket = ""
	if transfer.Preflight != nil {
		preflight := *transfer.Preflight
		clone.Preflight = &preflight
	}
	if transfer.Job != nil {
		job := *transfer.Job
		clone.Job = &job
	}
	return clone
}
