package services

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"live-polling-tool/config"
	"live-polling-tool/database"
	"live-polling-tool/models"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestConcurrentVotingDeduplication(t *testing.T) {
	cfg := config.LoadConfig()
	cfg.MongoDBName = "polling_app_test"

	mongoInst, err := database.ConnectMongoDB(cfg)
	if err != nil {
		t.Skipf("skipping live database test: mongodb not available: %v", err)
		return
	}
	defer func() {
		_ = mongoInst.Database.Drop(context.Background())
		_ = mongoInst.Client.Disconnect(context.Background())
	}()

	redisInst, err := database.ConnectRedis(cfg)
	if err != nil {
		t.Skipf("skipping live database test: redis not available: %v", err)
		return
	}
	defer redisInst.Client.Close()

	ctx := context.Background()

	// Clean up any existing test keys
	_ = redisInst.Client.FlushDB(ctx).Err()

	voteService := NewVoteService(mongoInst, redisInst)

	// Create test poll
	pollObjID := primitive.NewObjectID()
	ownerObjID := primitive.NewObjectID()
	optA := "opt_a"
	optB := "opt_b"

	poll := models.Poll{
		ID:       pollObjID,
		OwnerID:  ownerObjID,
		Question: "Concurrency Stress Test Poll",
		Options: []models.PollOption{
			{ID: optA, Text: "Option A", VoteCount: 0},
			{ID: optB, Text: "Option B", VoteCount: 0},
		},
		IsClosed:  false,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	_, err = mongoInst.Polls.InsertOne(ctx, poll)
	if err != nil {
		t.Fatalf("failed to insert test poll: %v", err)
	}

	// Seed Redis hash
	redisKey := fmt.Sprintf("poll:%s:votes", pollObjID.Hex())
	redisInst.Client.HSet(ctx, redisKey, optA, 0, optB, 0)

	var (
		wg                  sync.WaitGroup
		acceptedDistinct    int64
		acceptedDuplicate   int64
		rejectedDuplicate   int64
	)

	// 1. Launch 50 concurrent votes with 50 UNIQUE voter tokens for Option A
	numDistinct := 50
	for i := 0; i < numDistinct; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			uniqueToken := uuid.New().String()
			_, vErr := voteService.CastVote(context.Background(), pollObjID.Hex(), optA, uniqueToken)
			if vErr == nil {
				atomic.AddInt64(&acceptedDistinct, 1)
			} else {
				t.Errorf("unexpected error on unique vote: %v", vErr)
			}
		}()
	}

	// 2. Launch 20 concurrent votes with the SAME duplicate voter token for Option B
	sameToken := uuid.New().String()
	numDuplicates := 20
	for i := 0; i < numDuplicates; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, vErr := voteService.CastVote(context.Background(), pollObjID.Hex(), optB, sameToken)
			if vErr == nil {
				atomic.AddInt64(&acceptedDuplicate, 1)
			} else if vErr == ErrAlreadyVoted {
				atomic.AddInt64(&rejectedDuplicate, 1)
			} else {
				t.Errorf("unexpected error on duplicate vote attempt: %v", vErr)
			}
		}()
	}

	wg.Wait()

	// Verify acceptance counts
	if acceptedDistinct != int64(numDistinct) {
		t.Errorf("expected %d distinct votes accepted, got %d", numDistinct, acceptedDistinct)
	}
	if acceptedDuplicate != 1 {
		t.Errorf("expected exactly 1 duplicate vote accepted, got %d", acceptedDuplicate)
	}
	if rejectedDuplicate != int64(numDuplicates-1) {
		t.Errorf("expected %d duplicate votes rejected with ErrAlreadyVoted, got %d", numDuplicates-1, rejectedDuplicate)
	}

	// Verify Redis counts
	votesMap, err := redisInst.Client.HGetAll(ctx, redisKey).Result()
	if err != nil {
		t.Fatalf("failed to read redis counts: %v", err)
	}

	var redisOptA, redisOptB int64
	fmt.Sscanf(votesMap[optA], "%d", &redisOptA)
	fmt.Sscanf(votesMap[optB], "%d", &redisOptB)

	if redisOptA != 50 {
		t.Errorf("expected Redis Option A count = 50, got %d", redisOptA)
	}
	if redisOptB != 1 {
		t.Errorf("expected Redis Option B count = 1, got %d", redisOptB)
	}

	// Verify MongoDB Poll Document option tallies
	var updatedPoll models.Poll
	err = mongoInst.Polls.FindOne(ctx, bson.M{"_id": pollObjID}).Decode(&updatedPoll)
	if err != nil {
		t.Fatalf("failed to read updated poll from mongo: %v", err)
	}

	for _, opt := range updatedPoll.Options {
		if opt.ID == optA && opt.VoteCount != 50 {
			t.Errorf("expected Mongo Option A count = 50, got %d", opt.VoteCount)
		}
		if opt.ID == optB && opt.VoteCount != 1 {
			t.Errorf("expected Mongo Option B count = 1, got %d", opt.VoteCount)
		}
	}

	// Verify MongoDB Votes Collection Audit Document Count
	auditCount, err := mongoInst.Votes.CountDocuments(ctx, bson.M{"poll_id": pollObjID})
	if err != nil {
		t.Fatalf("failed to count mongo votes documents: %v", err)
	}
	if auditCount != 51 {
		t.Errorf("expected exactly 51 documents in mongo votes collection, got %d", auditCount)
	}
}

func TestCrashRecoveryAndOrphanedKeyCleanup(t *testing.T) {
	cfg := config.LoadConfig()
	cfg.MongoDBName = "polling_app_test"

	mongoInst, err := database.ConnectMongoDB(cfg)
	if err != nil {
		t.Skipf("skipping live database test: mongodb not available: %v", err)
		return
	}
	defer func() {
		_ = mongoInst.Database.Drop(context.Background())
		_ = mongoInst.Client.Disconnect(context.Background())
	}()

	redisInst, err := database.ConnectRedis(cfg)
	if err != nil {
		t.Skipf("skipping live database test: redis not available: %v", err)
		return
	}
	defer redisInst.Client.Close()

	ctx := context.Background()
	_ = redisInst.Client.FlushDB(ctx).Err()

	pollService := NewPollService(mongoInst, redisInst)
	voteService := NewVoteService(mongoInst, redisInst)
	wsHub := NewWSHub(mongoInst, redisInst, pollService)
	defer wsHub.Shutdown()

	// 1. Create test poll with 0 votes
	pollObjID := primitive.NewObjectID()
	ownerObjID := primitive.NewObjectID()
	optA := "opt_a"
	optB := "opt_b"

	poll := models.Poll{
		ID:       pollObjID,
		OwnerID:  ownerObjID,
		Question: "Crash Recovery Test Poll",
		Options: []models.PollOption{
			{ID: optA, Text: "Option A", VoteCount: 0},
			{ID: optB, Text: "Option B", VoteCount: 0},
		},
		IsClosed:  false,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	_, err = mongoInst.Polls.InsertOne(ctx, poll)
	if err != nil {
		t.Fatalf("failed to insert test poll: %v", err)
	}

	pollIDStr := pollObjID.Hex()
	redisVotesKey := fmt.Sprintf("poll:%s:votes", pollIDStr)
	orphanToken := uuid.New().String()
	voterKey := fmt.Sprintf("voter:%s:%s", pollIDStr, orphanToken)
	lastVoteKey := fmt.Sprintf("poll:%s:last_vote_ts", pollIDStr)

	// 2. Simulate server crash between Redis step and MongoDB step:
	// - Redis counted the vote (Option A = 1)
	// - Redis marked voter key
	// - MongoDB has ZERO records in votes collection and VoteCount 0 in poll document
	redisInst.Client.HSet(ctx, redisVotesKey, optA, 1, optB, 0)
	redisInst.Client.Set(ctx, voterKey, "1", 30*24*time.Hour)

	// Set last_vote_ts to 75 seconds in the past (past the 60-second quiet period)
	pastTimestamp := time.Now().Unix() - 75
	redisInst.Client.Set(ctx, lastVoteKey, pastTimestamp, 10*time.Minute)

	// 3. Run reconciliation
	repaired, rErr := wsHub.ReconcilePoll(ctx, pollIDStr)
	if rErr != nil {
		t.Fatalf("reconciliation failed: %v", rErr)
	}
	if !repaired {
		t.Fatalf("expected reconciliation to perform repair on mismatched counts")
	}

	// 4. Assert Redis counts were repaired back to verified MongoDB count (0)
	votesMap, err := redisInst.Client.HGetAll(ctx, redisVotesKey).Result()
	if err != nil {
		t.Fatalf("failed to read redis counts: %v", err)
	}

	var redisOptA int64
	fmt.Sscanf(votesMap[optA], "%d", &redisOptA)
	if redisOptA != 0 {
		t.Errorf("expected Redis Option A count repaired to 0, got %d", redisOptA)
	}

	// 5. Assert orphaned Redis voter key was deleted
	exists, err := redisInst.Client.Exists(ctx, voterKey).Result()
	if err != nil {
		t.Fatalf("failed to check voter key existence: %v", err)
	}
	if exists != 0 {
		t.Errorf("expected orphaned voter key to be deleted, but it still exists in Redis")
	}

	// 6. Verify that the voter can now successfully cast a vote without being blocked
	res, vErr := voteService.CastVote(ctx, pollIDStr, optA, orphanToken)
	if vErr != nil {
		t.Fatalf("voter should be able to vote after crash recovery, got error: %v", vErr)
	}
	if res.Votes[optA] != 1 {
		t.Errorf("expected Option A count = 1 after vote, got %d", res.Votes[optA])
	}

	// Verify MongoDB now contains the audit record
	mongoCount, err := mongoInst.Votes.CountDocuments(ctx, bson.M{
		"poll_id":   pollObjID,
		"voter_key": orphanToken,
	})
	if err != nil || mongoCount != 1 {
		t.Errorf("expected 1 audit vote in mongo for voter, got %d (err: %v)", mongoCount, err)
	}
}

