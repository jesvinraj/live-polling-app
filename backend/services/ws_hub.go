package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"live-polling-tool/database"
	"live-polling-tool/models"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	writeWait        = 10 * time.Second
	pongWait         = 60 * time.Second
	pingPeriod       = (pongWait * 9) / 10
	maxMessageSize   = 1024 // 1 KB max incoming message size
	maxConnPerIP     = 10
	maxConnPerRoom   = 1000
	viewerTTLSeconds = 120 // 2 minutes (over 2 full ping cycles)
)

type WSClient struct {
	ID     string
	PollID string
	IP     string
	Conn   *websocket.Conn
	Send   chan []byte
	Hub    *WSHub
}

type PollRoom struct {
	PollID    string
	Clients   map[*WSClient]bool
	IPCounts  map[string]int
	CancelSub context.CancelFunc
}

type WSHub struct {
	sync.RWMutex
	rooms       map[string]*PollRoom
	register    chan *WSClient
	unregister  chan *WSClient
	broadcast   chan *models.WSMessage
	db          *database.MongoInstance
	redisInst   *database.RedisInstance
	pollService *PollService
	stopChan    chan struct{}
}

func NewWSHub(db *database.MongoInstance, redisInst *database.RedisInstance, pollService *PollService) *WSHub {
	hub := &WSHub{
		rooms:       make(map[string]*PollRoom),
		register:    make(chan *WSClient),
		unregister:  make(chan *WSClient),
		broadcast:   make(chan *models.WSMessage, 100),
		db:          db,
		redisInst:   redisInst,
		pollService: pollService,
		stopChan:    make(chan struct{}),
	}
	go hub.run()
	go hub.startBackgroundCron()
	return hub
}

func (h *WSHub) run() {
	for {
		select {
		case <-h.stopChan:
			h.cleanupAll()
			return

		case client := <-h.register:
			h.handleRegister(client)

		case client := <-h.unregister:
			h.handleUnregister(client)

		case msg := <-h.broadcast:
			h.handleBroadcast(msg)
		}
	}
}

func (h *WSHub) handleRegister(client *WSClient) {
	h.Lock()
	room, exists := h.rooms[client.PollID]
	if !exists {
		ctx, cancel := context.WithCancel(context.Background())
		room = &PollRoom{
			PollID:    client.PollID,
			Clients:   make(map[*WSClient]bool),
			IPCounts:  make(map[string]int),
			CancelSub: cancel,
		}
		h.rooms[client.PollID] = room
		// Start exactly ONE Redis Pub/Sub subscription for this poll room on this server
		go h.subscribeRedisPoll(ctx, client.PollID)
	}

	// Enforce connection limits per IP and per room
	if len(room.Clients) >= maxConnPerRoom || room.IPCounts[client.IP] >= maxConnPerIP {
		h.Unlock()
		closeMsg := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "connection limit exceeded")
		_ = client.Conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(writeWait))
		client.Conn.Close()
		return
	}

	room.Clients[client] = true
	room.IPCounts[client.IP]++
	localViewerCount := len(room.Clients)
	h.Unlock()

	// Update Redis viewer presence using timestamped Sorted Set
	viewerCount := h.addViewerPresence(client.PollID, client.ID, localViewerCount)

	// Send INIT_STATE immediately to the newly connected client
	go h.sendInitState(client, viewerCount)

	// Broadcast updated viewer count across all instances
	h.publishViewerCount(client.PollID, viewerCount)
}

func (h *WSHub) handleUnregister(client *WSClient) {
	h.Lock()
	room, exists := h.rooms[client.PollID]
	if !exists {
		h.Unlock()
		return
	}

	if _, ok := room.Clients[client]; ok {
		delete(room.Clients, client)
		room.IPCounts[client.IP]--
		if room.IPCounts[client.IP] <= 0 {
			delete(room.IPCounts, client.IP)
		}
		close(client.Send)
	}

	remainingLocal := len(room.Clients)
	if remainingLocal == 0 {
		// No clients left on this server for this poll: stop Redis Pub/Sub listener
		if room.CancelSub != nil {
			room.CancelSub()
		}
		delete(h.rooms, client.PollID)
	}
	h.Unlock()

	// Remove from Redis viewer presence
	viewerCount := h.removeViewerPresence(client.PollID, client.ID, remainingLocal)

	if remainingLocal > 0 {
		h.publishViewerCount(client.PollID, viewerCount)
	}
}

func (h *WSHub) handleBroadcast(msg *models.WSMessage) {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.RLock()
	room, exists := h.rooms[msg.PollID]
	if !exists {
		h.RUnlock()
		return
	}

	for client := range room.Clients {
		select {
		case client.Send <- msgBytes:
		default:
			close(client.Send)
			delete(room.Clients, client)
		}
	}
	h.RUnlock()
}

func (h *WSHub) addViewerPresence(pollID, clientID string, fallbackCount int) int {
	if h.redisInst == nil || h.redisInst.Client == nil {
		return fallbackCount
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := fmt.Sprintf("poll:%s:viewers_zset", pollID)
	now := time.Now().Unix()

	// Add timestamp score
	h.redisInst.Client.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: clientID})
	h.redisInst.Client.Expire(ctx, key, 5*time.Minute)

	// Purge entries older than viewerTTLSeconds
	cutoff := fmt.Sprintf("%d", now-viewerTTLSeconds)
	h.redisInst.Client.ZRemRangeByScore(ctx, key, "-inf", cutoff)

	card, err := h.redisInst.Client.ZCard(ctx, key).Result()
	if err == nil && card > 0 {
		return int(card)
	}
	return fallbackCount
}

func (h *WSHub) removeViewerPresence(pollID, clientID string, fallbackCount int) int {
	if h.redisInst == nil || h.redisInst.Client == nil {
		return fallbackCount
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := fmt.Sprintf("poll:%s:viewers_zset", pollID)
	now := time.Now().Unix()

	h.redisInst.Client.ZRem(ctx, key, clientID)
	cutoff := fmt.Sprintf("%d", now-viewerTTLSeconds)
	h.redisInst.Client.ZRemRangeByScore(ctx, key, "-inf", cutoff)

	card, err := h.redisInst.Client.ZCard(ctx, key).Result()
	if err == nil {
		return int(card)
	}
	return fallbackCount
}

func (h *WSHub) refreshViewerHeartbeat(pollID, clientID string) {
	if h.redisInst == nil || h.redisInst.Client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	key := fmt.Sprintf("poll:%s:viewers_zset", pollID)
	now := time.Now().Unix()
	h.redisInst.Client.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: clientID})
	h.redisInst.Client.Expire(ctx, key, 5*time.Minute)
}

func (h *WSHub) sendInitState(client *WSClient, viewerCount int) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	poll, err := h.pollService.GetPollByID(ctx, client.PollID, "")
	if err != nil {
		return
	}

	votesMap := make(map[string]int64)
	for _, opt := range poll.Options {
		votesMap[opt.ID] = opt.VoteCount
	}

	initMsg := models.WSMessage{
		Type:        models.WSMsgInitState,
		PollID:      poll.ID,
		Votes:       votesMap,
		TotalVotes:  poll.TotalVotes,
		ViewerCount: viewerCount,
		IsClosed:    poll.IsClosed,
	}

	msgBytes, err := json.Marshal(initMsg)
	if err == nil {
		select {
		case client.Send <- msgBytes:
		default:
		}
	}
}

func (h *WSHub) subscribeRedisPoll(ctx context.Context, pollID string) {
	if h.redisInst == nil || h.redisInst.Client == nil {
		return
	}

	channelName := fmt.Sprintf("channel:poll:%s", pollID)
	pubsub := h.redisInst.Client.Subscribe(ctx, channelName)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var wsMsg models.WSMessage
			if err := json.Unmarshal([]byte(msg.Payload), &wsMsg); err == nil {
				h.broadcast <- &wsMsg
			}
		}
	}
}

func (h *WSHub) publishViewerCount(pollID string, count int) {
	if h.redisInst != nil && h.redisInst.Client != nil {
		msg := models.WSMessage{
			Type:        models.WSMsgViewerUpdate,
			PollID:      pollID,
			ViewerCount: count,
		}
		msgBytes, err := json.Marshal(msg)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			h.redisInst.Client.Publish(ctx, fmt.Sprintf("channel:poll:%s", pollID), string(msgBytes))
		}
	}
}

func (h *WSHub) BroadcastPollClosed(pollID string) {
	if h.redisInst != nil && h.redisInst.Client != nil {
		msg := models.WSMessage{
			Type:     models.WSMsgPollClosed,
			PollID:   pollID,
			IsClosed: true,
		}
		msgBytes, err := json.Marshal(msg)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			h.redisInst.Client.Publish(ctx, fmt.Sprintf("channel:poll:%s", pollID), string(msgBytes))
		}
	}
}

func (h *WSHub) BroadcastPollDeleted(pollID string) {
	if h.redisInst != nil && h.redisInst.Client != nil {
		msg := models.WSMessage{
			Type:      models.WSMsgPollDeleted,
			PollID:    pollID,
			IsDeleted: true,
		}
		msgBytes, err := json.Marshal(msg)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			h.redisInst.Client.Publish(ctx, fmt.Sprintf("channel:poll:%s", pollID), string(msgBytes))
		}
	}
}

// Background cron for automatic poll expiration checks, viewer presence pruning, and counts reconciliation
func (h *WSHub) startBackgroundCron() {
	autoCloseTicker := time.NewTicker(10 * time.Second)
	viewerCleanupTicker := time.NewTicker(60 * time.Second)
	reconcileTicker := time.NewTicker(5 * time.Minute)
	defer autoCloseTicker.Stop()
	defer viewerCleanupTicker.Stop()
	defer reconcileTicker.Stop()

	for {
		select {
		case <-h.stopChan:
			return

		case <-autoCloseTicker.C:
			h.checkAutoExpiredPolls()

		case <-viewerCleanupTicker.C:
			h.recountActiveViewers()

		case <-reconcileTicker.C:
			h.reconcileActivePollCounts()
		}
	}
}

func (h *WSHub) recountActiveViewers() {
	if h.redisInst == nil || h.redisInst.Client == nil {
		return
	}

	h.RLock()
	if len(h.rooms) == 0 {
		h.RUnlock()
		return
	}
	activePollIDs := make([]string, 0, len(h.rooms))
	for pollID := range h.rooms {
		activePollIDs = append(activePollIDs, pollID)
	}
	h.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().Unix()
	cutoff := fmt.Sprintf("%d", now-viewerTTLSeconds)

	for _, pollID := range activePollIDs {
		key := fmt.Sprintf("poll:%s:viewers_zset", pollID)
		h.redisInst.Client.ZRemRangeByScore(ctx, key, "-inf", cutoff)
		card, err := h.redisInst.Client.ZCard(ctx, key).Result()
		if err == nil {
			h.publishViewerCount(pollID, int(card))
		}
	}
}

func (h *WSHub) checkAutoExpiredPolls() {
	if h.db == nil || h.db.Polls == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	filter := bson.M{
		"is_closed": false,
		"closes_at": bson.M{"$lte": now, "$ne": nil},
	}
	update := bson.M{
		"$set": bson.M{
			"is_closed":  true,
			"updated_at": now,
		},
	}

	cursor, err := h.db.Polls.Find(ctx, filter)
	if err == nil && cursor != nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var poll models.Poll
			if err := cursor.Decode(&poll); err == nil {
				h.BroadcastPollClosed(poll.ID.Hex())
			}
		}
	}

	_, _ = h.db.Polls.UpdateMany(ctx, filter, update)
}

func (h *WSHub) reconcileActivePollCounts() {
	if h.redisInst == nil || h.redisInst.Client == nil || h.db == nil {
		return
	}

	h.RLock()
	if len(h.rooms) == 0 {
		h.RUnlock()
		return
	}
	activePollIDs := make([]string, 0, len(h.rooms))
	for pollID := range h.rooms {
		activePollIDs = append(activePollIDs, pollID)
	}
	h.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, pollID := range activePollIDs {
		_, _ = h.ReconcilePoll(ctx, pollID)
	}
}

// ReconcilePoll checks and repairs counts and orphaned keys for a single poll
func (h *WSHub) ReconcilePoll(ctx context.Context, pollID string) (bool, error) {
	if h.redisInst == nil || h.redisInst.Client == nil || h.db == nil {
		return false, nil
	}

	nowUnix := time.Now().Unix()

	// 1. Quiet Period Check: Skip if a vote occurred in the last 60 seconds
	lastVoteKey := fmt.Sprintf("poll:%s:last_vote_ts", pollID)
	lastVoteVal, lErr := h.redisInst.Client.Get(ctx, lastVoteKey).Int64()
	if lErr == nil && (nowUnix-lastVoteVal) < 60 {
		return false, nil // Skip during quiet period to protect in-flight writes
	}

	// 2. Acquire distributed lock (30s TTL)
	lockKey := fmt.Sprintf("lock:reconcile:%s", pollID)
	acquired, err := h.redisInst.Client.SetNX(ctx, lockKey, "1", 30*time.Second).Result()
	if err != nil || !acquired {
		return false, nil // Another instance is reconciling
	}
	defer h.redisInst.Client.Del(ctx, lockKey)

	poll, err := h.pollService.GetPollByID(ctx, pollID, "")
	if err != nil {
		return false, err
	}

	pollObjID, err := primitive.ObjectIDFromHex(pollID)
	if err != nil {
		return false, err
	}

	pollVotesKey := fmt.Sprintf("poll:%s:votes", pollID)
	redisCounts, err := h.redisInst.Client.HGetAll(ctx, pollVotesKey).Result()
	if err != nil {
		return false, err
	}

	// 3. Compare Redis counts vs MongoDB verified audit counts
	countsMismatch := len(redisCounts) == 0
	repairedVotes := make(map[string]int64)
	var repairedTotal int64

	for _, opt := range poll.Options {
		auditCount, cErr := h.db.Votes.CountDocuments(ctx, bson.M{
			"poll_id":   pollObjID,
			"option_id": opt.ID,
		})
		if cErr != nil {
			auditCount = opt.VoteCount
		}

		if auditCount != opt.VoteCount {
			countsMismatch = true
			_, _ = h.db.Polls.UpdateOne(ctx,
				bson.M{"_id": pollObjID, "options.id": opt.ID},
				bson.M{"$set": bson.M{"options.$.vote_count": auditCount, "updated_at": time.Now().UTC()}},
			)
		}

		var currentRedis int64
		if val, ok := redisCounts[opt.ID]; ok {
			fmt.Sscanf(val, "%d", &currentRedis)
		} else {
			countsMismatch = true
		}

		if currentRedis != auditCount {
			countsMismatch = true
		}

		repairedVotes[opt.ID] = auditCount
		repairedTotal += auditCount
	}

	// If counts match perfectly, skip voter key scanning completely (0 scan commands)
	if !countsMismatch {
		return false, nil
	}

	// 4. Counts differed: repair Redis hash
	pipe := h.redisInst.Client.Pipeline()
	for optID, count := range repairedVotes {
		pipe.HSet(ctx, pollVotesKey, optID, count)
	}
	pipe.Expire(ctx, pollVotesKey, 30*24*time.Hour)
	_, _ = pipe.Exec(ctx)

	// 5. Capped scan for orphaned voter keys (resumes from stored cursor if previous run hit batch limit)
	cursorKey := fmt.Sprintf("poll:%s:scan_cursor", pollID)
	voterPattern := fmt.Sprintf("voter:%s:*", pollID)
	var cursor uint64

	savedCursorStr, cErr := h.redisInst.Client.Get(ctx, cursorKey).Result()
	if cErr == nil {
		fmt.Sscanf(savedCursorStr, "%d", &cursor)
	}

	scanBatches := 0
	maxScanBatches := 5

	for {
		keys, nextCursor, scanErr := h.redisInst.Client.Scan(ctx, cursor, voterPattern, 100).Result()
		if scanErr != nil {
			break
		}
		for _, vk := range keys {
			parts := fmt.Sprintf("voter:%s:", pollID)
			if len(vk) > len(parts) {
				token := vk[len(parts):]
				count, _ := h.db.Votes.CountDocuments(ctx, bson.M{
					"poll_id":   pollObjID,
					"voter_key": token,
				})
				if count == 0 {
					// Orphaned voter key in Redis without MongoDB record -> delete it
					_ = h.redisInst.Client.Del(ctx, vk).Err()
				}
			}
		}
		cursor = nextCursor
		scanBatches++
		if cursor == 0 || scanBatches >= maxScanBatches {
			break
		}
	}

	// If scan reached the end (cursor == 0), clear stored cursor; otherwise persist cursor for next run
	if cursor == 0 {
		_ = h.redisInst.Client.Del(ctx, cursorKey).Err()
	} else {
		_ = h.redisInst.Client.Set(ctx, cursorKey, fmt.Sprintf("%d", cursor), 24*time.Hour).Err()
	}

	// 6. Broadcast repaired state to connected viewers
	h.broadcast <- &models.WSMessage{
		Type:       models.WSMsgVoteUpdate,
		PollID:     pollID,
		Votes:      repairedVotes,
		TotalVotes: repairedTotal,
		IsClosed:   poll.IsClosed,
	}

	return true, nil
}

func (h *WSHub) cleanupAll() {
	h.Lock()
	defer h.Unlock()

	for pollID, room := range h.rooms {
		if room.CancelSub != nil {
			room.CancelSub()
		}
		for client := range room.Clients {
			_ = client.Conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down"),
				time.Now().Add(writeWait),
			)
			client.Conn.Close()
		}
		delete(h.rooms, pollID)
	}
}

func (h *WSHub) Shutdown() {
	close(h.stopChan)
}

// Client ReadPump and WritePump
func (c *WSClient) ReadPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("websocket read error: %v", err)
			}
			break
		}
	}
}

func (c *WSClient) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			n := len(c.Send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			// Refresh viewer heartbeat in Redis sorted set
			c.Hub.refreshViewerHeartbeat(c.PollID, c.ID)
		}
	}
}

func ServeWS(hub *WSHub, conn *websocket.Conn, pollID string, clientIP string) {
	client := &WSClient{
		ID:     uuid.New().String(),
		PollID: pollID,
		IP:     clientIP,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		Hub:    hub,
	}

	hub.register <- client

	go client.WritePump()
	go client.ReadPump()
}
