package ioipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

// Client calls the parent app control socket.
type Client struct {
	SocketPath string
	Timeout    time.Duration
}

// SpawnWorker calls spawn_worker.
func (c Client) SpawnWorker(ctx context.Context, req SpawnWorkerRequest) (string, error) {
	resp, err := c.call(ctx, Request{Version: ProtocolVersion, Operation: OpSpawnWorker, SpawnWorker: &req})
	if err != nil {
		return "", err
	}
	return resp.WorkerID, nil
}

// WorkerStatus calls worker_status.
func (c Client) WorkerStatus(ctx context.Context) ([]WorkerStatus, error) {
	resp, err := c.call(ctx, Request{Version: ProtocolVersion, Operation: OpWorkerStatus})
	if err != nil {
		return nil, err
	}
	return resp.Statuses, nil
}

// ListProjects calls list_projects.
func (c Client) ListProjects(ctx context.Context) ([]string, error) {
	resp, err := c.call(ctx, Request{Version: ProtocolVersion, Operation: OpListProjects})
	if err != nil {
		return nil, err
	}
	return resp.Projects, nil
}

func (c Client) call(ctx context.Context, req Request) (Response, error) {
	if c.SocketPath == "" {
		return Response{}, errors.New("ioipc socket path is required")
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", c.SocketPath)
	if err != nil {
		return Response{}, fmt.Errorf("ioipc connect %s: %w", c.SocketPath, err)
	}
	defer conn.Close()
	deadline := time.Now().Add(timeout)
	_ = conn.SetDeadline(deadline)
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return Response{}, fmt.Errorf("ioipc write: %w", err)
	}
	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return Response{}, fmt.Errorf("ioipc read: %w", err)
	}
	if resp.Version != ProtocolVersion {
		return Response{}, fmt.Errorf("unsupported ioipc response version %d", resp.Version)
	}
	if !resp.OK {
		return Response{}, errors.New(resp.Error)
	}
	return resp, nil
}
