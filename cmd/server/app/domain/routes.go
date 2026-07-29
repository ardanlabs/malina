// Package domain provides Malina's HTTP transport.
package domain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	service "github.com/ardanlabs/malina/cmd/server/api/services/malina/owner"
	"github.com/ardanlabs/malina/cmd/server/app/sdk"
	root "github.com/ardanlabs/malina/sdk/malina"
	"github.com/ardanlabs/malina/sdk/malina/model"
)

const maxBodyBytes = 1 << 20

// NewMux composes all API and optional administration routes.
func NewMux(svc sdk.Service, admin http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		if _, ok := svc.Status(); !ok {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("POST /v1/images/generations", generate(svc))
	mux.HandleFunc("GET /v1/models", listOpenAI(svc))
	mux.HandleFunc("GET /v1/malina/models", status(svc))
	mux.HandleFunc("GET /v1/malina/models/ps", status(svc))
	mux.HandleFunc("POST /v1/malina/models/load", load(svc))
	mux.HandleFunc("POST /v1/malina/models/unload", unload(svc))
	if admin != nil {
		mux.Handle("/admin/", http.StripPrefix("/admin/", admin))
		mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/admin/", http.StatusTemporaryRedirect)
		})
	}
	return mux
}

type generationRequest struct {
	Prompt         string   `json:"prompt"`
	NegativePrompt string   `json:"negative_prompt"`
	N              *int     `json:"n"`
	Size           string   `json:"size"`
	Steps          *int     `json:"steps"`
	CFGScale       *float32 `json:"cfg_scale"`
	Seed           *int64   `json:"seed"`
	ResponseFormat string   `json:"response_format"`
}

func generate(svc sdk.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req generationRequest
		if err := decode(w, r, &req); err != nil {
			writeError(w, err)
			return
		}
		if req.N != nil && *req.N != 1 {
			writeError(w, errors.Join(model.ErrInvalidRequest, errors.New("n must equal 1")))
			return
		}
		if req.ResponseFormat != "" && req.ResponseFormat != "b64_json" {
			writeError(w, errors.Join(model.ErrInvalidRequest, errors.New("response_format must be b64_json")))
			return
		}
		p := model.GenerateParams{Prompt: req.Prompt, NegativePrompt: req.NegativePrompt, Width: 512, Height: 512, Steps: 20, CFGScale: 7, Seed: -1}
		if req.Size != "" {
			parts := strings.Split(req.Size, "x")
			if len(parts) != 2 {
				writeError(w, errors.Join(model.ErrInvalidRequest, errors.New("size must be WxH")))
				return
			}
			var err error
			p.Width, err = strconv.Atoi(parts[0])
			if err != nil {
				writeError(w, errors.Join(model.ErrInvalidRequest, errors.New("invalid width")))
				return
			}
			p.Height, err = strconv.Atoi(parts[1])
			if err != nil {
				writeError(w, errors.Join(model.ErrInvalidRequest, errors.New("invalid height")))
				return
			}
		}
		if req.Steps != nil {
			p.Steps = *req.Steps
		}
		if req.CFGScale != nil {
			p.CFGScale = *req.CFGScale
		}
		if req.Seed != nil {
			p.Seed = *req.Seed
		}
		if err := p.Validate(); err != nil {
			writeError(w, err)
			return
		}
		img, err := svc.Generate(r.Context(), p)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"created": time.Now().Unix(), "data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(img.PNG)}}})
	}
}

func listOpenAI(svc sdk.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		data := []map[string]any{}
		if info, ok := svc.Status(); ok {
			data = append(data, map[string]any{"id": info.Path, "object": "model", "owned_by": "malina"})
		}
		writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
	}
}

func status(svc sdk.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		info, ok := svc.Status()
		writeJSON(w, http.StatusOK, map[string]any{"loaded": ok, "model": func() any {
			if ok {
				return info
			}
			return nil
		}()})
	}
}

func load(svc sdk.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ModelPath  string `json:"model_path"`
			QueueDepth int    `json:"queue_depth"`
		}
		if err := decode(w, r, &req); err != nil {
			writeError(w, err)
			return
		}
		if req.ModelPath == "" || req.QueueDepth < 0 {
			writeError(w, errors.Join(model.ErrInvalidRequest, errors.New("model_path is required and queue_depth cannot be negative")))
			return
		}
		if err := svc.Load(r.Context(), req.ModelPath, req.QueueDepth); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"status": "loaded"})
	}
}

func unload(svc sdk.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := decode(w, r, &struct{}{}); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, err)
			return
		}
		if err := svc.Unload(r.Context()); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "unloaded"})
	}
}

func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return errors.Join(model.ErrInvalidRequest, err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.Join(model.ErrInvalidRequest, errors.New("request body must contain one JSON value"))
	}
	return nil
}

func writeError(w http.ResponseWriter, err error) {
	status, message := http.StatusInternalServerError, "internal server error"
	switch {
	case errors.Is(err, model.ErrInvalidRequest):
		status, message = http.StatusBadRequest, err.Error()
	case errors.Is(err, service.ErrModelResident):
		status, message = http.StatusConflict, err.Error()
	case errors.Is(err, service.ErrNoModel), errors.Is(err, root.ErrClosed), errors.Is(err, root.ErrPoisoned):
		status, message = http.StatusServiceUnavailable, "model unavailable"
	case errors.Is(err, root.ErrAdmissionTimeout):
		status, message = http.StatusTooManyRequests, "request admission timed out"
	case errors.Is(err, context.DeadlineExceeded):
		status, message = http.StatusGatewayTimeout, "request timed out"
	case errors.Is(err, context.Canceled):
		status, message = http.StatusRequestTimeout, "request canceled"
	}
	writeJSON(w, status, map[string]any{"error": map[string]string{"message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}
