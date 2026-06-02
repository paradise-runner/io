package ioipc

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeHandler struct {
	spawnReq SpawnWorkerRequest
}

func (f *fakeHandler) SpawnWorker(ctx context.Context, req SpawnWorkerRequest) (string, error) {
	f.spawnReq = req
	return "w1", nil
}

func (f *fakeHandler) WorkerStatus(ctx context.Context) ([]WorkerStatus, error) {
	return []WorkerStatus{{ID: "w1", CWD: "/tmp/project", Label: "project", State: "running"}}, nil
}

func (f *fakeHandler) ListProjects(ctx context.Context) ([]string, error) {
	return []string{"/tmp/project"}, nil
}

func TestClientServerRoundTrip(t *testing.T) {
	handler := &fakeHandler{}
	socket := t.TempDir() + "/io.sock"
	server, err := Listen(socket, handler)
	if err != nil {
		t.Fatalf("Listen error: %v", err)
	}
	defer server.Close()
	go func() { _ = server.Serve() }()

	client := Client{SocketPath: socket, Timeout: time.Second}
	id, err := client.SpawnWorker(context.Background(), SpawnWorkerRequest{CWD: "/tmp/project", Task: "inspect", Label: "docs"})
	if err != nil {
		t.Fatalf("SpawnWorker error: %v", err)
	}
	if id != "w1" {
		t.Fatalf("worker id = %q, want w1", id)
	}
	if handler.spawnReq.Task != "inspect" || handler.spawnReq.Label != "docs" {
		t.Fatalf("spawn request = %+v, want task inspect label docs", handler.spawnReq)
	}

	statuses, err := client.WorkerStatus(context.Background())
	if err != nil {
		t.Fatalf("WorkerStatus error: %v", err)
	}
	if len(statuses) != 1 || statuses[0].State != "running" {
		t.Fatalf("statuses = %+v, want one running worker", statuses)
	}

	projects, err := client.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects error: %v", err)
	}
	if len(projects) != 1 || projects[0] != "/tmp/project" {
		t.Fatalf("projects = %v, want [/tmp/project]", projects)
	}
}

func TestClientSocketFailure(t *testing.T) {
	client := Client{SocketPath: t.TempDir() + "/missing.sock", Timeout: 10 * time.Millisecond}
	_, err := client.WorkerStatus(context.Background())
	if err == nil {
		t.Fatal("WorkerStatus error = nil, want socket failure")
	}
	if !strings.Contains(err.Error(), "ioipc connect") {
		t.Fatalf("socket failure error = %q, want ioipc connect", err.Error())
	}
}

func TestServerRejectsUnknownOperation(t *testing.T) {
	server := &Server{handler: &fakeHandler{}}
	resp := server.dispatch(context.Background(), Request{Version: ProtocolVersion, Operation: "exec"})
	if resp.OK {
		t.Fatalf("unknown operation response OK, want error: %+v", resp)
	}
	if !strings.Contains(resp.Error, "unknown ioipc operation") {
		t.Fatalf("unknown operation error = %q", resp.Error)
	}
}
