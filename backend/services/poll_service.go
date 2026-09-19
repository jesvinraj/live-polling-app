package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"live-polling-tool/database"
	"live-polling-tool/models"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type PollService struct {
	db        *database.MongoInstance
	redisInst *database.RedisInstance
}

func NewPollService(db *database.MongoInstance, redisInst *database.RedisInstance) *PollService {
	return &PollService{
		db:        db,
		redisInst: redisInst,
	}
}

func (s *PollService) CreatePoll(ctx context.Context, ownerIDStr string, question string, optionsList []string, closesAt *time.Time) (*models.PollResponse, error) {
	ownerObjID, err := primitive.ObjectIDFromHex(ownerIDStr)
	if err != nil {
		return nil, errors.New("invalid owner id")
	}

	var pollOptions []models.PollOption
	for _, optText := range optionsList {
		pollOptions = append(pollOptions, models.PollOption{
			ID:        uuid.New().String(),
			Text:      optText,
			VoteCount: 0,
		})
	}

	now := time.Now().UTC()
	poll := models.Poll{
		ID:        primitive.NewObjectID(),
		OwnerID:   ownerObjID,
		Question:  question,
		Options:   pollOptions,
		IsClosed:  false,
		ClosesAt:  closesAt,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err = s.db.Polls.InsertOne(ctx, poll)
	if err != nil {
		return nil, fmt.Errorf("failed to save poll to database: %w", err)
	}

	// Seed Redis hash with initial vote counts (0 for each option)
	if s.redisInst != nil && s.redisInst.Client != nil {
		redisKey := fmt.Sprintf("poll:%s:votes", poll.ID.Hex())
		pipe := s.redisInst.Client.Pipeline()
		for _, opt := range pollOptions {
			pipe.HSet(ctx, redisKey, opt.ID, 0)
		}
		// Set TTL of 30 days for poll votes hash
		pipe.Expire(ctx, redisKey, 30*24*time.Hour)
		_, _ = pipe.Exec(ctx)
	}

	resp := poll.ToResponse(ownerIDStr)
	return &resp, nil
}

func (s *PollService) GetPollByID(ctx context.Context, pollIDStr string, currentUserID string) (*models.PollResponse, error) {
	pollObjID, err := primitive.ObjectIDFromHex(pollIDStr)
	if err != nil {
		return nil, errors.New("invalid poll id format")
	}

	var poll models.Poll
	err = s.db.Polls.FindOne(ctx, bson.M{"_id": pollObjID}).Decode(&poll)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("poll not found")
		}
		return nil, fmt.Errorf("database query error: %w", err)
	}

	// Check if poll has auto-expired
	if !poll.IsClosed && poll.ClosesAt != nil && time.Now().UTC().After(*poll.ClosesAt) {
		poll.IsClosed = true
		_, _ = s.db.Polls.UpdateOne(ctx, bson.M{"_id": pollObjID}, bson.M{
			"$set": bson.M{"is_closed": true, "updated_at": time.Now().UTC()},
		})
	}

	// Synchronize vote counts from Redis if available
	if s.redisInst != nil && s.redisInst.Client != nil {
		redisKey := fmt.Sprintf("poll:%s:votes", poll.ID.Hex())
		votesMap, err := s.redisInst.Client.HGetAll(ctx, redisKey).Result()
		if err == nil && len(votesMap) > 0 {
			for i, opt := range poll.Options {
				if valStr, exists := votesMap[opt.ID]; exists {
					var count int64
					fmt.Sscanf(valStr, "%d", &count)
					poll.Options[i].VoteCount = count
				}
			}
		} else if len(votesMap) == 0 {
			// Redis cache miss: rebuild Redis cache from MongoDB state
			pipe := s.redisInst.Client.Pipeline()
			for _, opt := range poll.Options {
				pipe.HSet(ctx, redisKey, opt.ID, opt.VoteCount)
			}
			pipe.Expire(ctx, redisKey, 30*24*time.Hour)
			_, _ = pipe.Exec(ctx)
		}
	}

	resp := poll.ToResponse(currentUserID)
	return &resp, nil
}

func (s *PollService) ListUserPolls(ctx context.Context, ownerIDStr string) ([]models.PollSummaryResponse, error) {
	ownerObjID, err := primitive.ObjectIDFromHex(ownerIDStr)
	if err != nil {
		return nil, errors.New("invalid owner id")
	}

	findOptions := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := s.db.Polls.Find(ctx, bson.M{"owner_id": ownerObjID}, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user polls: %w", err)
	}
	defer cursor.Close(ctx)

	var summaries []models.PollSummaryResponse
	for cursor.Next(ctx) {
		var poll models.Poll
		if err := cursor.Decode(&poll); err != nil {
			continue
		}
		summaries = append(summaries, poll.ToSummary())
	}

	if summaries == nil {
		summaries = []models.PollSummaryResponse{}
	}

	return summaries, nil
}

func (s *PollService) ClosePoll(ctx context.Context, ownerIDStr string, pollIDStr string) (*models.PollResponse, error) {
	ownerObjID, err := primitive.ObjectIDFromHex(ownerIDStr)
	if err != nil {
		return nil, errors.New("invalid owner id")
	}

	pollObjID, err := primitive.ObjectIDFromHex(pollIDStr)
	if err != nil {
		return nil, errors.New("invalid poll id format")
	}

	var poll models.Poll
	err = s.db.Polls.FindOne(ctx, bson.M{"_id": pollObjID}).Decode(&poll)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("poll not found")
		}
		return nil, fmt.Errorf("database query error: %w", err)
	}

	if poll.OwnerID != ownerObjID {
		return nil, errors.New("you do not have permission to close this poll")
	}

	if poll.IsClosed {
		resp := poll.ToResponse(ownerIDStr)
		return &resp, nil
	}

	now := time.Now().UTC()
	_, err = s.db.Polls.UpdateOne(ctx, bson.M{"_id": pollObjID}, bson.M{
		"$set": bson.M{
			"is_closed":  true,
			"updated_at": now,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to close poll: %w", err)
	}

	poll.IsClosed = true
	poll.UpdatedAt = now

	resp := poll.ToResponse(ownerIDStr)
	return &resp, nil
}

func (s *PollService) DeletePoll(ctx context.Context, ownerIDStr string, pollIDStr string) error {
	ownerObjID, err := primitive.ObjectIDFromHex(ownerIDStr)
	if err != nil {
		return errors.New("invalid owner id")
	}

	pollObjID, err := primitive.ObjectIDFromHex(pollIDStr)
	if err != nil {
		return errors.New("invalid poll id format")
	}

	var poll models.Poll
	err = s.db.Polls.FindOne(ctx, bson.M{"_id": pollObjID}).Decode(&poll)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("poll not found")
		}
		return fmt.Errorf("database query error: %w", err)
	}

	if poll.OwnerID != ownerObjID {
		return errors.New("you do not have permission to delete this poll")
	}

	// Delete from MongoDB polls collection
	_, err = s.db.Polls.DeleteOne(ctx, bson.M{"_id": pollObjID})
	if err != nil {
		return fmt.Errorf("failed to delete poll: %w", err)
	}

	// Clean up related votes collection
	_, _ = s.db.Votes.DeleteMany(ctx, bson.M{"poll_id": pollObjID})

	// Clean up Redis keys
	if s.redisInst != nil && s.redisInst.Client != nil {
		pipe := s.redisInst.Client.Pipeline()
		pipe.Del(ctx, fmt.Sprintf("poll:%s:votes", pollIDStr))
		pipe.Del(ctx, fmt.Sprintf("poll:%s:last_vote_ts", pollIDStr))
		pipe.Del(ctx, fmt.Sprintf("poll:%s:scan_cursor", pollIDStr))
		pipe.Del(ctx, fmt.Sprintf("poll:%s:viewers_zset", pollIDStr))
		_, _ = pipe.Exec(ctx)
	}

	return nil
}
