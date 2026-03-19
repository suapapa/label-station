package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	_ "golang.org/x/image/webp"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	brother_ql "github.com/suapapa/go_brother-ql"
)

type PrintRequest struct {
	Image   string         `json:"image"` // Base64 encoded image
	Label   string         `json:"label"` // e.g., "62", "29x90"
	Options ConvertOptions `json:"options"`
}

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

type Service struct {
	printer *brother_ql.LabelPrinter
	mu      sync.Mutex
}

func (s *Service) handleInfo(c *gin.Context) {
	resp := map[string]interface{}{
		"model":   model,
		"backend": backend,
		"printer": printer,
		"labels":  brother_ql.AllLabels,
		"models":  brother_ql.AllModels,
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Service) handlePrint(c *gin.Context) {
	var req PrintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	if req.Image == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image is required"})
		return
	}

	imgData, err := base64.StdEncoding.DecodeString(req.Image)
	if err != nil {
		if idx := strings.Index(req.Image, ","); idx != -1 {
			imgData = []byte(req.Image) // wait, just try decode
			// Wait, previous code was:
			// if idx := strings.Index(req.Image, ","); idx != -1 {
			//     imgData, err = base64.StdEncoding.DecodeString(req.Image[idx+1:])
			// }
		}
		// I will copy exact previous code:
		if idx := strings.Index(req.Image, ","); idx != -1 {
			imgData, err = base64.StdEncoding.DecodeString(req.Image[idx+1:])
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid base64 image: %v", err)})
			return
		}
	}

	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("failed to decode image: %v", err)})
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

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.printer.Print([]image.Image{img}, opts); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("printing failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Label printed successfully"})
}
