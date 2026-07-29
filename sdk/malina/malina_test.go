package malina

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ardanlabs/malina/sdk/malina/model"
)

type fakeBackend struct {
	entered chan struct{}
	release chan struct{}
	err     error
	result  model.GeneratedImage
	mu      sync.Mutex
	running int
	max     int
	unloads int
}

func (f *fakeBackend) Generate(model.GenerateParams) (model.GeneratedImage, error) {
	f.mu.Lock()
	f.running++
	f.max = max(f.max, f.running)
	f.mu.Unlock()
	f.entered <- struct{}{}
	<-f.release
	f.mu.Lock()
	f.running--
	f.mu.Unlock()
	result := f.result
	if result.PNG == nil {
		result.PNG = []byte("png")
	}
	return result, f.err
}
func (f *fakeBackend) Unload() error         { f.mu.Lock(); defer f.mu.Unlock(); f.unloads++; return nil }
func (f *fakeBackend) Config() model.Config  { return model.Config{} }
func (f *fakeBackend) Info() model.ModelInfo { return model.ModelInfo{} }

func newTestMalina(t *testing.T, f *fakeBackend, depth int) *Malina {
	t.Helper()
	initState.Lock()
	oldDone := initState.done
	oldPath := initState.path
	initState.done = true
	initState.path = "test"
	initState.Unlock()
	old := newBackend
	newBackend = func(context.Context, model.Config) (backend, error) { return f, nil }
	t.Cleanup(func() {
		newBackend = old
		initState.Lock()
		initState.done = oldDone
		initState.path = oldPath
		initState.Unlock()
	})
	m, err := New(model.WithModelPath("fake"), model.WithQueueDepth(depth), model.WithAdmissionTimeout(time.Minute))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

func testParams() model.GenerateParams {
	return model.GenerateParams{Prompt: "x", Width: 64, Height: 64, Steps: 1, CFGScale: 1}
}

func TestSystemInfoBeforeInit(t *testing.T) {
	initState.Lock()
	oldDone := initState.done
	initState.done = false
	initState.Unlock()
	t.Cleanup(func() {
		initState.Lock()
		initState.done = oldDone
		initState.Unlock()
	})

	if _, err := SystemInfo(); err == nil {
		t.Fatal("SystemInfo: got nil error, want error")
	}
}

func TestGenerateSerializesAndUnload(t *testing.T) {
	f := &fakeBackend{entered: make(chan struct{}, 2), release: make(chan struct{}, 2)}
	m := newTestMalina(t, f, 2)
	results := make(chan error, 2)
	go func() { _, err := m.Generate(context.Background(), testParams()); results <- err }()
	<-f.entered
	go func() { _, err := m.Generate(context.Background(), testParams()); results <- err }()
	select {
	case <-f.entered:
		t.Fatal("second generation entered concurrently")
	default:
	}
	f.release <- struct{}{}
	<-f.entered
	f.release <- struct{}{}
	if err := <-results; err != nil {
		t.Fatal(err)
	}
	if err := <-results; err != nil {
		t.Fatal(err)
	}
	if err := m.Unload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.Unload(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.max != 1 || f.unloads != 1 {
		t.Fatalf("max/unloads: got %d/%d, want 1/1", f.max, f.unloads)
	}
}

func TestCancellationAndPoisoning(t *testing.T) {
	native := errors.New("native")
	f := &fakeBackend{entered: make(chan struct{}, 1), release: make(chan struct{}, 1), err: errors.Join(model.ErrNativeGeneration, native)}
	m := newTestMalina(t, f, 1)
	result := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _, err := m.Generate(ctx, testParams()); result <- err }()
	<-f.entered
	cancel()
	select {
	case <-result:
		t.Fatal("returned while native generation was running")
	default:
	}
	f.release <- struct{}{}
	err := <-result
	if !errors.Is(err, ErrPoisoned) || !errors.Is(err, native) {
		t.Fatalf("Generate: got %v", err)
	}
	if err := m.Unload(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestQueuedCancellationDoesNotGenerate(t *testing.T) {
	f := &fakeBackend{entered: make(chan struct{}, 2), release: make(chan struct{}, 1)}
	m := newTestMalina(t, f, 2)
	first := make(chan error, 1)
	go func() { _, err := m.Generate(t.Context(), testParams()); first <- err }()
	<-f.entered

	ctx, cancel := context.WithCancel(t.Context())
	second := make(chan error, 1)
	go func() { _, err := m.Generate(ctx, testParams()); second <- err }()
	for m.ActiveGenerations() != 2 {
		runtime.Gosched()
	}
	cancel()
	if err := <-second; !errors.Is(err, context.Canceled) {
		t.Fatalf("Generate: got %v, want %v", err, context.Canceled)
	}

	f.release <- struct{}{}
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	select {
	case <-f.entered:
		t.Fatal("canceled queued generation entered backend")
	default:
	}
	if err := m.Unload(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestUnloadWaitsForRunningAndFailsQueued(t *testing.T) {
	f := &fakeBackend{entered: make(chan struct{}, 2), release: make(chan struct{}, 1)}
	m := newTestMalina(t, f, 2)
	first := make(chan error, 1)
	second := make(chan error, 1)
	go func() { _, err := m.Generate(t.Context(), testParams()); first <- err }()
	<-f.entered
	go func() { _, err := m.Generate(t.Context(), testParams()); second <- err }()
	for m.ActiveGenerations() != 2 {
		runtime.Gosched()
	}

	unloaded := make(chan error, 1)
	go func() { unloaded <- m.Unload(t.Context()) }()
	if err := <-second; !errors.Is(err, ErrClosed) {
		t.Fatalf("queued Generate: got %v, want ErrClosed", err)
	}
	select {
	case err := <-unloaded:
		t.Fatalf("Unload returned while native generation was running: %v", err)
	default:
	}

	f.release <- struct{}{}
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-unloaded; err != nil {
		t.Fatal(err)
	}
	select {
	case <-f.entered:
		t.Fatal("queued generation entered after unload")
	default:
	}
}

func TestNonNativeErrorDoesNotPoison(t *testing.T) {
	encodeErr := errors.New("encoding PNG")
	f := &fakeBackend{entered: make(chan struct{}, 2), release: make(chan struct{}, 2), err: encodeErr}
	m := newTestMalina(t, f, 1)

	for range 2 {
		result := make(chan error, 1)
		go func() { _, err := m.Generate(t.Context(), testParams()); result <- err }()
		<-f.entered
		f.release <- struct{}{}
		if err := <-result; !errors.Is(err, encodeErr) || errors.Is(err, ErrPoisoned) {
			t.Fatalf("Generate: got %v, want non-poisoning encoding error", err)
		}
	}
	if err := m.Unload(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestPoisonFailsQueuedWithPoisoned(t *testing.T) {
	native := errors.New("native")
	f := &fakeBackend{entered: make(chan struct{}, 1), release: make(chan struct{}, 1), err: errors.Join(model.ErrNativeGeneration, native)}
	m := newTestMalina(t, f, 2)
	results := make(chan error, 2)
	go func() { _, err := m.Generate(t.Context(), testParams()); results <- err }()
	<-f.entered
	go func() { _, err := m.Generate(t.Context(), testParams()); results <- err }()
	for m.ActiveGenerations() != 2 {
		runtime.Gosched()
	}
	f.release <- struct{}{}
	for range 2 {
		if err := <-results; !errors.Is(err, ErrPoisoned) {
			t.Fatalf("Generate: got %v, want ErrPoisoned", err)
		}
	}
	if err := m.Unload(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestUnloadTimeoutContinuesCleanup(t *testing.T) {
	f := &fakeBackend{entered: make(chan struct{}, 1), release: make(chan struct{}, 1)}
	m := newTestMalina(t, f, 1)
	result := make(chan error, 1)
	go func() { _, err := m.Generate(t.Context(), testParams()); result <- err }()
	<-f.entered
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := m.Unload(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Unload: got %v, want context.Canceled", err)
	}
	f.release <- struct{}{}
	if err := <-result; err != nil {
		t.Fatal(err)
	}

	unloads := make(chan error, 2)
	go func() { unloads <- m.Unload(t.Context()) }()
	go func() { unloads <- m.Unload(t.Context()) }()
	for range 2 {
		if err := <-unloads; err != nil {
			t.Fatal(err)
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.unloads != 1 {
		t.Fatalf("unloads: got %d, want 1", f.unloads)
	}
}

func TestInitRejectsConflictingPath(t *testing.T) {
	initState.Lock()
	oldDone, oldPath := initState.done, initState.path
	initState.done, initState.path = true, "/first"
	initState.Unlock()
	t.Cleanup(func() {
		initState.Lock()
		initState.done, initState.path = oldDone, oldPath
		initState.Unlock()
	})
	if err := Init(WithLibPath("/first")); err != nil {
		t.Fatalf("same path: %v", err)
	}
	if err := Init(WithLibPath("/second")); err == nil {
		t.Fatal("conflicting path: got nil error, want error")
	}
}
