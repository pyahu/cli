package connect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestPluginsUsesUnfilteredEndpointAndSortsByClass(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.RequestURI() != "/connector-plugins?connectorsOnly=false" {
			t.Fatalf("unexpected request URI: %s", request.URL.RequestURI())
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`[
			{"class":"org.example.ZSink","type":"sink","version":"2"},
			{"class":"org.example.ASource","type":"source","version":"1"}
		]`))
	}))
	t.Cleanup(server.Close)

	plugins, err := New(server.URL + "/").Plugins(context.Background())
	if err != nil {
		t.Fatalf("Plugins returned an error: %v", err)
	}
	if got := []string{plugins[0].Class, plugins[1].Class}; !reflect.DeepEqual(got, []string{"org.example.ASource", "org.example.ZSink"}) {
		t.Fatalf("plugins are not sorted: %v", got)
	}
}

func TestConnectorsMapsStatusSortsNamesAndTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.RequestURI() != "/connectors?expand=status" {
			t.Fatalf("unexpected request URI: %s", request.URL.RequestURI())
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{
			"z-sink":{"status":{"name":"z-sink","type":"sink","connector":{"state":"RUNNING","worker_id":"worker:8083"},"tasks":[{"id":2,"state":"FAILED","worker_id":"worker:8083","trace":"first line\nsecond line"},{"id":0,"state":"RUNNING","worker_id":"worker:8083"}]}},
			"a-source":{"status":{"name":"a-source","type":"source","connector":{"state":"RUNNING","worker_id":"worker:8083"},"tasks":[{"id":0,"state":"RUNNING","worker_id":"worker:8083"}]}}
		}`))
	}))
	t.Cleanup(server.Close)

	connectors, err := New(server.URL).Connectors(context.Background())
	if err != nil {
		t.Fatalf("Connectors returned an error: %v", err)
	}
	if got := []string{connectors[0].Name, connectors[1].Name}; !reflect.DeepEqual(got, []string{"a-source", "z-sink"}) {
		t.Fatalf("connectors are not sorted: %v", got)
	}
	failed := connectors[1]
	if got := []int{failed.Tasks[0].ID, failed.Tasks[1].ID}; !reflect.DeepEqual(got, []int{0, 2}) {
		t.Fatalf("tasks are not sorted: %v", got)
	}
	if failed.Tasks[1].Trace != "first line" {
		t.Fatalf("task trace was not reduced to its first line: %q", failed.Tasks[1].Trace)
	}
	if failed.Type != "sink" || failed.Worker != "worker:8083" {
		t.Fatalf("connector status fields were not mapped: %#v", failed)
	}
}

func TestClientReportsHTTPAndJSONErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       string
	}{
		{name: "HTTP status", statusCode: http.StatusServiceUnavailable, body: `{}`, want: "503 Service Unavailable"},
		{name: "invalid JSON", statusCode: http.StatusOK, body: `{`, want: "unexpected EOF"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.WriteHeader(test.statusCode)
				_, _ = response.Write([]byte(test.body))
			}))
			t.Cleanup(server.Close)

			_, err := New(server.URL).Plugins(context.Background())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestDeleteConnectorIsIdempotent(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodDelete || request.URL.Path != "/connectors/orders-source" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		if requests == 1 {
			response.WriteHeader(http.StatusNoContent)
			return
		}
		response.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	client := New(server.URL)
	if err := client.Delete(context.Background(), "orders-source"); err != nil {
		t.Fatalf("delete existing connector: %v", err)
	}
	if err := client.Delete(context.Background(), "orders-source"); err != nil {
		t.Fatalf("delete missing connector: %v", err)
	}
}

func TestDeleteConnectorReportsUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusConflict)
	}))
	t.Cleanup(server.Close)

	err := New(server.URL).Delete(context.Background(), "orders-source")
	if err == nil || !strings.Contains(err.Error(), "409 Conflict") {
		t.Fatalf("unexpected delete error: %v", err)
	}
}

func TestHealthyRequiresEveryDeclaredTaskToRun(t *testing.T) {
	tests := []struct {
		name       string
		connectors []Connector
		want       bool
	}{
		{name: "no connectors", want: true},
		{name: "running", connectors: []Connector{{Tasks: []Task{{State: "RUNNING"}}}}, want: true},
		{name: "no tasks", connectors: []Connector{{State: "RUNNING"}}, want: false},
		{name: "failed task", connectors: []Connector{{Tasks: []Task{{State: "FAILED"}}}}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Healthy(test.connectors); got != test.want {
				t.Fatalf("Healthy() = %t, want %t", got, test.want)
			}
		})
	}
}
