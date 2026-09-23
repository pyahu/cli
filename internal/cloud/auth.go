package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// scopes are what the CLI asks for when signing somebody in.
//
// `urn:zitadel:iam:user:metadata` is not optional decoration: it carries which organizations the
// person belongs to, which is what the platform reads to decide whether they may see an environment at
// all. `offline_access` is what makes a refresh token come back, so signing in is a thing somebody does
// once rather than every time a token expires.
var scopes = []string{"openid", "profile", "email", "urn:zitadel:iam:user:metadata", "offline_access"}

// ErrNotSignedIn is returned by every call that needs an identity and finds none. Callers turn it into
// the one instruction that fixes it.
var ErrNotSignedIn = errors.New("not signed in")

// Credentials is what a successful sign-in leaves on disk.
type Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	Issuer       string    `json:"issuer"`
	ClientID     string    `json:"client_id"`
}

func (c Credentials) expired(now time.Time) bool {
	// A minute of margin, so a token does not expire between this check and the request it authorises.
	return !c.ExpiresAt.IsZero() && now.Add(time.Minute).After(c.ExpiresAt)
}

// CredentialsPath is where the signed-in session lives.
func CredentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".pyahu", "credentials.json"), nil
}

// SaveCredentials writes the session with permissions only its owner can read.
//
// The file is created 0600 rather than chmod'ed afterwards: a file that is briefly world-readable is
// world-readable, and on a shared machine "briefly" is enough.
func SaveCredentials(c Credentials) error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	_, err = file.Write(payload)
	return err
}

// LoadCredentials reads the session, or reports that there is none.
func LoadCredentials() (Credentials, error) {
	path, err := CredentialsPath()
	if err != nil {
		return Credentials{}, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Credentials{}, ErrNotSignedIn
	}
	if err != nil {
		return Credentials{}, err
	}
	var c Credentials
	if err := json.Unmarshal(raw, &c); err != nil {
		return Credentials{}, fmt.Errorf("%s is not readable; sign in again: %w", path, err)
	}
	return c, nil
}

// ForgetCredentials removes the session from this machine.
func ForgetCredentials() error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// discovery is the part of the issuer's published configuration this CLI uses.
type discovery struct {
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
}

func fetchDiscovery(ctx context.Context, client *http.Client, issuer string) (discovery, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimSuffix(issuer, "/")+"/.well-known/openid-configuration", nil)
	if err != nil {
		return discovery{}, err
	}
	res, err := client.Do(req)
	if err != nil {
		return discovery{}, fmt.Errorf("reach %s: %w", issuer, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return discovery{}, fmt.Errorf("%s answered %s to its own configuration", issuer, res.Status)
	}
	var d discovery
	if err := json.NewDecoder(res.Body).Decode(&d); err != nil {
		return discovery{}, err
	}
	if d.DeviceAuthorizationEndpoint == "" || d.TokenEndpoint == "" {
		return discovery{}, fmt.Errorf("%s does not offer the device flow", issuer)
	}
	return d, nil
}

// DeviceCode is what the person has to act on to finish signing in.
type DeviceCode struct {
	UserCode        string
	VerificationURI string
	// VerificationURIComplete already carries the code, so a browser that opens it needs no typing.
	VerificationURIComplete string
	interval                time.Duration
	deviceCode              string
	tokenEndpoint           string
	expiresAt               time.Time
}

// StartDeviceLogin asks the issuer for a code the person approves in a browser.
//
// The device flow rather than a loopback redirect, and the reason is where this runs: a loopback flow
// needs a port on the machine running the CLI and a browser on that same machine, which is exactly what
// an SSH session, a container and a CI runner do not have. A code the person types somewhere else works
// in all of them.
func StartDeviceLogin(ctx context.Context, cfg Config, client *http.Client) (*DeviceCode, error) {
	d, err := fetchDiscovery(ctx, client, cfg.Issuer)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("client_id", cfg.ClientID)
	form.Set("scope", strings.Join(scopes, " "))

	res, err := postForm(ctx, client, d.DeviceAuthorizationEndpoint, form)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the issuer refused to start a sign-in: %s", res.Status)
	}

	var body struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		ExpiresIn               int    `json:"expires_in"`
		Interval                int    `json:"interval"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, err
	}
	interval := time.Duration(body.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	expires := time.Now().Add(time.Duration(body.ExpiresIn) * time.Second)
	if body.ExpiresIn == 0 {
		expires = time.Now().Add(10 * time.Minute)
	}
	return &DeviceCode{
		UserCode:                body.UserCode,
		VerificationURI:         preferWorkingLoginUI(ctx, client, body.VerificationURI),
		VerificationURIComplete: preferWorkingLoginUI(ctx, client, body.VerificationURIComplete),
		interval:                interval,
		deviceCode:              body.DeviceCode,
		tokenEndpoint:           d.TokenEndpoint,
		expiresAt:               expires,
	}, nil
}

// legacyZitadelLoginPrefix is the login UI Zitadel redirects a device flow into. It is a PREFIX, not a
// whole path, because the redirect lands on /ui/login/device and from there on /ui/login/login.
const (
	legacyZitadelLoginPrefix = "/ui/login/"
	zitadelV2LoginPrefix     = "/ui/v2/login/"
)

// preferWorkingLoginUI points the person at the approval page that actually completes the sign-in.
//
// Zitadel can be configured to require its v2 login instance-wide, and then its authorize endpoint sends
// browsers to /ui/v2/login while the DEVICE endpoint still redirects into the v1 UI. On such an instance
// the v1 page accepts the password, re-renders itself and never approves the device: the person sees the
// screen blink and stay put, the CLI polls authorization_pending until the code expires, and nothing is
// logged as an error on either side. That cost an evening on 2026-09-23.
//
// The check has to FOLLOW the redirect rather than read the URL. The issuer hands back its OIDC endpoint
// (…/device?user_code=X), not a login path, so there is nothing in the string to match on: which UI the
// person lands in is only visible in the Location header that endpoint answers with. A first version of
// this matched the string and therefore never fired, which is how it shipped still broken.
//
// Conservative by construction: anything unexpected returns what the issuer said, because a sign-in that
// might work beats one that certainly cannot, and this CLI also talks to issuers that are not Zitadel.
// Once an instance sets its v2 base URI the redirect lands on v2 by itself and this becomes a no-op.
func preferWorkingLoginUI(ctx context.Context, client *http.Client, raw string) string {
	if raw == "" {
		return raw
	}

	probe, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(probe, http.MethodGet, raw, nil)
	if err != nil {
		return raw
	}
	// A client of our own, so the caller's redirect policy is never mutated and the hop is not followed:
	// the Location header IS the answer we came for.
	noFollow := &http.Client{
		Transport: client.Transport,
		Jar:       client.Jar,
		Timeout:   client.Timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	res, err := noFollow.Do(req)
	if err != nil {
		return raw
	}
	defer func() { _ = res.Body.Close() }()

	location := res.Header.Get("Location")
	if location == "" || !strings.Contains(location, legacyZitadelLoginPrefix) {
		return raw
	}

	// Resolve against the endpoint we asked, so a relative Location (which is what Zitadel sends) becomes
	// something a browser can be handed.
	base, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	landed, err := base.Parse(location)
	if err != nil {
		return raw
	}
	landed.Path = strings.Replace(landed.Path, legacyZitadelLoginPrefix, zitadelV2LoginPrefix, 1)

	// Only prefer the v2 page once it has answered for itself.
	check, cancelCheck := context.WithTimeout(ctx, 5*time.Second)
	defer cancelCheck()
	verify, err := http.NewRequestWithContext(check, http.MethodGet, landed.String(), nil)
	if err != nil {
		return raw
	}
	got, err := noFollow.Do(verify)
	if err != nil {
		return raw
	}
	defer func() { _ = got.Body.Close() }()
	if got.StatusCode >= 200 && got.StatusCode < 400 {
		return landed.String()
	}
	return raw
}

// Wait polls until the person approves, the code expires, or the context is cancelled.
func (d *DeviceCode) Wait(ctx context.Context, cfg Config, client *http.Client) (Credentials, error) {
	interval := d.interval
	for {
		select {
		case <-ctx.Done():
			return Credentials{}, ctx.Err()
		case <-time.After(interval):
		}
		if time.Now().After(d.expiresAt) {
			return Credentials{}, errors.New("the sign-in code expired; run `pyahu login` again")
		}

		form := url.Values{}
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		form.Set("device_code", d.deviceCode)
		form.Set("client_id", cfg.ClientID)

		creds, retryIn, err := exchange(ctx, client, d.tokenEndpoint, form, cfg)
		switch {
		case err != nil:
			return Credentials{}, err
		case retryIn > 0:
			// `slow_down` means the issuer wants a longer gap, and ignoring it gets the CLI throttled.
			interval = retryIn
		case creds.AccessToken != "":
			return creds, nil
		}
	}
}

// AccessToken returns a usable token, refreshing the stored one when it has expired.
//
// This is the function every command calls, and it is the reason signing in is a once-a-week act rather
// than a daily one.
func AccessToken(ctx context.Context, cfg Config, client *http.Client) (string, error) {
	creds, err := LoadCredentials()
	if err != nil {
		return "", err
	}
	if !creds.expired(time.Now()) {
		return creds.AccessToken, nil
	}
	if creds.RefreshToken == "" {
		return "", ErrNotSignedIn
	}

	d, err := fetchDiscovery(ctx, client, cfg.Issuer)
	if err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", creds.RefreshToken)
	form.Set("client_id", cfg.ClientID)

	refreshed, _, err := exchange(ctx, client, d.TokenEndpoint, form, cfg)
	if err != nil {
		// A refresh token the issuer no longer accepts is a session that ended: somebody signed out
		// elsewhere, or the account changed. Saying "sign in again" is the whole fix.
		return "", ErrNotSignedIn
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = creds.RefreshToken
	}
	if err := SaveCredentials(refreshed); err != nil {
		return "", err
	}
	return refreshed.AccessToken, nil
}

// exchange posts a token request and interprets the three answers that are not an error.
func exchange(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	form url.Values,
	cfg Config,
) (Credentials, time.Duration, error) {
	res, err := postForm(ctx, client, endpoint, form)
	if err != nil {
		return Credentials{}, 0, err
	}
	defer func() { _ = res.Body.Close() }()

	var body struct {
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		ExpiresIn        int    `json:"expires_in"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return Credentials{}, 0, fmt.Errorf("the issuer answered %s with something unreadable", res.Status)
	}

	switch body.Error {
	case "":
	case "authorization_pending":
		return Credentials{}, 0, nil
	case "slow_down":
		return Credentials{}, 10 * time.Second, nil
	case "access_denied":
		return Credentials{}, 0, errors.New("the sign-in was denied")
	case "expired_token":
		return Credentials{}, 0, errors.New("the sign-in code expired; run `pyahu login` again")
	default:
		if body.ErrorDescription != "" {
			return Credentials{}, 0, fmt.Errorf("%s: %s", body.Error, body.ErrorDescription)
		}
		return Credentials{}, 0, errors.New(body.Error)
	}

	if body.AccessToken == "" {
		return Credentials{}, 0, fmt.Errorf("the issuer answered %s without a token", res.Status)
	}
	return Credentials{
		AccessToken:  body.AccessToken,
		RefreshToken: body.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(body.ExpiresIn) * time.Second),
		Issuer:       cfg.Issuer,
		ClientID:     cfg.ClientID,
	}, 0, nil
}

func postForm(ctx context.Context, client *http.Client, endpoint string, form url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return client.Do(req)
}
