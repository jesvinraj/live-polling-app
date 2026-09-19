package services

import (
	"testing"
	"time"

	"live-polling-tool/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestPollModelMethods(t *testing.T) {
	ownerID := primitive.NewObjectID()
	pollID := primitive.NewObjectID()

	options := []models.PollOption{
		{ID: "opt_1", Text: "Go", VoteCount: 15},
		{ID: "opt_2", Text: "Rust", VoteCount: 25},
		{ID: "opt_3", Text: "TypeScript", VoteCount: 10},
	}

	futureTime := time.Now().UTC().Add(24 * time.Hour)

	poll := models.Poll{
		ID:        pollID,
		OwnerID:   ownerID,
		Question:  "What language do you prefer?",
		Options:   options,
		IsClosed:  false,
		ClosesAt:  &futureTime,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	// Test TotalVotes
	if poll.TotalVotes() != 50 {
		t.Errorf("expected total votes to be 50, got %d", poll.TotalVotes())
	}

	// Test IsClosedNow with future expiration
	if poll.IsClosedNow() {
		t.Errorf("expected poll not to be closed with future expiration")
	}

	// Test IsClosedNow with past expiration
	pastTime := time.Now().UTC().Add(-1 * time.Hour)
	pollExpired := poll
	pollExpired.ClosesAt = &pastTime
	if !pollExpired.IsClosedNow() {
		t.Errorf("expected poll with past closesAt to be closed")
	}

	// Test IsClosedNow with explicit isClosed = true
	pollManuallyClosed := poll
	pollManuallyClosed.IsClosed = true
	if !pollManuallyClosed.IsClosedNow() {
		t.Errorf("expected poll with IsClosed=true to be closed")
	}

	// Test ToResponse
	respOwner := poll.ToResponse(ownerID.Hex())
	if !respOwner.IsOwner {
		t.Errorf("expected isOwner=true for poll owner")
	}
	if respOwner.TotalVotes != 50 {
		t.Errorf("expected total votes to be 50, got %d", respOwner.TotalVotes)
	}

	respPublic := poll.ToResponse("different_user_id")
	if respPublic.IsOwner {
		t.Errorf("expected isOwner=false for non-owner")
	}

	// Test ToSummary
	summary := poll.ToSummary()
	if summary.ID != pollID.Hex() {
		t.Errorf("expected summary ID %s, got %s", pollID.Hex(), summary.ID)
	}
	if summary.OptionCount != 3 {
		t.Errorf("expected summary option count 3, got %d", summary.OptionCount)
	}
	if summary.TotalVotes != 50 {
		t.Errorf("expected summary total votes 50, got %d", summary.TotalVotes)
	}
}
