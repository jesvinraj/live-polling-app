package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"live-polling-tool/database"
	"live-polling-tool/models"
	"live-polling-tool/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrPollNotFound   = errors.New("poll not found")
	ErrPollClosed     = errors.New("this poll is closed and no longer accepting votes")
	ErrInvalidOption  = errors.New("the selected option does not belong to this poll")
	ErrAlreadyVoted   = errors.New("you have already voted on this poll")
	ErrInvalidToken   = errors.New("invalid voter token")
	ErrInvalidOptionID= errors.New("invalid option id")
)

// Redis Lua script for atomic voter deduplication, counter increment, and last_vote_ts recording
const recordVoteLuaScript = `
local voter_key = KEYS[1]
local poll_votes_key = KEYS[2]
local last_vote_key = KEYS[3]
local option_id = ARGV[1]
local ttl_seconds = tonumber(ARGV[2])
local now_ts = ARGV[3]

if redis.call("EXISTS", voter_key) == 1 then
    return -1
end

redis.call("SET", voter_key, "1", "EX", ttl_seconds)
local new_count = redis.call("HINCRBY", poll_votes_key, option_id, 1)
redis.call("SET", last_vote_key, now_ts, "EX", 600)
return new_count
`

// Redis Lua script to roll back vote if MongoDB persistence fails
const rollbackVoteLuaScript = `
local voter_key = KEYS[1]
local poll_votes_key = KEYS[2]
local option_id = ARGV[1]

redis.call("DEL", voter_key)
redis.call("HINCRBY", poll_votes_key, option_id, -1)
return 1
`

type VoteService struct {
	db        *database.MongoInstance
	redisInst *database.RedisInstance
}

func NewVoteService(db *database.MongoInstance, redisInst *database.RedisInstance) *VoteService {
	return &VoteService{
		db:        db,
		redisInst: redisInst,
	}
}

func (s *VoteService) CastVote(ctx context.Context, pollIDStr string, optionID string, rawVoterToken string) (*models.VoteResultResponse, error) {
	voterToken, err := utils.ValidateVoterToken(rawVoterToken)
	if err != nil {
		return nil, ErrInvalidToken
	}

	if optionID == "" {
		return nil, ErrInvalidOptionID
	}

	pollObjID, err := primitive.ObjectIDFromHex(pollIDStr)
	if err != nil {
		return nil, ErrPollNotFound
	}

	// 1. Fetch current poll from MongoDB to verify existence, options, and status
	var poll models.Poll
	err = s.db.Polls.FindOne(ctx, bson.M{"_id": pollObjID}).Decode(&poll)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrPollNotFound
		}
		return nil, fmt.Errorf("database query error: %w", err)
	}

	// Check if poll is closed or expired
	if poll.IsClosedNow() {
		return nil, ErrPollClosed
	}

	// Validate option exists in this poll
	validOption := false
	for _, opt := range poll.Options {
		if opt.ID == optionID {
			validOption = true
			break
		}
	}
	if !validOption {
		return nil, ErrInvalidOption
	}

	// 2. Redis Atomic Deduplication & Increment
	voterKey := fmt.Sprintf("voter:%s:%s", pollIDStr, voterToken)
	pollVotesKey := fmt.Sprintf("poll:%s:votes", pollIDStr)
	lastVoteKey := fmt.Sprintf("poll:%s:last_vote_ts", pollIDStr)
	ttlSeconds := int64(30 * 24 * 3600) // 30 days TTL
	now := time.Now().UTC()
	nowUnixStr := fmt.Sprintf("%d", now.Unix())

	if s.redisInst != nil && s.redisInst.Client != nil {
		// Ensure Redis hash exists; if missing, seed from MongoDB
		exists, _ := s.redisInst.Client.Exists(ctx, pollVotesKey).Result()
		if exists == 0 {
			pipe := s.redisInst.Client.Pipeline()
			for _, opt := range poll.Options {
				pipe.HSet(ctx, pollVotesKey, opt.ID, opt.VoteCount)
			}
			pipe.Expire(ctx, pollVotesKey, 30*24*time.Hour)
			_, _ = pipe.Exec(ctx)
		}

		res, err := s.redisInst.Client.Eval(ctx, recordVoteLuaScript, []string{voterKey, pollVotesKey, lastVoteKey}, optionID, ttlSeconds, nowUnixStr).Result()
		if err != nil {
			log.Printf("warning: Redis Lua script error: %v, falling back to database check", err)
		} else {
			countResult, ok := res.(int64)
			if ok && countResult == -1 {
				return nil, ErrAlreadyVoted
			}
		}
	}

	// 3. MongoDB Durable Persistence (Write-through)
	// Insert audit vote record with compound unique index (poll_id + voter_key)
	voteRecord := bson.M{
		"poll_id":    pollObjID,
		"option_id":  optionID,
		"voter_key":  voterToken,
		"created_at": now,
	}

	_, err = s.db.Votes.InsertOne(ctx, voteRecord)
	if err != nil {
		// If duplicate key error in MongoDB (already voted), roll back Redis and return conflict
		if mongo.IsDuplicateKeyError(err) {
			s.rollbackRedis(ctx, voterKey, pollVotesKey, optionID)
			return nil, ErrAlreadyVoted
		}

		// MongoDB write failure: perform Redis rollback compensation
		s.rollbackRedis(ctx, voterKey, pollVotesKey, optionID)
		return nil, fmt.Errorf("failed to persist vote to database: %w", err)
	}

	// Increment vote_count on the option inside MongoDB poll document
	filter := bson.M{
		"_id":        pollObjID,
		"options.id": optionID,
	}
	update := bson.M{
		"$inc": bson.M{"options.$.vote_count": 1},
		"$set": bson.M{"updated_at": now},
	}
	_, err = s.db.Polls.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Printf("warning: failed to increment option vote count in MongoDB poll doc: %v", err)
	}

	// 4. Retrieve updated vote totals across all options
	updatedVotes := make(map[string]int64)
	var totalVotes int64

	if s.redisInst != nil && s.redisInst.Client != nil {
		votesMap, rErr := s.redisInst.Client.HGetAll(ctx, pollVotesKey).Result()
		if rErr == nil && len(votesMap) > 0 {
			for _, opt := range poll.Options {
				var count int64
				if valStr, ok := votesMap[opt.ID]; ok {
					fmt.Sscanf(valStr, "%d", &count)
				}
				updatedVotes[opt.ID] = count
				totalVotes += count
			}
		}
	}

	if len(updatedVotes) == 0 {
		for _, opt := range poll.Options {
			cnt := opt.VoteCount
			if opt.ID == optionID {
				cnt++
			}
			updatedVotes[opt.ID] = cnt
			totalVotes += cnt
		}
	}

	// 5. Publish real-time event to Redis channel
	if s.redisInst != nil && s.redisInst.Client != nil {
		wsMsg := models.WSMessage{
			Type:       models.WSMsgVoteUpdate,
			PollID:     pollIDStr,
			OptionID:   optionID,
			Votes:      updatedVotes,
			TotalVotes: totalVotes,
			IsClosed:   poll.IsClosedNow(),
		}
		msgBytes, mErr := json.Marshal(wsMsg)
		if mErr == nil {
			channel := fmt.Sprintf("channel:poll:%s", pollIDStr)
			s.redisInst.Client.Publish(ctx, channel, string(msgBytes))
		}
	}

	return &models.VoteResultResponse{
		PollID:     pollIDStr,
		OptionID:   optionID,
		Votes:      updatedVotes,
		TotalVotes: totalVotes,
		IsClosed:   poll.IsClosedNow(),
	}, nil
}

func (s *VoteService) rollbackRedis(ctx context.Context, voterKey, pollVotesKey, optionID string) {
	if s.redisInst != nil && s.redisInst.Client != nil {
		_, _ = s.redisInst.Client.Eval(ctx, rollbackVoteLuaScript, []string{voterKey, pollVotesKey}, optionID).Result()
	}
}

func (s *VoteService) HasUserVoted(ctx context.Context, pollIDStr string, voterToken string) (bool, error) {
	if voterToken == "" {
		return false, nil
	}

	voterKey := fmt.Sprintf("voter:%s:%s", pollIDStr, voterToken)
	if s.redisInst != nil && s.redisInst.Client != nil {
		exists, err := s.redisInst.Client.Exists(ctx, voterKey).Result()
		if err == nil && exists == 1 {
			return true, nil
		}
	}

	pollObjID, err := primitive.ObjectIDFromHex(pollIDStr)
	if err != nil {
		return false, nil
	}

	var voteRecord bson.M
	err = s.db.Votes.FindOne(ctx, bson.M{
		"poll_id":   pollObjID,
		"voter_key": voterToken,
	}).Decode(&voteRecord)

	if err == nil {
		// Re-populate Redis key if it was missing
		if s.redisInst != nil && s.redisInst.Client != nil {
			s.redisInst.Client.Set(ctx, voterKey, "1", 30*24*time.Hour)
		}
		return true, nil
	}

	if err == mongo.ErrNoDocuments {
		return false, nil
	}

	return false, err
}
