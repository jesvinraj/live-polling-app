package handlers

import (
	"errors"
	"net/http"
	"strings"

	"live-polling-tool/models"
	"live-polling-tool/services"
	"live-polling-tool/utils"

	"github.com/gin-gonic/gin"
)

type VoteHandler struct {
	voteService *services.VoteService
}

func NewVoteHandler(voteService *services.VoteService) *VoteHandler {
	return &VoteHandler{
		voteService: voteService,
	}
}

func (h *VoteHandler) SubmitVote(c *gin.Context) {
	pollID := strings.TrimSpace(c.Param("id"))
	if pollID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "poll id is required"})
		return
	}

	var req models.VoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body format"})
		return
	}

	req.OptionID = strings.TrimSpace(req.OptionID)
	req.VoterToken = strings.TrimSpace(req.VoterToken)

	if req.OptionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "option id is required"})
		return
	}

	cleanToken, err := utils.ValidateVoterToken(req.VoterToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.voteService.CastVote(c.Request.Context(), pollID, req.OptionID, cleanToken)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrPollNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		case errors.Is(err, services.ErrPollClosed):
			c.JSON(http.StatusBadRequest, gin.H{"error": "this poll is closed and no longer accepting votes"})
		case errors.Is(err, services.ErrInvalidOption):
			c.JSON(http.StatusBadRequest, gin.H{"error": "the selected option does not belong to this poll"})
		case errors.Is(err, services.ErrAlreadyVoted):
			c.JSON(http.StatusConflict, gin.H{"error": "you have already voted on this poll"})
		case errors.Is(err, services.ErrInvalidToken):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid voter token"})
		case errors.Is(err, services.ErrInvalidOptionID):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid option id"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record vote"})
		}
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *VoteHandler) CheckVoteStatus(c *gin.Context) {
	pollID := strings.TrimSpace(c.Param("id"))
	if pollID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "poll id is required"})
		return
	}

	voterToken := strings.TrimSpace(c.Query("voterToken"))
	if voterToken == "" {
		c.JSON(http.StatusOK, gin.H{"hasVoted": false})
		return
	}

	hasVoted, err := h.voteService.HasUserVoted(c.Request.Context(), pollID, voterToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check vote status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"hasVoted": hasVoted})
}
