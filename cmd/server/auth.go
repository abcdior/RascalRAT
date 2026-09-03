package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v5"
)

func initAuthRoutes(e *echo.Echo) {
	e.POST("/validate_token", handleValidateToken)
	e.POST("/logout", handleLogout)
}

func applyAuthMiddleware(e *echo.Echo) {
	public := []string{
		"/validate_token",
		"/logout",
		"/static/",
		"/favicon.ico",
	}
	protected := []string{
		"/",
		"/status",
		"/nodes",
		"/nodes/task",
		"/ws/connect",
	}

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			p := c.Path()
			for _, prefix := range public {
				if p == prefix || strings.HasPrefix(p, prefix) {
					return next(c)
				}
			}
			for _, prefix := range protected {
				if p == prefix || strings.HasPrefix(p, prefix) {
					if !isAuthenticated(c) {
						if strings.HasPrefix(c.Path(), "/api/") {
							return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
						}
						return c.Render(http.StatusOK, "index.html", map[string]any{
							"Title":         "RascalRAT Console",
							"Unauthorized": true,
						})
					}
					return next(c)
				}
			}
			return next(c)
		}
	})
}

func isAuthenticated(c *echo.Context) bool {
	cookie, err := c.Cookie("rascalrat_token")
	if err != nil {
		return false
	}
	return cookie.Value != ""
}

func handleValidateToken(c *echo.Context) error {
	var req struct {
		Token     string `json:"token"`
		Tool      string `json:"tool"`
		RemoteURL string `json:"remote_url"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}
	if req.Token == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "token is required"})
	}

	tokenServerURL := os.Getenv("TOKEN_SERVER_URL")
	if tokenServerURL == "" {
		tokenServerURL = "http://localhost:8080"
	}
	tokenServerURL = strings.TrimRight(tokenServerURL, "/")

	resp, err := http.Post(
		tokenServerURL+"/api/v1/token/validate",
		"application/json",
		strings.NewReader(fmt.Sprintf(`{"token":"%s","tool":"RascalRAT","remote_url":"%s"}`, req.Token, req.RemoteURL)),
	)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "Validator unreachable"})
	}
	defer resp.Body.Close()

	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "Invalid validator response"})
	}

	validBool := false
	if v, ok := payload["valid"].(bool); ok {
		validBool = v
	}

	if resp.StatusCode != http.StatusOK || !validBool {
		reason := "Invalid or expired token"
		if errStr, ok := payload["error"].(string); ok && errStr != "" {
			reason = errStr
		}
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": reason})
	}

	c.SetCookie(&http.Cookie{
		Name:   "rascalrat_token",
		Value:  req.Token,
		MaxAge: 60 * 60 * 24 * 30,
		Path:   "/",
	})
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func handleLogout(c *echo.Context) error {
	c.SetCookie(&http.Cookie{
		Name:   "rascalrat_token",
		Value:  "",
		MaxAge: -1,
		Path:   "/",
	})
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
