package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNordOAuthRejectsExpiredStateAndConsumesIt(t *testing.T) {
	routes := NewNordVPNRoutes()
	routes.pending = map[string]nordPendingLogin{"expired": {expires: time.Now().Add(-time.Minute)}}
	for _, state := range []string{"expired", "missing"} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest("POST", "/api/nordvpn/oauth/complete", strings.NewReader(`{"state":"`+state+`","exchangeToken":"secret"}`))
		routes.OAuthComplete(response, request)
		if response.Code != 400 {
			t.Fatalf("status %d", response.Code)
		}
		if strings.Contains(response.Body.String(), "secret") {
			t.Fatal("exchange token leaked")
		}
	}
	if len(routes.pending) != 0 {
		t.Fatal("expired state was retained")
	}
}
