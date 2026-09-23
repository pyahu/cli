package cloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

// Asks a REAL issuer for a real device code and checks where preferWorkingLoginUI would send a person.
//
// This exists because the first version of that function passed every unit test and still shipped inert:
// the fixtures used the URL I ASSUMED the issuer returns, and the issuer returns a different one. A unit
// test cannot catch a wrong premise about the outside world; only the call can. The failure it missed
// was expensive, and it was invisible from inside the repo.
//
// SKIPPED unless PYAHU_LIVE_ISSUER is set, so it never runs in CI and never depends on a network:
//
//	PYAHU_LIVE_ISSUER=https://zitadel.de.pyahu.cloud \
//	PYAHU_LIVE_CLIENT_ID=<client id> \
//	go test ./internal/cloud -run TestPreferWorkingLoginUIAgainstARealIssuer -v
func TestPreferWorkingLoginUIAgainstARealIssuer(t *testing.T) {
	issuer := os.Getenv("PYAHU_LIVE_ISSUER")
	clientID := os.Getenv("PYAHU_LIVE_CLIENT_ID")
	if issuer == "" || clientID == "" {
		t.Skip("live issuer check not requested")
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("scope", strings.Join(scopes, " "))
	res, err := http.PostForm(issuer+"/oauth/v2/device_authorization", form)
	if err != nil {
		t.Fatalf("device_authorization: %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	var body struct {
		UserCode                string `json:"user_code"`
		VerificationURIComplete string `json:"verification_uri_complete"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.VerificationURIComplete == "" {
		t.Fatal("the issuer returned no verification_uri_complete")
	}

	got := preferWorkingLoginUI(context.Background(), http.DefaultClient, body.VerificationURIComplete)
	t.Logf("issuer returned: %s", body.VerificationURIComplete)
	t.Logf("CLI would open:  %s", got)

	// Where the person lands matters, not whether the string changed: an instance already serving v2
	// needs no rewrite and must still pass.
	landing := got
	if probe, err := http.NewRequest(http.MethodGet, got, nil); err == nil {
		client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}
		if hop, err := client.Do(probe); err == nil {
			defer func() { _ = hop.Body.Close() }()
			if loc := hop.Header.Get("Location"); loc != "" {
				landing = loc
			}
		}
	}
	if strings.Contains(landing, "/ui/login/") {
		t.Fatalf("the person would land in the v1 login UI (%s), where the device is never approved", landing)
	}
}
