package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestTrustedBridgePairsPersistsAndAcceptsSignedTransfer(t *testing.T) {
	dataDir := t.TempDir()
	manager := NewBridgeManager(dataDir, bridgeArchiveStub{}, nil)
	privateKey, publicSPKI := testBridgeDeviceKey(t)
	request, err := manager.CreateDeviceRequest("Firefox Toolkit", publicSPKI)
	if err != nil || request.Status != "pending" || len(request.VerificationCode) != 6 {
		t.Fatalf("device request = %+v, %v", request, err)
	}
	approved, err := manager.ApproveDeviceRequest(request.Token)
	if err != nil || approved.Status != "approved" || approved.DeviceID == "" || approved.InstanceID == "" {
		t.Fatalf("approved request = %+v, %v", approved, err)
	}

	reloaded := NewBridgeManager(dataDir, bridgeArchiveStub{}, nil)
	devices := reloaded.Devices()
	if len(devices) != 1 || devices[0].ID != approved.DeviceID || devices[0].Name != "Firefox Toolkit" {
		t.Fatalf("reloaded devices = %+v", devices)
	}
	challenge, err := reloaded.CreateChallenge(approved.DeviceID)
	if err != nil || challenge.InstanceID != approved.InstanceID {
		t.Fatalf("challenge = %+v, %v", challenge, err)
	}
	content := []byte("archive")
	digest := sha256.Sum256(content)
	input := BridgeTransferRequest{
		DeviceID: approved.DeviceID, InstanceID: approved.InstanceID, Challenge: challenge.Token,
		Filename: "sample.treehole.zip", Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:]),
	}
	messageDigest := sha256.Sum256(bridgeTransferMessage(input.DeviceID, input.InstanceID, input.Challenge, input.Filename, input.Size, input.SHA256))
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, messageDigest[:])
	if err != nil {
		t.Fatal(err)
	}
	rawSignature := make([]byte, 64)
	r.FillBytes(rawSignature[:32])
	s.FillBytes(rawSignature[32:])
	input.Signature = base64.StdEncoding.EncodeToString(rawSignature)
	transfer, err := reloaded.CreateTrustedTransfer(input)
	if err != nil || transfer.UploadTicket == "" || transfer.Status != "waiting_upload" {
		t.Fatalf("transfer = %+v, %v", transfer, err)
	}
	listed := reloaded.TrustedTransfers()
	if len(listed) != 1 || listed[0].UploadTicket != "" {
		t.Fatalf("public transfer list leaked ticket: %+v", listed)
	}
	uploaded, err := reloaded.UploadTrusted(context.Background(), transfer.ID, transfer.UploadTicket, strings.NewReader(string(content)))
	if err != nil || uploaded.Status != "awaiting_confirmation" || uploaded.Preflight == nil || uploaded.Preflight.Counts.ValidItems != 1 {
		t.Fatalf("uploaded transfer = %+v, %v", uploaded, err)
	}
	revokeChallenge, err := reloaded.CreateChallenge(approved.DeviceID)
	if err != nil {
		t.Fatal(err)
	}
	revokeDigest := sha256.Sum256(bridgeDeviceRevokeMessage(approved.DeviceID, approved.InstanceID, revokeChallenge.Token))
	revokeR, revokeS, err := ecdsa.Sign(rand.Reader, privateKey, revokeDigest[:])
	if err != nil {
		t.Fatal(err)
	}
	revokeSignature := make([]byte, 64)
	revokeR.FillBytes(revokeSignature[:32])
	revokeS.FillBytes(revokeSignature[32:])
	if err := reloaded.RevokeTrustedDevice(BridgeDeviceRevokeRequest{
		DeviceID: approved.DeviceID, InstanceID: approved.InstanceID, Challenge: revokeChallenge.Token,
		Signature: base64.StdEncoding.EncodeToString(revokeSignature),
	}); err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Devices()) != 0 || len(reloaded.TrustedTransfers()) != 0 {
		t.Fatal("revoking a device retained its trust or staged transfers")
	}
}

func TestLegacyBridgeUsesStateSpecificLifetimes(t *testing.T) {
	now := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	manager := NewBridgeManager(t.TempDir(), bridgeArchiveStub{}, nil)
	manager.now = func() time.Time { return now }
	pairing, err := manager.Create("8080:")
	if err != nil {
		t.Fatal(err)
	}
	if got := pairing.ExpiresAt.Sub(now); got != bridgeWaitingLifetime {
		t.Fatalf("waiting lifetime = %v", got)
	}
	uploaded, err := manager.Upload(context.Background(), pairing.Token, "archive.treehole.zip", strings.NewReader("archive"))
	if err != nil {
		t.Fatal(err)
	}
	if got := uploaded.ExpiresAt.Sub(now); got != bridgeConfirmationLifetime {
		t.Fatalf("confirmation lifetime = %v", got)
	}
	now = now.Add(bridgeConfirmationLifetime + time.Second)
	if _, ok := manager.Get(pairing.Token); ok {
		t.Fatal("expired awaiting-confirmation pairing was retained")
	}
}

func TestTrustedBridgeLimitsOnlyPendingRequestsAndActiveTransfers(t *testing.T) {
	_, publicSPKI := testBridgeDeviceKey(t)
	limited := NewBridgeManager(t.TempDir(), bridgeArchiveStub{}, nil)
	for index := 0; index < maxPendingDeviceRequests; index++ {
		if _, err := limited.CreateDeviceRequest("Toolkit", publicSPKI); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := limited.CreateDeviceRequest("Toolkit", publicSPKI); err == nil {
		t.Fatal("pending device request capacity was not enforced")
	}

	manager := NewBridgeManager(t.TempDir(), bridgeArchiveStub{}, nil)
	for index := 0; index < maxPendingDeviceRequests; index++ {
		request, err := manager.CreateDeviceRequest("Toolkit", publicSPKI)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := manager.ApproveDeviceRequest(request.Token); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := manager.CreateDeviceRequest("Toolkit", publicSPKI); err != nil {
		t.Fatalf("approved requests still occupied pending request capacity: %v", err)
	}

	manager.mu.Lock()
	for index := 0; index < maxActiveBridgeTransfers; index++ {
		id := fmt.Sprintf("queued-%d", index)
		manager.transfers[id] = &BridgeTransfer{ID: id, Status: "queued", CreatedAt: manager.now(), ExpiresAt: manager.now().Add(time.Hour)}
	}
	manager.mu.Unlock()
	approved := manager.Devices()[0]
	privateKey, signedPublicSPKI := testBridgeDeviceKey(t)
	signedRequest, err := manager.CreateDeviceRequest("Signed Toolkit", signedPublicSPKI)
	if err != nil {
		t.Fatal(err)
	}
	signedApproval, err := manager.ApproveDeviceRequest(signedRequest.Token)
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := manager.CreateChallenge(signedApproval.DeviceID)
	if err != nil {
		t.Fatal(err)
	}
	input := signedBridgeTransferRequest(t, privateKey, signedApproval, challenge, []byte("archive"))
	if _, err := manager.CreateTrustedTransfer(input); err != nil {
		t.Fatalf("queued transfers still occupied active transfer capacity (first device %s): %v", approved.ID, err)
	}
}

func signedBridgeTransferRequest(t *testing.T, privateKey *ecdsa.PrivateKey, approved BridgeDeviceRequest, challenge BridgeChallenge, content []byte) BridgeTransferRequest {
	t.Helper()
	digest := sha256.Sum256(content)
	input := BridgeTransferRequest{
		DeviceID: approved.DeviceID, InstanceID: approved.InstanceID, Challenge: challenge.Token,
		Filename: "sample.treehole.zip", Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:]),
	}
	messageDigest := sha256.Sum256(bridgeTransferMessage(input.DeviceID, input.InstanceID, input.Challenge, input.Filename, input.Size, input.SHA256))
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, messageDigest[:])
	if err != nil {
		t.Fatal(err)
	}
	rawSignature := make([]byte, 64)
	r.FillBytes(rawSignature[:32])
	s.FillBytes(rawSignature[32:])
	input.Signature = base64.StdEncoding.EncodeToString(rawSignature)
	return input
}

func testBridgeDeviceKey(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return privateKey, base64.StdEncoding.EncodeToString(encoded)
}
