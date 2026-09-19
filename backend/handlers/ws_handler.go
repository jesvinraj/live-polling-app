package handlers

import (
	"log"
	"net/http"
	"strings"

	"live-polling-tool/config"
	"live-polling-tool/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WSHandler struct {
	hub      *services.WSHub
	upgrader websocket.Upgrader
}

func NewWSHandler(hub *services.WSHub, cfg *config.Config) *WSHandler {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			for _, allowed := range cfg.AllowedOrigins {
				if allowed == "*" || strings.EqualFold(allowed, origin) {
					return true
				}
			}
			return false
		},
	}

	return &WSHandler{
		hub:      hub,
		upgrader: upgrader,
	}
}

func (h *WSHandler) HandlePollWS(c *gin.Context) {
	pollID := strings.TrimSpace(c.Param("id"))
	if pollID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "poll id is required"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("failed to upgrade websocket connection: %v", err)
		return
	}

	clientIP := c.ClientIP()
	services.ServeWS(h.hub, conn, pollID, clientIP)
}
