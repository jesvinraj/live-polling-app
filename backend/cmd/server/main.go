package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"live-polling-tool/config"
	"live-polling-tool/database"
	"live-polling-tool/handlers"
	"live-polling-tool/middleware"
	"live-polling-tool/services"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadConfig()

	if cfg.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	log.Printf("Connecting to MongoDB at %s...", cfg.MongoURI)
	mongoInstance, err := database.ConnectMongoDB(cfg)
	if err != nil {
		log.Fatalf("MongoDB connection failed: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mongoInstance.Client.Disconnect(ctx); err != nil {
			log.Printf("Error disconnecting MongoDB: %v", err)
		}
	}()
	log.Printf("Successfully connected to MongoDB database: %s", cfg.MongoDBName)

	log.Printf("Connecting to Redis at %s...", cfg.RedisAddr)
	redisInstance, err := database.ConnectRedis(cfg)
	if err != nil {
		log.Printf("Warning: Redis connection failed: %v. Live features will attempt reconnect.", err)
	} else {
		log.Println("Successfully connected to Redis")
		defer redisInstance.Client.Close()
	}

	// Initialize Services
	authService := services.NewAuthService(mongoInstance, cfg)
	pollService := services.NewPollService(mongoInstance, redisInstance)
	voteService := services.NewVoteService(mongoInstance, redisInstance)
	wsHub := services.NewWSHub(mongoInstance, redisInstance, pollService)

	// Initialize Handlers
	authHandler := handlers.NewAuthHandler(authService)
	pollHandler := handlers.NewPollHandler(pollService, authService, wsHub)
	voteHandler := handlers.NewVoteHandler(voteService)
	wsHandler := handlers.NewWSHandler(wsHub, cfg)

	// Router Setup
	r := gin.New()
	_ = r.SetTrustedProxies(cfg.TrustedProxies)
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware(cfg.AllowedOrigins))
	r.Use(middleware.RequestSizeLimiter(32 * 1024)) // 32 KB request size limit

	// Health Check Route
	r.GET("/api/v1/health", func(c *gin.Context) {
		redisStatus := "connected"
		if redisInstance == nil || redisInstance.Client.Ping(c.Request.Context()).Err() != nil {
			redisStatus = "disconnected"
		}
		c.JSON(http.StatusOK, gin.H{
			"status":   "healthy",
			"database": "connected",
			"redis":    redisStatus,
			"clientIp": c.ClientIP(),
			"time":     time.Now().UTC(),
		})
	})

	// Public WebSocket Route
	r.GET("/ws/polls/:id", wsHandler.HandlePollWS)

	// API Routes Group
	v1 := r.Group("/api/v1")
	{
		authGroup := v1.Group("/auth")
		{
			// Rate limit signup and login: max 10 requests per minute per IP
			authRateLimit := middleware.RateLimiter(redisInstance, "auth", 10, time.Minute)
			authGroup.POST("/signup", authRateLimit, authHandler.Signup)
			authGroup.POST("/login", authRateLimit, authHandler.Login)
			authGroup.GET("/me", middleware.AuthMiddleware(authService), authHandler.GetMe)
		}

		pollGroup := v1.Group("/polls")
		{
			// Public read and vote status routes
			pollGroup.GET("/:id", pollHandler.GetPoll)
			pollGroup.GET("/:id/status", voteHandler.CheckVoteStatus)

			// Rate limit voting: max 30 vote attempts per minute per IP
			voteRateLimit := middleware.RateLimiter(redisInstance, "vote", 30, time.Minute)
			pollGroup.POST("/:id/vote", voteRateLimit, voteHandler.SubmitVote)

			// Protected poll management routes
			protectedPolls := pollGroup.Group("")
			protectedPolls.Use(middleware.AuthMiddleware(authService))
			{
				protectedPolls.POST("", pollHandler.CreatePoll)
				protectedPolls.GET("/my", pollHandler.GetMyPolls)
				protectedPolls.PATCH("/:id/close", pollHandler.ClosePoll)
				protectedPolls.DELETE("/:id", pollHandler.DeletePoll)
			}
		}
	}

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server listening on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Graceful shutdown listener
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server and closing connections...")
	wsHub.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting cleanly")
}
