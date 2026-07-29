// Package owner provides ownership of the server's single resident model.
package owner

import (
	"context"
	"errors"
	"sync"

	"github.com/ardanlabs/malina/cmd/server/app/sdk"
	root "github.com/ardanlabs/malina/sdk/malina"
	"github.com/ardanlabs/malina/sdk/malina/model"
)

var (
	// ErrNoModel identifies an operation requiring a resident model.
	ErrNoModel = errors.New("no model is loaded")
	// ErrModelResident identifies an attempt to load a second model.
	ErrModelResident = errors.New("a model is already loaded")
)

type resident interface {
	Generate(context.Context, model.GenerateParams) (model.GeneratedImage, error)
	Unload(context.Context) error
	ModelConfig() model.Config
	ModelInfo() model.ModelInfo
	ActiveGenerations() int
	Ready() bool
}

type loader func(context.Context, ...model.Option) (resident, error)

// Service serializes model lifecycle changes and owns zero or one model.
type Service struct {
	mu                sync.Mutex
	model             resident
	load              loader
	defaultQueueDepth int
	lifecycle         chan struct{}
}

// New initializes the shared native library and constructs a Service.
func New(libPath string, queueDepth int) (*Service, error) {
	if err := root.Init(root.WithLibPath(libPath)); err != nil {
		return nil, err
	}
	return NewWithLoader(queueDepth, func(ctx context.Context, opts ...model.Option) (resident, error) {
		return root.NewWithContext(ctx, opts...)
	}), nil
}

// NewWithLoader constructs a Service with a model constructor seam.
func NewWithLoader(queueDepth int, load loader) *Service {
	if queueDepth < 1 {
		queueDepth = 2
	}
	s := Service{load: load, defaultQueueDepth: queueDepth, lifecycle: make(chan struct{}, 1)}
	return &s
}

// Load installs exactly one resident model.
func (s *Service) Load(ctx context.Context, path string, queueDepth int) error {
	if err := s.acquireLifecycle(ctx); err != nil {
		return err
	}
	defer s.releaseLifecycle()
	s.mu.Lock()
	if s.model != nil {
		s.mu.Unlock()
		return ErrModelResident
	}
	s.mu.Unlock()
	if queueDepth == 0 {
		queueDepth = s.defaultQueueDepth
	}
	m, err := s.load(ctx, model.WithModelPath(path), model.WithQueueDepth(queueDepth))
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		if unloadErr := m.Unload(context.Background()); unloadErr != nil {
			return errors.Join(err, unloadErr)
		}
		return err
	}
	s.mu.Lock()
	s.model = m
	s.mu.Unlock()
	return nil
}

// Generate runs a request through the Malina SDK's resident model.
func (s *Service) Generate(ctx context.Context, params model.GenerateParams) (model.GeneratedImage, error) {
	s.mu.Lock()
	m := s.model
	s.mu.Unlock()
	if m == nil {
		return model.GeneratedImage{}, ErrNoModel
	}
	img, err := m.Generate(ctx, params)
	if errors.Is(err, root.ErrPoisoned) {
		if unloadErr := m.Unload(context.Background()); unloadErr != nil {
			err = errors.Join(err, unloadErr)
		}
		s.mu.Lock()
		if s.model == m {
			s.model = nil
		}
		s.mu.Unlock()
	}
	return img, err
}

// Unload waits for active generation and releases the resident model.
func (s *Service) Unload(ctx context.Context) error {
	if err := s.acquireLifecycle(ctx); err != nil {
		return err
	}
	defer s.releaseLifecycle()
	s.mu.Lock()
	m := s.model
	s.mu.Unlock()
	if m == nil {
		return ErrNoModel
	}
	if err := m.Unload(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.model == m {
		s.model = nil
	}
	return nil
}

// Status returns a snapshot of the resident model.
func (s *Service) Status() (sdk.ModelInfo, bool) {
	s.mu.Lock()
	m := s.model
	s.mu.Unlock()
	if m == nil || !m.Ready() {
		return sdk.ModelInfo{}, false
	}
	cfg := m.ModelConfig()
	info := m.ModelInfo()
	return sdk.ModelInfo{Path: info.ModelPath, QueueDepth: cfg.QueueDepth, ActiveGenerations: m.ActiveGenerations()}, true
}

func (s *Service) acquireLifecycle(ctx context.Context) error {
	select {
	case s.lifecycle <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) releaseLifecycle() {
	<-s.lifecycle
}
