package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"smith/internal/source/model"
)

func TestUnresolvedControllerProcessesInitialUnresolvedStates(t *testing.T) {
	src := &fakeStateSource{
		listStatesData: []StateSnapshot{
			{LoopID: "loop-a", State: model.LoopStateSynced, Revision: 1},
			{LoopID: "loop-b", State: model.LoopStateUnresolved, Revision: 2},
		},
		watchChans: []chan StateSnapshot{make(chan StateSnapshot)},
	}

	handler := &recordingHandler{events: make(chan UnresolvedEvent, 1)}
	controller := NewUnresolvedController(src, handler, model.LoopPolicy{})
	controller.sleep = noSleep

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = controller.Run(ctx) }()

	select {
	case event := <-handler.events:
		if event.LoopID != "loop-b" || event.Revision != 2 {
			t.Fatalf("unexpected event: %+v", event)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for initial unresolved event")
	}

	cancel()
}

func TestUnresolvedControllerProcessesWatchEvents(t *testing.T) {
	watchCh := make(chan StateSnapshot, 2)
	src := &fakeStateSource{
		listStatesData: []StateSnapshot{},
		watchChans:     []chan StateSnapshot{watchCh},
	}
	handler := &recordingHandler{events: make(chan UnresolvedEvent, 2)}
	controller := NewUnresolvedController(src, handler, model.LoopPolicy{})
	controller.sleep = noSleep

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = controller.Run(ctx) }()

	watchCh <- StateSnapshot{LoopID: "loop-ignore", State: model.LoopStateRunning, Revision: 1}
	watchCh <- StateSnapshot{LoopID: "loop-watch", State: model.LoopStateUnresolved, Revision: 7}

	select {
	case event := <-handler.events:
		if event.LoopID != "loop-watch" || event.Revision != 7 {
			t.Fatalf("unexpected event: %+v", event)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for watch unresolved event")
	}

	cancel()
}

func TestUnresolvedControllerRestartsWatchWhenChannelCloses(t *testing.T) {
	watchOne := make(chan StateSnapshot)
	close(watchOne)
	watchTwo := make(chan StateSnapshot, 1)

	src := &fakeStateSource{
		listStatesData: []StateSnapshot{},
		watchChans:     []chan StateSnapshot{watchOne, watchTwo},
	}
	handler := &recordingHandler{events: make(chan UnresolvedEvent, 1)}
	controller := NewUnresolvedController(src, handler, model.LoopPolicy{})
	controller.sleep = noSleep

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = controller.Run(ctx) }()

	watchTwo <- StateSnapshot{LoopID: "loop-restart", State: model.LoopStateUnresolved, Revision: 9}

	select {
	case event := <-handler.events:
		if event.LoopID != "loop-restart" || event.Revision != 9 {
			t.Fatalf("unexpected event: %+v", event)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for event after watch restart")
	}

	cancel()

	if got := src.watchCalls(); got < 2 {
		t.Fatalf("expected watch to be restarted at least once, got %d calls", got)
	}
}

func TestUnresolvedControllerContinuesWhenHandlerFails(t *testing.T) {
	src := &fakeStateSource{
		listStatesData: []StateSnapshot{
			{LoopID: "loop-1", State: model.LoopStateUnresolved, Revision: 1},
			{LoopID: "loop-2", State: model.LoopStateUnresolved, Revision: 2},
		},
		watchChans: []chan StateSnapshot{make(chan StateSnapshot)},
	}
	handler := &recordingHandler{
		events: make(chan UnresolvedEvent, 2),
		results: []error{
			errors.New("first-failure"),
			nil,
		},
	}
	controller := NewUnresolvedController(src, handler, model.LoopPolicy{})
	controller.sleep = noSleep

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = controller.Run(ctx) }()

	for i := 0; i < 2; i++ {
		select {
		case <-handler.events:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for unresolved event")
		}
	}

	cancel()
}

type fakeStateSource struct {
	listStatesData []StateSnapshot
	listStatesErr  error
	watchChans     []chan StateSnapshot
	mu             sync.Mutex
	watchIdx       int
}

func (f *fakeStateSource) ListStates(_ context.Context) ([]StateSnapshot, error) {
	if f.listStatesErr != nil {
		return nil, f.listStatesErr
	}
	out := make([]StateSnapshot, len(f.listStatesData))
	copy(out, f.listStatesData)
	return out, nil
}

func (f *fakeStateSource) WatchStates(_ context.Context) <-chan StateSnapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.watchIdx >= len(f.watchChans) {
		ch := make(chan StateSnapshot)
		close(ch)
		return ch
	}
	ch := f.watchChans[f.watchIdx]
	f.watchIdx++
	return ch
}

func (f *fakeStateSource) watchCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.watchIdx
}

type recordingHandler struct {
	events  chan UnresolvedEvent
	results []error
	mu      sync.Mutex
}

func (r *recordingHandler) Handle(_ context.Context, event UnresolvedEvent) error {
	r.events <- event

	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.results) == 0 {
		return nil
	}
	err := r.results[0]
	r.results = r.results[1:]
	return err
}
