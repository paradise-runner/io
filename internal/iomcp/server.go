package iomcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/edward-champion/io/internal/ioipc"
)

const protocolVersion = "2025-06-18"

// ControlClient is the parent-control API used by MCP tools.
type ControlClient interface {
	SpawnWorker(context.Context, ioipc.SpawnWorkerRequest) (string, error)
	WorkerStatus(context.Context) ([]ioipc.WorkerStatus, error)
	ListProjects(context.Context) ([]string, error)
}

// Server speaks a small MCP-over-stdio server for io worker tools.
type Server struct {
	client ControlClient
}

// New constructs an MCP stdio server.
func New(client ControlClient) *Server { return &Server{client: client} }

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve reads JSON-RPC messages from in and writes responses to out.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			if err := enc.Encode(errorMessage(nil, -32700, "Parse error")); err != nil {
				return err
			}
			continue
		}
		if len(req.ID) == 0 {
			continue
		}
		resp := s.handle(ctx, req)
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return sc.Err()
}

func (s *Server) handle(ctx context.Context, req rpcRequest) rpcResponse {
	switch req.Method {
	case "initialize":
		return ok(req.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{
				"name":    "io",
				"title":   "io Worker Orchestrator",
				"version": "0.1.0",
			},
		})
	case "ping":
		return ok(req.ID, map[string]any{})
	case "tools/list":
		return ok(req.ID, map[string]any{"tools": toolDefinitions()})
	case "tools/call":
		return s.handleToolCall(ctx, req)
	default:
		return errorMessage(req.ID, -32601, "Method not found: "+req.Method)
	}
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) handleToolCall(ctx context.Context, req rpcRequest) rpcResponse {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorMessage(req.ID, -32602, "Invalid tools/call params")
	}
	switch params.Name {
	case "spawn_worker":
		var args ioipc.SpawnWorkerRequest
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return toolError(req.ID, "spawn_worker arguments must be an object")
		}
		id, err := s.client.SpawnWorker(ctx, args)
		if err != nil {
			return toolError(req.ID, err.Error())
		}
		return toolOK(req.ID, fmt.Sprintf("spawned worker %s", id), map[string]any{"worker_id": id})
	case "worker_status":
		statuses, err := s.client.WorkerStatus(ctx)
		if err != nil {
			return toolError(req.ID, err.Error())
		}
		return toolOK(req.ID, marshalText(statuses), map[string]any{"statuses": statuses})
	case "list_projects":
		projects, err := s.client.ListProjects(ctx)
		if err != nil {
			return toolError(req.ID, err.Error())
		}
		return toolOK(req.ID, marshalText(projects), map[string]any{"projects": projects})
	default:
		return errorMessage(req.ID, -32602, "Unknown tool: "+params.Name)
	}
}

func toolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name":        "spawn_worker",
			"title":       "Spawn Worker",
			"description": "Start a background io worker for a bounded task.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"cwd":   map[string]any{"type": "string", "description": "Working directory for the worker."},
					"task":  map[string]any{"type": "string", "description": "Task for the worker to perform."},
					"label": map[string]any{"type": "string", "description": "Short label shown in io's worker strip."},
				},
				"required": []string{"task"},
			},
		},
		{
			"name":        "worker_status",
			"title":       "Worker Status",
			"description": "List active and recently completed io workers.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "list_projects",
			"title":       "List Projects",
			"description": "List project directories known to this io session.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}
}

func ok(id json.RawMessage, result any) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func errorMessage(id json.RawMessage, code int, message string) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}}
}

func toolOK(id json.RawMessage, text string, structured map[string]any) rpcResponse {
	return ok(id, map[string]any{
		"content":           []map[string]string{{"type": "text", "text": text}},
		"isError":           false,
		"structuredContent": structured,
	})
}

func toolError(id json.RawMessage, text string) rpcResponse {
	return ok(id, map[string]any{
		"content": []map[string]string{{"type": "text", "text": text}},
		"isError": true,
	})
}

func marshalText(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}
