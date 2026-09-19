package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"live-polling-tool/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoInstance struct {
	Client   *mongo.Client
	Database *mongo.Database
	Users    *mongo.Collection
	Polls    *mongo.Collection
	Votes    *mongo.Collection
}

func ConnectMongoDB(cfg *config.Config) (*MongoInstance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(cfg.MongoURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize mongodb client: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping mongodb: %w", err)
	}

	db := client.Database(cfg.MongoDBName)
	instance := &MongoInstance{
		Client:   client,
		Database: db,
		Users:    db.Collection("users"),
		Polls:    db.Collection("polls"),
		Votes:    db.Collection("votes"),
	}

	if err := instance.ensureIndexes(ctx); err != nil {
		log.Printf("warning: failed to create one or more mongo indexes: %v", err)
	}

	return instance, nil
}

func (m *MongoInstance) ensureIndexes(ctx context.Context) error {
	// Unique index on user email
	_, err := m.Users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	// Unique index on username
	_, err = m.Users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "username", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	// Index on polls by owner_id
	_, err = m.Polls.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "owner_id", Value: 1}, {Key: "created_at", Value: -1}},
	})
	if err != nil {
		return err
	}

	// Unique compound index on votes (poll_id + voter_key) for deduplication
	_, err = m.Votes.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "poll_id", Value: 1}, {Key: "voter_key", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}

	return nil
}
