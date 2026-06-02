package iomcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/edward-champion/io/internal/ioipc"
)

type fakeControlClient struct {
	spawnReq ioipc.SpawnWorkerRequest
	err      error
}

func (f *fakeControlClient) SpawnWorker(ctx context.Context, req ioipc.SpawnWorkerRequest) (string, error) {
	f.spawnReq = req
	if f.err != nil {
		return "", f.err
	}
	return "w1", nil
}

func (f *fakeControlClient) WorkerStatus(ctx context.Context) ([]ioipc.WorkerStatus, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []ioipc.WorkerStatus{{ID: "w1", State: "running"}}, nil
}

func (f *fakeControlClient) ListProjects(ctx context.Context) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []string{"/tmp/project"}, nil
}

func TestServerInitializeAndToolsList(t *testing.T) {
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n")
	var out bytes.Buffer
	if err := New(&fakeControlClient{}).Serve(context.Background(), in, &out); err != nil {
		t.Fatalf("Serve error: %v", err)
	}
	responses := decodeResponses(t, out.String())
	if len(responses) != 2 {
		t.Fatalf("response count = %d, want 2: %s", len(responses), out.String())
	}
	if responses[0]["result"] == nil {
		t.Fatalf("initialize response missing result: %+v", responses[0])
	}
	tools := responses[1]["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 3 {
		t.Fatalf("tools len = %d, want 3", len(tools))
	}
}

func TestServerSpawnWorkerTool(t *testing.T) {
	client := &fakeControlClient{}
	in := strings.NewReader(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"spawn_worker","arguments":{"cwd":"/tmp/project","task":"inspect","label":"docs"}}}` + "\n")
	var out bytes.Buffer
	if err := New(client).Serve(context.Background(), in, &out); err != nil {
		t.Fatalf("Serve error: %v", err)
	}
	if client.spawnReq.Task != "inspect" || client.spawnReq.Label != "docs" {
		t.Fatalf("spawn request = %+v, want inspect/docs", client.spawnReq)
	}
	resp := decodeResponses(t, out.String())[0]
	result := resp["result"].(map[string]any)
	if result["isError"].(bool) {
		t.Fatalf("spawn_worker returned error: %+v", result)
	}
	if !strings.Contains(result["content"].([]any)[0].(map[string]any)["text"].(string), "w1") {
		t.Fatalf("spawn_worker content missing id: %+v", result)
	}
}

func TestServerToolError(t *testing.T) {
	in := strings.NewReader(`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"worker_status","arguments":{}}}` + "\n")
	var out bytes.Buffer
	if err := New(&fakeControlClient{err: errBoom{}}).Serve(context.Background(), in, &out); err != nil {
		t.Fatalf("Serve error: %v", err)
	}
	resp := decodeResponses(t, out.String())[0]
	result := resp["result"].(map[string]any)
	if !result["isError"].(bool) {
		t.Fatalf("tool error isError = false: %+v", result)
	}
	if !strings.Contains(result["content"].([]any)[0].(map[string]any)["text"].(string), "boom") {
		t.Fatalf("tool error text missing boom: %+v", result)
	}
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }

func decodeResponses(t *testing.T, out string) []map[string]any {
	t.Helper()
	var responses []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		var resp map[string]any
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("decode response %q: %v", line, err)
		}
		responses = append(responses, resp)
	}
	return responses
}
