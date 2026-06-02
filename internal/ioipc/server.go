package ioipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// Handler is implemented by the parent app process.
type Handler interface {
	SpawnWorker(context.Context, SpawnWorkerRequest) (string, error)
	WorkerStatus(context.Context) ([]WorkerStatus, error)
	ListProjects(context.Context) ([]string, error)
}

// Server accepts local control requests over a Unix-domain socket.
type Server struct {
	path    string
	ln      net.Listener
	handler Handler
	once    sync.Once
}

// Listen creates a Server bound to path.
func Listen(path string, handler Handler) (*Server, error) {
	if handler == nil {
		return nil, errors.New("ioipc handler is required")
	}
	if path == "" {
		return nil, errors.New("ioipc socket path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	return &Server{path: path, ln: ln, handler: handler}, nil
}

// Serve accepts connections until Close is called.
func (s *Server) Serve() error {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return err
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		writeResponse(conn, Response{Version: ProtocolVersion, OK: false, Error: "invalid request: " + err.Error()})
		return
	}
	resp := s.dispatch(context.Background(), req)
	writeResponse(conn, resp)
}

func (s *Server) dispatch(ctx context.Context, req Request) Response {
	if req.Version != ProtocolVersion {
		return errorResponse(fmt.Sprintf("unsupported ioipc version %d", req.Version))
	}
	switch req.Operation {
	case OpSpawnWorker:
		if req.SpawnWorker == nil {
			return errorResponse("spawn_worker payload is required")
		}
		id, err := s.handler.SpawnWorker(ctx, *req.SpawnWorker)
		if err != nil {
			return errorResponse(err.Error())
		}
		return Response{Version: ProtocolVersion, OK: true, WorkerID: id}
	case OpWorkerStatus:
		statuses, err := s.handler.WorkerStatus(ctx)
		if err != nil {
			return errorResponse(err.Error())
		}
		return Response{Version: ProtocolVersion, OK: true, Statuses: statuses}
	case OpListProjects:
		projects, err := s.handler.ListProjects(ctx)
		if err != nil {
			return errorResponse(err.Error())
		}
		return Response{Version: ProtocolVersion, OK: true, Projects: projects}
	default:
		return errorResponse("unknown ioipc operation: " + req.Operation)
	}
}

func errorResponse(msg string) Response {
	return Response{Version: ProtocolVersion, OK: false, Error: msg}
}

func writeResponse(conn net.Conn, resp Response) {
	_ = json.NewEncoder(conn).Encode(resp)
}

// Close stops the server and removes the socket.
func (s *Server) Close() error {
	var err error
	s.once.Do(func() {
		err = s.ln.Close()
		_ = os.Remove(s.path)
	})
	return err
}
