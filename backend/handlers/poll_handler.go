package handlers

import (
	"net/http"
	"strings"

	"live-polling-tool/middleware"
	"live-polling-tool/models"
	"live-polling-tool/services"
	"live-polling-tool/utils"

	"github.com/gin-gonic/gin"
)

type PollHandler struct {
	pollService *services.PollService
	authService *services.AuthService
	wsHub       *services.WSHub
}

func NewPollHandler(pollService *services.PollService, authService *services.AuthService, wsHub *services.WSHub) *PollHandler {
	return &PollHandler{
		pollService: pollService,
		authService: authService,
		wsHub:       wsHub,
	}
}

func (h *PollHandler) CreatePoll(c *gin.Context) {
	userIDVal, exists := c.Get(middleware.CtxUserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID := userIDVal.(string)

	var req models.CreatePollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body format"})
		return
	}

	cleanQuestion, cleanOptions, err := utils.ValidatePollInput(req.Question, req.Options, req.ClosesAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	poll, err := h.pollService.CreatePoll(c.Request.Context(), userID, cleanQuestion, cleanOptions, req.ClosesAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create poll"})
		return
	}

	c.JSON(http.StatusCreated, poll)
}

func (h *PollHandler) GetMyPolls(c *gin.Context) {
	userIDVal, exists := c.Get(middleware.CtxUserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID := userIDVal.(string)

	polls, err := h.pollService.ListUserPolls(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve polls"})
		return
	}

	c.JSON(http.StatusOK, polls)
}

func (h *PollHandler) GetPoll(c *gin.Context) {
	pollID := c.Param("id")
	if strings.TrimSpace(pollID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "poll id is required"})
		return
	}

	currentUserID := ""
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		tokenString := strings.TrimSpace(authHeader[7:])
		claims, err := h.authService.ValidateToken(tokenString)
		if err == nil && claims != nil {
			currentUserID = claims.UserID
		}
	}

	poll, err := h.pollService.GetPollByID(c.Request.Context(), pollID, currentUserID)
	if err != nil {
		if err.Error() == "poll not found" || err.Error() == "invalid poll id format" {
			c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve poll"})
		return
	}

	c.JSON(http.StatusOK, poll)
}

func (h *PollHandler) ClosePoll(c *gin.Context) {
	userIDVal, exists := c.Get(middleware.CtxUserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID := userIDVal.(string)

	pollID := c.Param("id")
	if strings.TrimSpace(pollID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "poll id is required"})
		return
	}

	poll, err := h.pollService.ClosePoll(c.Request.Context(), userID, pollID)
	if err != nil {
		if err.Error() == "poll not found" || err.Error() == "invalid poll id format" {
			c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
			return
		}
		if err.Error() == "you do not have permission to close this poll" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to close poll"})
		return
	}

	if h.wsHub != nil {
		h.wsHub.BroadcastPollClosed(pollID)
	}

	c.JSON(http.StatusOK, poll)
}

func (h *PollHandler) DeletePoll(c *gin.Context) {
	userIDVal, exists := c.Get(middleware.CtxUserIDKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID := userIDVal.(string)

	pollID := c.Param("id")
	if strings.TrimSpace(pollID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "poll id is required"})
		return
	}

	err := h.pollService.DeletePoll(c.Request.Context(), userID, pollID)
	if err != nil {
		if err.Error() == "poll not found" || err.Error() == "invalid poll id format" {
			c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
			return
		}
		if err.Error() == "you do not have permission to delete this poll" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete poll"})
		return
	}

	if h.wsHub != nil {
		h.wsHub.BroadcastPollDeleted(pollID)
	}

	c.JSON(http.StatusOK, gin.H{"message": "poll deleted successfully"})
}
