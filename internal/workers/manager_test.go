package workers

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestManagerSpawnSuccessEvents(t *testing.T) {
	m := NewManager(RunnerFunc(func(ctx context.Context, req Request) (Result, error) {
		return Result{Summary: "done"}, nil
	}))

	id, err := m.Spawn(context.Background(), Request{CWD: "/tmp/project", Task: "inspect", Label: "docs"})
	if err != nil {
		t.Fatalf("Spawn error: %v", err)
	}

	wantStates := []State{StateQueued, StateRunning, StateSucceeded}
	for _, want := range wantStates {
		ev := waitEvent(t, m)
		if ev.Status.ID != id {
			t.Fatalf("event id = %q, want %q", ev.Status.ID, id)
		}
		if ev.Status.State != want {
			t.Fatalf("event state = %s, want %s", ev.Status.State, want)
		}
	}

	snap := m.Snapshot()
	if len(snap) != 1 || snap[0].State != StateSucceeded {
		t.Fatalf("Snapshot = %+v, want one succeeded worker", snap)
	}
}

func TestManagerSpawnFailure(t *testing.T) {
	m := NewManager(RunnerFunc(func(ctx context.Context, req Request) (Result, error) {
		return Result{}, errors.New("boom")
	}))

	id, err := m.Spawn(context.Background(), Request{Task: "fail"})
	if err != nil {
		t.Fatalf("Spawn error: %v", err)
	}
	ev := waitState(t, m, id, StateFailed)
	if ev.Status.Error != "boom" {
		t.Fatalf("failure error = %q, want boom", ev.Status.Error)
	}
}

func TestManagerTimeout(t *testing.T) {
	m := NewManager(RunnerFunc(func(ctx context.Context, req Request) (Result, error) {
		<-ctx.Done()
		return Result{}, ctx.Err()
	}))

	id, err := m.Spawn(context.Background(), Request{Task: "slow", Timeout: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("Spawn error: %v", err)
	}
	ev := waitState(t, m, id, StateTimedOut)
	if ev.Status.Error != "worker timed out" {
		t.Fatalf("timeout error = %q, want worker timed out", ev.Status.Error)
	}
}

func TestManagerCancel(t *testing.T) {
	m := NewManager(RunnerFunc(func(ctx context.Context, req Request) (Result, error) {
		<-ctx.Done()
		return Result{}, ctx.Err()
	}))

	id, err := m.Spawn(context.Background(), Request{Task: "stop"})
	if err != nil {
		t.Fatalf("Spawn error: %v", err)
	}
	_ = waitState(t, m, id, StateRunning)
	if !m.Cancel(id) {
		t.Fatal("Cancel returned false, want true")
	}
	ev := waitState(t, m, id, StateCanceled)
	if ev.Status.Error != "worker canceled" {
		t.Fatalf("cancel error = %q, want worker canceled", ev.Status.Error)
	}
	if m.Cancel(id) {
		t.Fatal("Cancel terminal worker returned true, want false")
	}
}

func TestManagerSnapshotActiveThenCompleted(t *testing.T) {
	release := make(chan struct{})
	m := NewManager(RunnerFunc(func(ctx context.Context, req Request) (Result, error) {
		select {
		case <-release:
			return Result{Summary: req.Task}, nil
		case <-ctx.Done():
			return Result{}, ctx.Err()
		}
	}), WithMaxCompleted(2))

	id1, err := m.Spawn(context.Background(), Request{Task: "one", Label: "api"})
	if err != nil {
		t.Fatalf("Spawn one error: %v", err)
	}
	_ = waitState(t, m, id1, StateRunning)

	snap := m.Snapshot()
	if len(snap) != 1 || snap[0].ID != id1 || snap[0].State != StateRunning {
		t.Fatalf("active Snapshot = %+v, want running %s", snap, id1)
	}

	close(release)
	_ = waitState(t, m, id1, StateSucceeded)

	for _, task := range []string{"two", "three"} {
		id, err := m.Spawn(context.Background(), Request{Task: task})
		if err != nil {
			t.Fatalf("Spawn %s error: %v", task, err)
		}
		_ = waitState(t, m, id, StateSucceeded)
	}

	snap = m.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("bounded Snapshot len = %d, want 2: %+v", len(snap), snap)
	}
	if snap[0].ID == id1 || snap[1].ID == id1 {
		t.Fatalf("oldest completed worker should be pruned: %+v", snap)
	}
	if snap[0].State != StateSucceeded || snap[1].State != StateSucceeded {
		t.Fatalf("completed Snapshot states = %+v, want succeeded", snap)
	}
}

func TestManagerRejectsInvalidRequests(t *testing.T) {
	m := NewManager(RunnerFunc(func(ctx context.Context, req Request) (Result, error) {
		return Result{}, nil
	}))
	if _, err := m.Spawn(context.Background(), Request{}); err == nil {
		t.Fatal("Spawn empty task error = nil, want error")
	}

	empty := NewManager(nil)
	if _, err := empty.Spawn(context.Background(), Request{Task: "x"}); err == nil {
		t.Fatal("Spawn without runner error = nil, want error")
	}
}

func waitEvent(t *testing.T, m *Manager) Event {
	t.Helper()
	select {
	case ev := <-m.Events():
		return ev
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for worker event")
		return Event{}
	}
}

func waitState(t *testing.T, m *Manager, id string, state State) Event {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case ev := <-m.Events():
			if ev.Status.ID == id && ev.Status.State == state {
				return ev
			}
		case <-deadline:
			t.Fatalf("timed out waiting for worker %s state %s", id, state)
			return Event{}
		}
	}
}
