package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	brother_ql "github.com/suapapa/go_brother-ql"
)

var (
	model   string
	backend string
	printer string
	port    int
)


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

	brd, err := brother_ql.NewLabelPrinter(model, backend, printer)
	if err != nil {
		fmt.Printf("Error connecting to printer: %v\n", err)
		os.Exit(1)
	}
	defer brd.Close()

	svc := &Service{printer: brd}

	r := gin.Default()

	// Wrap with simple CORS middleware
	r.Use(corsMiddleware())

	// API endpoints
	r.GET("/api/v1/info", svc.handleInfo)
	r.POST("/api/v1/print", svc.handlePrint)

	// Serve Frontend if exists
	if _, err := os.Stat("./fe/dist"); err == nil {
		r.Static("/assets", "./fe/dist/assets")

		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
				return
			}

			if path == "/" {
				c.File("./fe/dist/index.html")
				return
			}

			fpath := filepath.Join("fe", "dist", path)
			if _, err := os.Stat(fpath); os.IsNotExist(err) {
				c.File("./fe/dist/index.html")
				return
			}
			c.File(fpath)
		})
	} else {
		fmt.Println("Frontend static files (./fe/dist) not found. Starting in API-only mode.")
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}

	go func() {
		fmt.Printf("Starting server on :%d\n", port)
		fmt.Printf("Model: %s, Backend: %s, Printer: %s\n", model, backend, printer)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	if err := svc.printer.Close(); err != nil {
		log.Printf("Error closing printer connection: %v\n", err)
	} else {
		log.Println("Printer connection closed")
	}

	log.Println("Server exiting")
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


