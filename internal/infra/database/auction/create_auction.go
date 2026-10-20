package auction

import (
	"context"
	"fullcycle-auction_go/configuration/logger"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"fullcycle-auction_go/internal/internal_error"
	"os"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type AuctionEntityMongo struct {
	Id          string                          `bson:"_id"`
	ProductName string                          `bson:"product_name"`
	Category    string                          `bson:"category"`
	Description string                          `bson:"description"`
	Condition   auction_entity.ProductCondition `bson:"condition"`
	Status      auction_entity.AuctionStatus    `bson:"status"`
	Timestamp   int64                           `bson:"timestamp"`
}
type AuctionRepository struct {
	Collection *mongo.Collection
	appCtx     context.Context
}

func NewAuctionRepository(ctx context.Context, database *mongo.Database) *AuctionRepository {
	return &AuctionRepository{
		Collection: database.Collection("auctions"),
		appCtx:     ctx,
	}
}

func (ar *AuctionRepository) CreateAuction(
	ctx context.Context,
	auctionEntity *auction_entity.Auction) *internal_error.InternalError {
	auctionEntityMongo := &AuctionEntityMongo{
		Id:          auctionEntity.Id,
		ProductName: auctionEntity.ProductName,
		Category:    auctionEntity.Category,
		Description: auctionEntity.Description,
		Condition:   auctionEntity.Condition,
		Status:      auctionEntity.Status,
		Timestamp:   auctionEntity.Timestamp.Unix(),
	}
	_, err := ar.Collection.InsertOne(ctx, auctionEntityMongo)
	if err != nil {
		logger.Error("Error trying to insert auction", err)
		return internal_error.NewInternalServerError("Error trying to insert auction")
	}

	go ar.setAuctionClosed(ar.appCtx, auctionEntity.Id)

	return nil
}

func (ar *AuctionRepository) setAuctionClosed(ctx context.Context, id string) {
	duration := os.Getenv("AUCTION_DURATION")

	durationInt, err := strconv.Atoi(duration)
	if err != nil {
		logger.Error("Error trying to parse duration", err)
		return
	}

	durationS := time.Second * time.Duration(durationInt)

	filter := bson.M{"_id": id}

	select {
	case <-ctx.Done():
		flushCtx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		ar.closeAuction(flushCtx, filter)
		return
	case <-time.After(durationS):
		ar.closeAuction(ar.appCtx, filter)
		return
	}
}

func (ar *AuctionRepository) closeAuction(ctx context.Context, filter bson.M) {
	update := bson.M{"$set": bson.M{"status": auction_entity.Completed}}
	_, err := ar.Collection.UpdateOne(ctx, filter, update)
	if err != nil {
		logger.Error("Error trying to close auction", err)
		return
	}
}
