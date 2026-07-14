package service

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/client"
	"github.com/Susurrium/PkuHoleStudio/internal/config"
)

func TestNormalizePKUUsernameAcceptsStudentEmailAndID(t *testing.T) {
	tests := map[string]string{
		" 1234567890 ":               "1234567890",
		"1234567890@stu.pku.edu.cn":  "1234567890",
		"1234567890@STU.PKU.EDU.CN ": "1234567890",
		"teacher@pku.edu.cn":         "teacher",
		"unrelated@example.com":      "unrelated@example.com",
	}
	for input, want := range tests {
		if got := normalizePKUUsername(input); got != want {
			t.Errorf("normalizePKUUsername(%q) = %q, want %q", input, got, want)
		}
	}
}

type treeholeSMSRoundTripper struct {
	calls int
}

func (r *treeholeSMSRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.String() == string(client.SEND_MESSAGE) {
		r.calls++
	}
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"success":true}`)), Request: request}, nil
}

func TestAuthServiceSendsTreeholeStageSMSWithTreeholeEndpoint(t *testing.T) {
	config.SetRuntimeDataDir(t.TempDir())
	t.Cleanup(func() { config.SetRuntimeDataDir("") })
	treeholeClient, err := client.NewClient("test-device")
	if err != nil {
		t.Fatal(err)
	}
	transport := &treeholeSMSRoundTripper{}
	treeholeClient.GetHttpClient().Transport = transport
	status := NewAuthService(treeholeClient, nil).SendSMS(t.Context(), "treehole", "")
	if transport.calls != 1 {
		t.Fatalf("treehole SMS calls = %d, want 1", transport.calls)
	}
	if status.ChallengeStage != "treehole" || status.Challenge != "sms" {
		t.Fatalf("status = %+v", status)
	}
}
