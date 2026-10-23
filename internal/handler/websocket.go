package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/Lockok/web-msg/internal/auth"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	wsAuthTimeout    = 5 * time.Second
	wsWriteTimeout   = 5 * time.Second
	wsMaxMessageSize = 4 * 1024
)

type WebSocketHandler struct {
	jwtManager *auth.JWTManager
}

func NewWebSocketHandler(jwtManager *auth.JWTManager) *WebSocketHandler {
	return &WebSocketHandler{
		jwtManager: jwtManager,
	}
}

type wsAuthRequest struct {
	Type        string `json:"type"`
	AccessToken string `json:"access_token"`
}

type wsAuthResponse struct {
	Type string `json:"type"`
}

func (h *WebSocketHandler) Connect(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()

	conn.SetReadLimit(wsMaxMessageSize)

	authCtx, cancelAuth := context.WithTimeout(r.Context(), wsAuthTimeout)

	var request wsAuthRequest
	err = wsjson.Read(authCtx, conn, &request)
	cancelAuth()

	if err != nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "authentication required")
		return
	}

	if request.Type != "auth" {
		_ = conn.Close(websocket.StatusPolicyViolation, "first message must be auth")
		return
	}

	_, expiresAt, err := h.jwtManager.ValidateWithExpiry(request.AccessToken)
	if err != nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "invalid access token")
		return
	}

	writeCtx, cancelWrite := context.WithTimeout(r.Context(), wsWriteTimeout)

	err = wsjson.Write(writeCtx, conn, wsAuthResponse{
		Type: "auth.ok",
	})
	cancelWrite()

	if err != nil {
		return
	}

	timer := time.NewTimer(time.Until(expiresAt))
	defer timer.Stop()

	ctx := conn.CloseRead(r.Context())

	select {
	case <-ctx.Done():
		return // клиент отключился или соеидинение завершилось.

	case <-timer.C:
		_ = conn.Close(websocket.StatusPolicyViolation, "access token expired")
		return
	}
}
