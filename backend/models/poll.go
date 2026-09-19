package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PollOption struct {
	ID        string `bson:"id" json:"id"`
	Text      string `bson:"text" json:"text"`
	VoteCount int64  `bson:"vote_count" json:"voteCount"`
}

type Poll struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	OwnerID   primitive.ObjectID `bson:"owner_id" json:"ownerId"`
	Question  string             `bson:"question" json:"question"`
	Options   []PollOption       `bson:"options" json:"options"`
	IsClosed  bool               `bson:"is_closed" json:"isClosed"`
	ClosesAt  *time.Time         `bson:"closes_at,omitempty" json:"closesAt,omitempty"`
	CreatedAt time.Time          `bson:"created_at" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updatedAt"`
}

type CreatePollRequest struct {
	Question string     `json:"question"`
	Options  []string   `json:"options"`
	ClosesAt *time.Time `json:"closesAt,omitempty"`
}

type VoteRequest struct {
	OptionID   string `json:"optionId"`
	VoterToken string `json:"voterToken"`
}

type VoteResultResponse struct {
	PollID     string           `json:"pollId"`
	OptionID   string           `json:"optionId"`
	Votes      map[string]int64 `json:"votes"`
	TotalVotes int64            `json:"totalVotes"`
	IsClosed   bool             `json:"isClosed"`
}

type PollResponse struct {
	ID         string       `json:"id"`
	Question   string       `json:"question"`
	Options    []PollOption `json:"options"`
	TotalVotes int64        `json:"totalVotes"`
	IsClosed   bool         `json:"isClosed"`
	ClosesAt   *time.Time   `json:"closesAt,omitempty"`
	CreatedAt  time.Time    `json:"createdAt"`
	UpdatedAt  time.Time    `json:"updatedAt"`
	IsOwner    bool         `json:"isOwner"`
}

type PollSummaryResponse struct {
	ID          string     `json:"id"`
	Question    string     `json:"question"`
	OptionCount int        `json:"optionCount"`
	TotalVotes  int64      `json:"totalVotes"`
	IsClosed    bool       `json:"isClosed"`
	ClosesAt    *time.Time `json:"closesAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

func (p *Poll) TotalVotes() int64 {
	var sum int64
	for _, opt := range p.Options {
		sum += opt.VoteCount
	}
	return sum
}

func (p *Poll) ToResponse(currentUserID string) PollResponse {
	isOwner := currentUserID != "" && p.OwnerID.Hex() == currentUserID
	return PollResponse{
		ID:         p.ID.Hex(),
		Question:   p.Question,
		Options:    p.Options,
		TotalVotes: p.TotalVotes(),
		IsClosed:   p.IsClosedNow(),
		ClosesAt:   p.ClosesAt,
		CreatedAt:  p.CreatedAt,
		UpdatedAt:  p.UpdatedAt,
		IsOwner:    isOwner,
	}
}

func (p *Poll) ToSummary() PollSummaryResponse {
	return PollSummaryResponse{
		ID:          p.ID.Hex(),
		Question:    p.Question,
		OptionCount: len(p.Options),
		TotalVotes:  p.TotalVotes(),
		IsClosed:    p.IsClosedNow(),
		ClosesAt:    p.ClosesAt,
		CreatedAt:   p.CreatedAt,
	}
}

func (p *Poll) IsClosedNow() bool {
	if p.IsClosed {
		return true
	}
	if p.ClosesAt != nil && time.Now().UTC().After(*p.ClosesAt) {
		return true
	}
	return false
}
