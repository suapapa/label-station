package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"os"
	"strings"

	brother_ql "github.com/suapapa/go_brother-ql"
)

var (
	model   string
	backend string
	printer string
	port    int
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

func main() {
	flag.StringVar(&model, "model", os.Getenv("BROTHER_QL_MODEL"), "Printer model (e.g., QL-800, QL-700)")
	flag.StringVar(&backend, "backend", os.Getenv("BROTHER_QL_BACKEND"), "Backend type (e.g., network, linux_kernel)")
	flag.StringVar(&printer, "printer", os.Getenv("BROTHER_QL_PRINTER"), "Printer identifier/address (e.g., 192.168.1.100 or /dev/usb/lp0)")
	flag.IntVar(&port, "port", 8080, "Server port")
	flag.Parse()

	if model == "" || backend == "" || printer == "" {
		fmt.Println("Error: --model, --backend, and --printer are all required.")
		fmt.Println("Example: ./label-station --model QL-800 --backend network --printer 192.168.1.100")
		os.Exit(1)
	}

	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("GET /api/v1/info", handleInfo)
	mux.HandleFunc("POST /api/v1/print", handlePrint)

	// Serve Frontend
	fs := http.FileServer(http.Dir("./fe/dist"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// If file doesn't exist, serve index.html (for SPA router, though maybe not needed for simple app)
		path := r.URL.Path
		if path == "/" {
			fs.ServeHTTP(w, r)
			return
		}
		
		// Fallback to index.html if file doesn't exist
		if _, err := os.Stat("fe/dist" + path); os.IsNotExist(err) {
			http.ServeFile(w, r, "fe/dist/index.html")
			return
		}
		fs.ServeHTTP(w, r)
	})

	// Wrap with simple CORS middleware for development
	handler := corsMiddleware(mux)

	fmt.Printf("Starting server on :%d\n", port)
	fmt.Printf("Model: %s, Backend: %s, Printer: %s\n", model, backend, printer)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), handler))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func handleInfo(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"model":   model,
		"backend": backend,
		"printer": printer,
		"labels":  brother_ql.AllLabels,
		"models":  brother_ql.AllModels,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handlePrint(w http.ResponseWriter, r *http.Request) {
	var req PrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.Image == "" {
		http.Error(w, "image is required", http.StatusBadRequest)
		return
	}

	// Decode Base64 image
	imgData, err := base64.StdEncoding.DecodeString(req.Image)
	if err != nil {
		// Try with data URL stripping
		if idx := strings.Index(req.Image, ","); idx != -1 {
			imgData, err = base64.StdEncoding.DecodeString(req.Image[idx+1:])
		}
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid base64 image: %v", err), http.StatusBadRequest)
			return
		}
	}

	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to decode image: %v", err), http.StatusBadRequest)
		return
	}

	// Create printer
	brd, err := brother_ql.NewLabelPrinter(model, backend, printer)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to connect to printer: %v", err), http.StatusInternalServerError)
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

	// If Threshold is 0, default to 70.0 as seen in main.go
	if opts.Threshold == 0 {
		opts.Threshold = 70.0
	}
	if opts.DitherAlgo == "" {
		opts.DitherAlgo = "floyd_steinberg"
	}

	if err := brd.Print([]image.Image{img}, opts); err != nil {
		http.Error(w, fmt.Sprintf("printing failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Label printed successfully"})
}
