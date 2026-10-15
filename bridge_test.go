package zammadbridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fake3CX struct{}

func (fake3CX) AuthenticateRetry(time.Duration) error  { return nil }
func (fake3CX) FetchCalls() ([]CallInformation, error) { return nil, nil }
func (fake3CX) IsExtension(string) bool                { return true }

// newTestBridge returns a bridge with 3-digit extensions and 5-digit trunks,
// and a pointer to the directions of the events Zammad received.
func newTestBridge(t *testing.T, logOutbound bool) (*ZammadBridge, *[]string) {
	t.Helper()
	var directions []string
	zammad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload ZammadApiRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		directions = append(directions, payload.Direction)
	}))
	t.Cleanup(zammad.Close)

	config := new(Config)
	config.Phone3CX.ExtensionDigits = 3
	config.Phone3CX.TrunkDigits = 5
	config.Zammad.Endpoint = zammad.URL
	config.Zammad.LogOutboundCalls = logOutbound

	return &ZammadBridge{
		Config:       config,
		Client3CX:    fake3CX{},
		ongoingCalls: map[json.Number]CallInformation{},
	}, &directions
}

func outboundCall() *CallInformation {
	return &CallInformation{ID: "1", Status: "Routing", CallerNumber: "150", CalleeNumber: "10007", CalleeName: "0123456789"}
}

func inboundCall() *CallInformation {
	return &CallInformation{ID: "2", Status: "Routing", CallerNumber: "10007", CallerName: "0123456789", CalleeNumber: "150"}
}

func TestProcessCallSendsOutboundCallsWhenEnabled(t *testing.T) {
	bridge, directions := newTestBridge(t, true)

	if err := bridge.ProcessCall(outboundCall()); err != nil {
		t.Fatal(err)
	}

	if len(*directions) != 1 || (*directions)[0] != "out" {
		t.Errorf("expected one outbound event, got %v", *directions)
	}
}

func TestProcessCallIgnoresOutboundCallsWhenDisabled(t *testing.T) {
	bridge, directions := newTestBridge(t, false)

	if err := bridge.ProcessCall(outboundCall()); err != nil {
		t.Fatal(err)
	}

	if len(*directions) != 0 {
		t.Errorf("expected no events, got %v", *directions)
	}
	if len(bridge.ongoingCalls) != 0 {
		t.Error("expected the outbound call not to be tracked, so no hangup is sent later")
	}
}

func TestProcessCallSendsInboundCallsWhenOutboundDisabled(t *testing.T) {
	bridge, directions := newTestBridge(t, false)

	if err := bridge.ProcessCall(inboundCall()); err != nil {
		t.Fatal(err)
	}

	if len(*directions) != 1 || (*directions)[0] != "in" {
		t.Errorf("expected one inbound event, got %v", *directions)
	}
}
