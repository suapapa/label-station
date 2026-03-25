package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	brother_ql "github.com/suapapa/go_brother-ql"
	_ "golang.org/x/image/webp"
)

// PrintRequest defines the API request body for printing a label.
type PrintRequest struct {
	Image   string         `json:"image"` // Base64 encoded image
	Label   string         `json:"label"` // e.g., "62", "29x90"
	Options ConvertOptions `json:"options"`
}

// ConvertOptions defines the image processing options for printing.
type ConvertOptions struct {
	Cut        bool    `json:"cut"`
	Dither     bool    `json:"dither"`
	DitherAlgo string  `json:"dither_algo"`
	Compress   bool    `json:"compress"`
	Red        bool    `json:"red"`
	Rotate     string  `json:"rotate"`
	Dpi600     bool    `json:"dpi600"`
	Hq         bool    `json:"hq"`
	Threshold  float64 `json:"threshold"`
}

// Service handles the label printing operations and printer connectivity.
type Service struct {
	model   string
	backend string
	address string

	printer *brother_ql.LabelPrinter
	mu      sync.RWMutex

	status Status
	subs   map[chan Status]struct{}
	subMu  sync.Mutex
}

// Status represents the printer connectivity state.
type Status string

const (
	StatusOnline  Status = "online"
	StatusOffline Status = "offline"
	StatusUnknown Status = "checking"
)

// NewService creates a new Service instance.
func NewService(model, backend, address string) *Service {
	return &Service{
		model:   model,
		backend: backend,
		address: address,
		status:  StatusUnknown,
		subs:    make(map[chan Status]struct{}),
	}
}

// HandleInfo returns information about the printer and available labels.
func (s *Service) HandleInfo(c *gin.Context) {
	resp := map[string]interface{}{
		"model":   s.model,
		"backend": s.backend,
		"printer": s.address,
		"labels":  brother_ql.AllLabels,
		"models":  brother_ql.AllModels,
	}
	c.JSON(http.StatusOK, resp)
}

// HandlePrint processes a print request and sends it to the printer.
func (s *Service) HandlePrint(c *gin.Context) {
	var req PrintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Errorf("invalid request body: %w", err).Error()})
		return
	}

	if req.Image == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image is required"})
		return
	}

	imgData, err := decodeImage(req.Image)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Errorf("invalid base64 image: %w", err).Error()})
		return
	}

	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Errorf("failed to decode image: %w", err).Error()})
		return
	}

	opts := brother_ql.PrintOptions{
		Label: req.Label,
		ConvertOptions: brother_ql.ConvertOptions{
			Cut:        req.Options.Cut,
			Dither:     req.Options.Dither,
			DitherAlgo: req.Options.DitherAlgo,
			Compress:   req.Options.Compress,
			Red:        req.Options.Red,
			Rotate:     req.Options.Rotate,
			Dpi600:     req.Options.Dpi600,
			Hq:         req.Options.Hq,
			Threshold:  req.Options.Threshold,
		},
	}

	if opts.Threshold == 0 {
		opts.Threshold = 70.0
	}
	if opts.DitherAlgo == "" {
		opts.DitherAlgo = "floyd_steinberg"
	}

	s.mu.RLock()
	printer := s.printer
	s.mu.RUnlock()

	if printer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "printer not connected"})
		return
	}

	if err := printer.Print(c.Request.Context(), []image.Image{img}, opts); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Errorf("printing failed: %w", err).Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Label printed successfully"})
}

// HandlePing checks the current status of the printer.
func (s *Service) HandlePing(c *gin.Context) {
	s.mu.RLock()
	status := s.status
	s.mu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"status": status})
}

// HandleEvents provides a Server-Sent Events stream for printer status updates.
func (s *Service) HandleEvents(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	ch := make(chan Status, 1)
	s.subscribe(ch)
	defer s.unsubscribe(ch)

	// Send initial status
	s.mu.RLock()
	initialStatus := s.status
	s.mu.RUnlock()

	c.SSEvent("status", string(initialStatus))
	c.Writer.Flush()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case status := <-ch:
			c.SSEvent("status", string(status))
			c.Writer.Flush()
		}
	}
}

func (s *Service) subscribe(ch chan Status) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	s.subs[ch] = struct{}{}
}

func (s *Service) unsubscribe(ch chan Status) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	delete(s.subs, ch)
}

func (s *Service) broadcast(status Status) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- status:
		default:
			// Buffer full, skip this subscriber
		}
	}
}

// StartReconnectLoop maintains the printer connection in the background.
// It stops when the given context is cancelled.
func (s *Service) StartReconnectLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("stopping reconnect loop")
			return
		case <-ticker.C:
			s.checkAndReconnect(ctx)
		}
	}
}

func (s *Service) checkAndReconnect(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var newStatus Status = StatusOffline
	if s.printer != nil {
		if s.printer.IsLive(ctx) {
			newStatus = StatusOnline
		} else {
			slog.Warn("printer is offline, attempting to reconnect", "model", s.model, "address", s.address)
			if err := s.printer.Reconnect(ctx); err != nil {
				slog.Error("reconnect failed", "error", err)
			} else {
				slog.Info("printer reconnected successfully")
				newStatus = StatusOnline
			}
		}
	} else {
		slog.Info("printer not initialized, attempting to connect", "model", s.model, "backend", s.backend, "address", s.address)
		newBrd, err := brother_ql.NewLabelPrinter(ctx, s.model, s.backend, s.address)
		if err == nil {
			s.printer = newBrd
			slog.Info("printer connected successfully")
			newStatus = StatusOnline
		} else {
			slog.Error("connection failed", "error", err)
		}
	}

	if s.status != newStatus {
		s.status = newStatus
		go s.broadcast(newStatus)
	}
}

// Close gracefully closes the printer connection.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.printer != nil {
		return s.printer.Close()
	}
	return nil
}

func decodeImage(data string) ([]byte, error) {
	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
	}
	return base64.StdEncoding.DecodeString(data)
}

