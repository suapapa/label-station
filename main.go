package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	var (
		model   = flag.String("model", os.Getenv("BROTHER_QL_MODEL"), "Printer model (e.g., QL-800, QL-700)")
		backend = flag.String("backend", os.Getenv("BROTHER_QL_BACKEND"), "Backend type (e.g., network, linux_kernel)")
		printer = flag.String("printer", os.Getenv("BROTHER_QL_PRINTER"), "Printer identifier/address (e.g., 192.168.1.100 or /dev/usb/lp0)")
		port    = flag.Int("port", 8080, "Server port")
	)
	flag.Parse()

	if *model == "" || *backend == "" || *printer == "" {
		fmt.Println("Error: --model, --backend, and --printer are all required.")
		fmt.Println("Example: ./label-station --model QL-800 --backend network --printer 192.168.1.100")
		os.Exit(1)
	}

	// Setup context and cancellation for the whole application
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	svc := NewService(*model, *backend, *printer)
	// Initial connection effort (optional, reconnect loop will handle it anyway)
	go svc.StartReconnectLoop(ctx)
	defer func() {
		if err := svc.Close(); err != nil {
			slog.Error("error closing printer", "error", err)
		}
	}()

	r := gin.Default()
	r.Use(corsMiddleware())

	// API endpoints
	api := r.Group("/api/v1")
	{
		api.GET("/info", svc.HandleInfo)
		api.GET("/ping", svc.HandlePing)
		api.GET("/events", svc.HandleEvents)
		api.POST("/print", svc.HandlePrint)
	}

	// Serve Frontend if exists
	setupFrontend(r)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: r,
	}

	go func() {
		slog.Info("starting server", "port", *port, "model", *model, "backend", *backend, "printer", *printer)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen and serve failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server exiting")
}

func setupFrontend(r *gin.Engine) {
	distPath := "./fe/dist"
	if _, err := os.Stat(distPath); err != nil {
		slog.Warn("frontend static files not found, starting in API-only mode", "path", distPath)
		return
	}

	r.Static("/assets", filepath.Join(distPath, "assets"))

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
			return
		}

		if path == "/" {
			c.File(filepath.Join(distPath, "index.html"))
			return
		}

		fpath := filepath.Join(distPath, path)
		if _, err := os.Stat(fpath); os.IsNotExist(err) {
			c.File(filepath.Join(distPath, "index.html"))
			return
		}
		c.File(fpath)
	})
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}


