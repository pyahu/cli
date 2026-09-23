package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Environment is one Pyahu Cloud environment this person can reach with `kubectl`.
type Environment struct {
	OrganizationID   string `json:"organizationId"`
	OrganizationName string `json:"organizationName"`
	TenantID         string `json:"tenantId"`
	Name             string `json:"name"`
	Server           string `json:"server"`
	Namespace        string `json:"namespace"`
	ContextName      string `json:"contextName"`
	Level            string `json:"level"`
	MinCLIVersion    string `json:"minCliVersion"`
}

// AccessToken is the short-lived credential `kubectl` presents to a cluster.
type ClusterCredential struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	Server    string    `json:"server"`
	Namespace string    `json:"namespace"`
}

// Client talks to the console's cluster-access endpoints.
type Client struct {
	Config Config
	HTTP   *http.Client
}

// NewClient builds a client with a timeout that suits a command somebody is waiting on.
func NewClient(cfg Config) *Client {
	return &Client{Config: cfg, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// Environments lists what this person can reach.
func (c *Client) Environments(ctx context.Context, token string) ([]Environment, error) {
	res, err := c.get(ctx, "/platform/api/cluster-access/environments", token)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if err := expectOK(res); err != nil {
		return nil, err
	}
	var body struct {
		Environments []Environment `json:"environments"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.Environments, nil
}

// Credential exchanges the signed-in session for a credential scoped to one environment.
func (c *Client) Credential(ctx context.Context, token, organizationID, tenantID string) (ClusterCredential, error) {
	payload, err := json.Marshal(map[string]string{
		"organizationId": organizationID,
		"tenantId":       tenantID,
	})
	if err != nil {
		return ClusterCredential{}, err
	}
	res, err := c.post(ctx, "/platform/api/cluster-access/token", token, payload)
	if err != nil {
		return ClusterCredential{}, err
	}
	defer func() { _ = res.Body.Close() }()
	if err := expectOK(res); err != nil {
		return ClusterCredential{}, err
	}
	var credential ClusterCredential
	if err := json.NewDecoder(res.Body).Decode(&credential); err != nil {
		return ClusterCredential{}, err
	}
	return credential, nil
}

func (c *Client) get(ctx context.Context, path, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url(path), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return c.HTTP.Do(req)
}

func (c *Client) post(ctx context.Context, path, token string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url(path), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return c.HTTP.Do(req)
}

func (c *Client) url(path string) string {
	return strings.TrimSuffix(c.Config.ConsoleURL, "/") + path
}

// expectOK turns a non-2xx into an error carrying what the server actually said.
//
// The server's sentence is kept rather than replaced, because the three refusals it distinguishes
// ("this cluster cannot serve it", "nobody turned it on", "your role does not include it") send a
// person to three different places, and one generic message would send them to the wrong one twice.
func expectOK(res *http.Response) error {
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	detail, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
	message := strings.TrimSpace(string(detail))
	if res.StatusCode == http.StatusUnauthorized {
		return ErrNotSignedIn
	}
	if message == "" {
		return fmt.Errorf("the console answered %s", res.Status)
	}
	return fmt.Errorf("%s", message)
}
