package services

import (
	"context"
	"testing"

	"live-polling-tool/models"
)

func TestVoteValidationErrors(t *testing.T) {
	service := NewVoteService(nil, nil)
	ctx := context.Background()

	// Test empty voter token
	_, err := service.CastVote(ctx, "507f1f77bcf86cd799439011", "opt_1", "")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}

	// Test short voter token
	_, err = service.CastVote(ctx, "507f1f77bcf86cd799439011", "opt_1", "abc")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken for short token, got %v", err)
	}

	// Test empty option ID
	_, err = service.CastVote(ctx, "507f1f77bcf86cd799439011", "", "11111111-1111-4111-8111-111111111111")
	if err != ErrInvalidOptionID {
		t.Errorf("expected ErrInvalidOptionID, got %v", err)
	}

	// Test invalid poll hex ID
	_, err = service.CastVote(ctx, "invalid-hex-id", "opt_1", "11111111-1111-4111-8111-111111111111")
	if err != ErrPollNotFound {
		t.Errorf("expected ErrPollNotFound, got %v", err)
	}
}

func TestVoteResultResponseStructure(t *testing.T) {
	votesMap := map[string]int64{
		"opt_1": 12,
		"opt_2": 8,
	}

	resp := models.VoteResultResponse{
		PollID:     "507f1f77bcf86cd799439011",
		OptionID:   "opt_1",
		Votes:      votesMap,
		TotalVotes: 20,
		IsClosed:   false,
	}

	if resp.TotalVotes != 20 {
		t.Errorf("expected TotalVotes=20, got %d", resp.TotalVotes)
	}

	if resp.Votes["opt_1"] != 12 {
		t.Errorf("expected votes for opt_1=12, got %d", resp.Votes["opt_1"])
	}
}
