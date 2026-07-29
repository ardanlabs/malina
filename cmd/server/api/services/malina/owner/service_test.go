package owner

import (
	"context"
	"errors"
	"sync"
	"testing"

	root "github.com/ardanlabs/malina/sdk/malina"
	"github.com/ardanlabs/malina/sdk/malina/model"
)

type fakeResident struct {
	generateStarted chan struct{}
	generateRelease chan struct{}
	unloadStarted   chan struct{}
	generateDone    chan struct{}
	poison          bool
	ready           bool
	unloadOnce      sync.Once
}

func (f *fakeResident) Generate(context.Context, model.GenerateParams) (model.GeneratedImage, error) {
	if f.generateStarted != nil {
		close(f.generateStarted)
		<-f.generateRelease
		close(f.generateDone)
	}
	if f.poison {
		return model.GeneratedImage{}, root.ErrPoisoned
	}
	return model.GeneratedImage{}, nil
}

func (f *fakeResident) Unload(context.Context) error {
	f.unloadOnce.Do(func() {
		f.ready = false
		if f.unloadStarted != nil {
			close(f.unloadStarted)
		}
	})
	if f.generateDone != nil {
		<-f.generateDone
	}
	return nil
}

func (f *fakeResident) ModelConfig() model.Config  { return model.Config{QueueDepth: 2} }
func (f *fakeResident) ModelInfo() model.ModelInfo { return model.ModelInfo{ModelPath: "model"} }
func (f *fakeResident) ActiveGenerations() int     { return 0 }
func (f *fakeResident) Ready() bool                { return f.ready }

func TestUnloadStartsWhileGenerationRuns(t *testing.T) {
	modelHandle := &fakeResident{
		generateStarted: make(chan struct{}),
		generateRelease: make(chan struct{}),
		unloadStarted:   make(chan struct{}),
		generateDone:    make(chan struct{}),
		ready:           true,
	}
	svc := NewWithLoader(2, func(context.Context, ...model.Option) (resident, error) { return modelHandle, nil })
	if err := svc.Load(t.Context(), "model", 0); err != nil {
		t.Fatal(err)
	}
	generated := make(chan error, 1)
	go func() { _, err := svc.Generate(t.Context(), model.GenerateParams{}); generated <- err }()
	<-modelHandle.generateStarted
	unloaded := make(chan error, 1)
	go func() { unloaded <- svc.Unload(t.Context()) }()
	<-modelHandle.unloadStarted
	close(modelHandle.generateRelease)
	if err := <-generated; err != nil {
		t.Fatal(err)
	}
	if err := <-unloaded; err != nil {
		t.Fatal(err)
	}
}

func TestCanceledLoadCleansConstructedModel(t *testing.T) {
	modelHandle := &fakeResident{ready: true}
	entered := make(chan struct{})
	returned := make(chan struct{})
	svc := NewWithLoader(2, func(context.Context, ...model.Option) (resident, error) {
		close(entered)
		<-returned
		return modelHandle, nil
	})
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() { result <- svc.Load(ctx, "model", 0) }()
	<-entered
	cancel()
	close(returned)
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("Load: got %v, want context.Canceled", err)
	}
	if modelHandle.ready {
		t.Fatal("model remained ready after canceled load")
	}
	if _, ok := svc.Status(); ok {
		t.Fatal("canceled model was published")
	}
}

func TestPoisonRemovesResident(t *testing.T) {
	modelHandle := &fakeResident{poison: true, ready: true}
	svc := NewWithLoader(2, func(context.Context, ...model.Option) (resident, error) { return modelHandle, nil })
	if err := svc.Load(t.Context(), "model", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Generate(t.Context(), model.GenerateParams{}); !errors.Is(err, root.ErrPoisoned) {
		t.Fatalf("Generate: got %v, want ErrPoisoned", err)
	}
	if _, ok := svc.Status(); ok {
		t.Fatal("poisoned model remained resident")
	}
}
