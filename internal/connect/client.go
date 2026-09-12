// Package connect talks to a running Kafka Connect worker through its REST API
// on the host port, for the parts of the CLI that report what the worker is
// actually doing rather than what the stack file declares.
package connect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

// Plugin is one entry of GET /connector-plugins.
type Plugin struct {
	Class   string `json:"class"`
	Type    string `json:"type,omitempty"`
	Version string `json:"version,omitempty"`
}

// Connector is one entry of GET /connectors?expand=status.
type Connector struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Worker string `json:"worker,omitempty"`
	Type   string `json:"type,omitempty"`
	Tasks  []Task `json:"tasks"`
}

type Task struct {
	ID     int    `json:"id"`
	State  string `json:"state"`
	Worker string `json:"worker,omitempty"`
	Trace  string `json:"trace,omitempty"`
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Plugins lists every plugin the worker loaded, transforms and converters
// included; connectorsOnly=false is what makes an SMT visible.
func (c *Client) Plugins(ctx context.Context) ([]Plugin, error) {
	var plugins []Plugin
	if err := c.get(ctx, "/connector-plugins?connectorsOnly=false", &plugins); err != nil {
		return nil, err
	}
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].Class < plugins[j].Class })
	return plugins, nil
}

// Connectors returns every registered connector with its state and the state of
// each of its tasks, sorted by name.
func (c *Client) Connectors(ctx context.Context) ([]Connector, error) {
	var raw map[string]struct {
		Status struct {
			Name      string `json:"name"`
			Type      string `json:"type"`
			Connector struct {
				State      string `json:"state"`
				WorkerID   string `json:"worker_id"`
				TraceValue string `json:"trace"`
			} `json:"connector"`
			Tasks []struct {
				ID       int    `json:"id"`
				State    string `json:"state"`
				WorkerID string `json:"worker_id"`
				Trace    string `json:"trace"`
			} `json:"tasks"`
		} `json:"status"`
	}
	if err := c.get(ctx, "/connectors?expand=status", &raw); err != nil {
		return nil, err
	}
	connectors := make([]Connector, 0, len(raw))
	for name, entry := range raw {
		connector := Connector{
			Name:   name,
			State:  entry.Status.Connector.State,
			Worker: entry.Status.Connector.WorkerID,
			Type:   entry.Status.Type,
			Tasks:  make([]Task, 0, len(entry.Status.Tasks)),
		}
		for _, task := range entry.Status.Tasks {
			connector.Tasks = append(connector.Tasks, Task{ID: task.ID, State: task.State, Worker: task.WorkerID, Trace: firstLine(task.Trace)})
		}
		sort.Slice(connector.Tasks, func(i, j int) bool { return connector.Tasks[i].ID < connector.Tasks[j].ID })
		connectors = append(connectors, connector)
	}
	sort.Slice(connectors, func(i, j int) bool { return connectors[i].Name < connectors[j].Name })
	return connectors, nil
}

// Delete removes a registered connector. A missing connector is already in the
// desired state, which keeps reconciliation idempotent.
func (c *Client) Delete(ctx context.Context, name string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/connectors/"+url.PathEscape(name), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call Kafka Connect at %s: %w", c.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return fmt.Errorf("delete Kafka Connect connector %s: endpoint returned %s", name, resp.Status)
}

// Healthy reports whether every task is RUNNING. A connector with no tasks is
// not healthy: the connector row can stay RUNNING while its only task died, and
// that silent stop is exactly what connector-level alerting misses.
func Healthy(connectors []Connector) bool {
	for _, connector := range connectors {
		if len(connector.Tasks) == 0 {
			return false
		}
		for _, task := range connector.Tasks {
			if task.State != "RUNNING" {
				return false
			}
		}
	}
	return true
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call Kafka Connect at %s: %w", c.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("call Kafka Connect %s: endpoint returned %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func firstLine(trace string) string {
	trace = strings.TrimSpace(trace)
	if index := strings.IndexByte(trace, '\n'); index >= 0 {
		return trace[:index]
	}
	return trace
}
