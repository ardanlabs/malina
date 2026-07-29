// Package malina provides concurrency-safe, fully local image generation
// backed by stable-diffusion.cpp (https://github.com/leejet/stable-diffusion.cpp).
// The sdk/malina/sd package is the low-level FFI layer and is not intended for
// application code.
//
//   - Run any stable-diffusion.cpp supported model on Linux, macOS, or Windows.
//   - Use any available hardware acceleration such as CUDA (https://en.wikipedia.org/wiki/CUDA),
//     Metal (https://en.wikipedia.org/wiki/Metal_(API)), or Vulkan (https://en.wikipedia.org/wiki/Vulkan)
//     for maximum performance.
//   - malina uses the purego (https://github.com/ebitengine/purego) and ffi (https://github.com/JupiterRider/ffi)
//     packages so CGo is not needed.
//   - Works with the newest stable-diffusion.cpp releases so you can use the latest features and
//     model support.
//
// malina is the text-to-image sibling of bucky (https://github.com/ardanlabs/bucky), which
// provides the same kind of FFI bindings for whisper.cpp.
package malina

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ardanlabs/malina/sdk/malina/model"
	"github.com/ardanlabs/malina/sdk/malina/sd"
)

var (
	// ErrInvalidRequest identifies invalid generation parameters.
	ErrInvalidRequest = model.ErrInvalidRequest
	// ErrAdmissionTimeout identifies expiration while waiting for queue admission.
	ErrAdmissionTimeout = errors.New("generation admission timed out")
	// ErrClosed identifies use after unloading has begun.
	ErrClosed = errors.New("malina is closed")
	// ErrPoisoned identifies a terminal native generation failure.
	ErrPoisoned = errors.New("malina is poisoned")
)

// InitOption configures process-wide initialization.
type InitOption func(*initConfig)

type initConfig struct{ libPath string }

var initState struct {
	sync.Mutex
	done bool
	path string
}

// WithLibPath sets the stable-diffusion shared library path.
func WithLibPath(path string) InitOption {
	return func(cfg *initConfig) { cfg.libPath = path }
}

// Init loads stable-diffusion and registers its dynamic backends.
func Init(opts ...InitOption) error {
	initState.Lock()
	defer initState.Unlock()
	if initState.done {
		var cfg initConfig
		for _, opt := range opts {
			opt(&cfg)
		}
		if cfg.libPath != "" && cfg.libPath != initState.path {
			return fmt.Errorf("init: stable-diffusion already initialized from %q, cannot use %q", initState.path, cfg.libPath)
		}
		return nil
	}

	cfg := initConfig{libPath: os.Getenv("MALINA_LIB")}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.libPath == "" {
		return errors.New("init: library path is required (set MALINA_LIB or use WithLibPath)")
	}
	if err := sd.Load(cfg.libPath); err != nil {
		return fmt.Errorf("loading stable-diffusion: %w", err)
	}
	if sd.GGMLBackendDeviceCount() <= 0 {
		if err := sd.Init(cfg.libPath); err != nil {
			return fmt.Errorf("initializing stable-diffusion backends: %w", err)
		}
	}
	initState.done = true
	initState.path = cfg.libPath
	return nil
}

// Initialized reports whether process-wide initialization succeeded.
func Initialized() bool {
	initState.Lock()
	defer initState.Unlock()
	return initState.done
}

// SystemDiagnostics contains a snapshot of native library and host diagnostics.
type SystemDiagnostics struct {
	NativeVersion      string
	PhysicalCores      int32
	BackendDeviceCount int
	Description        string
}

// SystemInfo returns native library and host diagnostics after initialization.
func SystemInfo() (SystemDiagnostics, error) {
	if !Initialized() {
		return SystemDiagnostics{}, errors.New("system info: malina is not initialized")
	}
	info := SystemDiagnostics{
		NativeVersion:      sd.Version(),
		PhysicalCores:      sd.NumPhysicalCores(),
		BackendDeviceCount: sd.GGMLBackendDeviceCount(),
		Description:        sd.SystemInfo(),
	}
	return info, nil
}

type backend interface {
	Generate(model.GenerateParams) (model.GeneratedImage, error)
	Unload() error
	Config() model.Config
	Info() model.ModelInfo
}

var newBackend = func(ctx context.Context, cfg model.Config) (backend, error) {
	return model.NewModel(ctx, cfg)
}

type request struct {
	ctx    context.Context
	params model.GenerateParams
	done   chan result
	mu     sync.Mutex
	start  bool
	stop   bool
}

type result struct {
	image model.GeneratedImage
	err   error
}

// Malina serializes access to one reusable native model context.
type Malina struct {
	backend   backend
	config    model.Config
	jobs      chan *request
	stop      chan struct{}
	done      chan struct{}
	mu        sync.Mutex
	closed    bool
	terminal  error
	unloadErr error
	active    atomic.Int64
	admit     chan struct{}
}

// New constructs a Malina with a background model-loading context.
func New(opts ...model.Option) (*Malina, error) {
	return NewWithContext(context.Background(), opts...)
}

// NewWithContext constructs a Malina and loads its model.
func NewWithContext(ctx context.Context, opts ...model.Option) (*Malina, error) {
	if !Initialized() {
		return nil, errors.New("new: malina is not initialized")
	}
	cfg, err := model.NewConfig(opts...)
	if err != nil {
		return nil, err
	}
	b, err := newBackend(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("loading model: %w", err)
	}
	m := Malina{backend: b, config: cfg, jobs: make(chan *request), stop: make(chan struct{}), done: make(chan struct{}), admit: make(chan struct{}, cfg.QueueDepth)}
	go m.worker()
	return &m, nil
}

// Generate admits and synchronously executes one generation.
func (m *Malina) Generate(ctx context.Context, params model.GenerateParams) (model.GeneratedImage, error) {
	if err := params.Validate(); err != nil {
		return model.GeneratedImage{}, err
	}
	m.mu.Lock()
	if m.closed {
		err := errors.Join(ErrClosed, m.terminal)
		m.mu.Unlock()
		return model.GeneratedImage{}, err
	}
	m.mu.Unlock()

	timer := time.NewTimer(m.config.AdmissionTimeout)
	defer timer.Stop()
	select {
	case m.admit <- struct{}{}:
	case <-ctx.Done():
		return model.GeneratedImage{}, ctx.Err()
	case <-timer.C:
		return model.GeneratedImage{}, errors.Join(ErrAdmissionTimeout, context.DeadlineExceeded)
	case <-m.stop:
		return model.GeneratedImage{}, m.closedError()
	}
	defer func() { <-m.admit }()
	m.active.Add(1)
	defer m.active.Add(-1)

	r := &request{ctx: ctx, params: params, done: make(chan result, 1)}
	select {
	case m.jobs <- r:
	case <-ctx.Done():
		return model.GeneratedImage{}, ctx.Err()
	case <-m.stop:
		return model.GeneratedImage{}, m.closedError()
	}
	select {
	case out := <-r.done:
		return out.image, out.err
	case <-ctx.Done():
		if r.cancel() {
			return model.GeneratedImage{}, ctx.Err()
		}
		out := <-r.done
		return out.image, out.err
	case <-m.stop:
		if r.cancel() {
			return model.GeneratedImage{}, m.closedError()
		}
		out := <-r.done
		return out.image, out.err
	}
}

func (r *request) cancel() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.start {
		return false
	}
	r.stop = true
	return true
}

// ModelConfig returns the model's immutable configuration.
func (m *Malina) ModelConfig() model.Config { return m.config }

// ModelInfo returns descriptive model information.
func (m *Malina) ModelInfo() model.ModelInfo { return m.backend.Info() }

// ActiveGenerations returns the number of admitted generation calls.
func (m *Malina) ActiveGenerations() int { return int(m.active.Load()) }

// Ready reports whether the model can accept generation requests.
func (m *Malina) Ready() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return !m.closed
}

// Unload stops admission and waits for safe context release.
func (m *Malina) Unload(ctx context.Context) error {
	m.mu.Lock()
	if !m.closed {
		m.closed = true
		close(m.stop)
	}
	m.mu.Unlock()
	select {
	case <-m.done:
		m.mu.Lock()
		err := m.unloadErr
		m.mu.Unlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Malina) worker() {
	defer close(m.done)
	for {
		select {
		case <-m.stop:
			m.finishUnload()
			return
		case r := <-m.jobs:
			if err := m.start(r); err != nil {
				r.done <- result{err: err}
				continue
			}
			img, err := m.backend.Generate(r.params)
			if errors.Is(err, model.ErrNativeGeneration) {
				err = errors.Join(ErrPoisoned, err)
				m.mu.Lock()
				m.terminal = ErrPoisoned
				if !m.closed {
					m.closed = true
					close(m.stop)
				}
				m.mu.Unlock()
			}
			r.done <- result{image: img, err: err}
			if errors.Is(err, ErrPoisoned) {
				m.finishUnload()
				return
			}
		}
	}
}

func (m *Malina) start(r *request) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if m.closed {
		return errors.Join(ErrClosed, m.terminal)
	}
	if r.stop {
		return r.ctx.Err()
	}
	if err := r.ctx.Err(); err != nil {
		r.stop = true
		return err
	}
	r.start = true
	return nil
}

func (m *Malina) closedError() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return errors.Join(ErrClosed, m.terminal)
}

func (m *Malina) finishUnload() {
	err := m.backend.Unload()
	m.mu.Lock()
	m.unloadErr = err
	m.mu.Unlock()
}
