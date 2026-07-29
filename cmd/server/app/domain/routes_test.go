package domain

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	service "github.com/ardanlabs/malina/cmd/server/api/services/malina/owner"
	"github.com/ardanlabs/malina/cmd/server/api/services/malina/static"
	"github.com/ardanlabs/malina/cmd/server/app/sdk"
	root "github.com/ardanlabs/malina/sdk/malina"
	"github.com/ardanlabs/malina/sdk/malina/model"
)

type fakeService struct {
	info                            sdk.ModelInfo
	loaded                          bool
	generateErr, loadErr, unloadErr error
	png                             []byte
	params                          model.GenerateParams
}

func (f *fakeService) Generate(_ context.Context, params model.GenerateParams) (model.GeneratedImage, error) {
	f.params = params
	return model.GeneratedImage{PNG: f.png}, f.generateErr
}
func (f *fakeService) Load(context.Context, string, int) error {
	if f.loadErr == nil {
		f.loaded = true
	}
	return f.loadErr
}
func (f *fakeService) Unload(context.Context) error {
	if f.unloadErr == nil {
		f.loaded = false
	}
	return f.unloadErr
}
func (f *fakeService) Status() (sdk.ModelInfo, bool) { return f.info, f.loaded }

func TestGeneration(t *testing.T) {
	tests := []struct {
		name, body string
		svc        *fakeService
		want       int
		contains   string
	}{
		{name: "success", body: `{"prompt":"cat"}`, svc: &fakeService{loaded: true, png: []byte("png")}, want: http.StatusOK, contains: base64.StdEncoding.EncodeToString([]byte("png"))},
		{name: "n", body: `{"prompt":"cat","n":2}`, svc: &fakeService{}, want: http.StatusBadRequest, contains: "n must equal 1"},
		{name: "zero n", body: `{"prompt":"cat","n":0}`, svc: &fakeService{}, want: http.StatusBadRequest, contains: "n must equal 1"},
		{name: "zero steps", body: `{"prompt":"cat","steps":0}`, svc: &fakeService{}, want: http.StatusBadRequest, contains: "steps must be"},
		{name: "zero cfg", body: `{"prompt":"cat","cfg_scale":0}`, svc: &fakeService{}, want: http.StatusBadRequest, contains: "CFG scale"},
		{name: "invalid size", body: `{"prompt":"cat","size":"63x512"}`, svc: &fakeService{}, want: http.StatusBadRequest, contains: "dimensions must be"},
		{name: "format", body: `{"prompt":"cat","response_format":"url"}`, svc: &fakeService{}, want: http.StatusBadRequest, contains: "response_format"},
		{name: "unknown", body: `{"prompt":"cat","extra":true}`, svc: &fakeService{}, want: http.StatusBadRequest, contains: "unknown field"},
		{name: "malformed", body: `{"prompt":`, svc: &fakeService{}, want: http.StatusBadRequest, contains: "unexpected EOF"},
		{name: "multiple values", body: `{"prompt":"cat"}{}`, svc: &fakeService{}, want: http.StatusBadRequest, contains: "one JSON value"},
		{name: "no model", body: `{"prompt":"cat"}`, svc: &fakeService{generateErr: service.ErrNoModel}, want: http.StatusServiceUnavailable, contains: "model unavailable"},
		{name: "admission timeout", body: `{"prompt":"cat"}`, svc: &fakeService{generateErr: root.ErrAdmissionTimeout}, want: http.StatusTooManyRequests, contains: "admission timed out"},
		{name: "deadline", body: `{"prompt":"cat"}`, svc: &fakeService{generateErr: context.DeadlineExceeded}, want: http.StatusGatewayTimeout, contains: "request timed out"},
		{name: "canceled", body: `{"prompt":"cat"}`, svc: &fakeService{generateErr: context.Canceled}, want: http.StatusRequestTimeout, contains: "request canceled"},
		{name: "poisoned", body: `{"prompt":"cat"}`, svc: &fakeService{generateErr: errors.Join(root.ErrPoisoned, errors.New("native secret"))}, want: http.StatusServiceUnavailable, contains: "model unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			NewMux(tt.svc, nil).ServeHTTP(w, r)
			if w.Code != tt.want || !strings.Contains(w.Body.String(), tt.contains) {
				t.Errorf("response: got %d %q, want %d containing %q", w.Code, w.Body.String(), tt.want, tt.contains)
			}
			if tt.name == "poisoned" && strings.Contains(w.Body.String(), "native secret") {
				t.Errorf("response leaked native error: %q", w.Body.String())
			}
		})
	}
}

func TestHealthReadinessAndUnknownRoutes(t *testing.T) {
	svc := &fakeService{}
	tests := []struct {
		path string
		want int
	}{
		{path: "/healthz", want: http.StatusOK},
		{path: "/readyz", want: http.StatusServiceUnavailable},
		{path: "/v1/unknown", want: http.StatusNotFound},
		{path: "/admin/missing", want: http.StatusNotFound},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodGet, tt.path, nil)
		w := httptest.NewRecorder()
		NewMux(svc, static.Handler()).ServeHTTP(w, r)
		if w.Code != tt.want {
			t.Errorf("%s: got %d, want %d", tt.path, w.Code, tt.want)
		}
	}
	svc.loaded = true
	r := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	NewMux(svc, nil).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("ready with model: got %d, want 200", w.Code)
	}
}

func TestGenerationPreservesZeroSeed(t *testing.T) {
	svc := &fakeService{loaded: true, png: []byte("png")}
	r := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"prompt":"cat","seed":0}`))
	w := httptest.NewRecorder()
	NewMux(svc, nil).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if svc.params.Seed != 0 {
		t.Errorf("seed: got %d, want 0", svc.params.Seed)
	}
}

func TestManagementAndStatic(t *testing.T) {
	svc := &fakeService{loaded: true, info: sdk.ModelInfo{Path: "model.gguf"}}
	for _, path := range []string{"/v1/models", "/v1/malina/models", "/v1/malina/models/ps", "/admin/"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		NewMux(svc, static.Handler()).ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("%s: got %d, want 200", path, w.Code)
		}
	}
	svc.loadErr = service.ErrModelResident
	r := httptest.NewRequest(http.MethodPost, "/v1/malina/models/load", bytes.NewBufferString(`{"model_path":"other"}`))
	w := httptest.NewRecorder()
	NewMux(svc, nil).ServeHTTP(w, r)
	if w.Code != http.StatusConflict {
		t.Errorf("load: got %d, want 409", w.Code)
	}
	svc.loadErr = nil
	r = httptest.NewRequest(http.MethodPost, "/v1/malina/models/unload", strings.NewReader(`{}`))
	w = httptest.NewRecorder()
	NewMux(svc, nil).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("unload: got %d, want 200", w.Code)
	}
}

func TestOversizedJSON(t *testing.T) {
	svc := &fakeService{}
	r := httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"prompt":"`+strings.Repeat("x", maxBodyBytes)+`"}`))
	w := httptest.NewRecorder()
	NewMux(svc, nil).ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", w.Code)
	}
}
