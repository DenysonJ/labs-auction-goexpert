package auction

import (
	"context"
	"os"
	"testing"
	"time"

	"fullcycle-auction_go/internal/entity/auction_entity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// testDBName is an isolated database so the test never touches real data.
const testDBName = "auctions_test"

func connectTestDB(t *testing.T, ctx context.Context) *mongo.Database {
	t.Helper()

	mongoURL := os.Getenv("MONGODB_URL")
	if mongoURL == "" {
		mongoURL = "mongodb://admin:admin@localhost:27017/auctions?authSource=admin"
	}

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	client, err := mongo.Connect(pingCtx, options.Client().ApplyURI(mongoURL))
	if err != nil {
		t.Skipf("skipping: cannot connect to MongoDB (%v). Start it with `docker compose up -d mongodb`.", err)
	}

	if err := client.Ping(pingCtx, nil); err != nil {
		t.Skipf("skipping: MongoDB is not reachable (%v). Start it with `docker compose up -d mongodb`.", err)
	}

	return client.Database(testDBName)
}

// TestCreateAuction_AutomaticClose proves the challenge scenario:
//
//  1. Create an auction            -> status Active (open)
//  2. Wait for AUCTION_DURATION    -> no manual intervention
//  3. Check the auction status     -> automatically Completed (closed)
func TestCreateAuction_AutomaticClose(t *testing.T) {
	t.Setenv("AUCTION_DURATION", "1")

	ctx := context.Background()
	database := connectTestDB(t, ctx)
	t.Cleanup(func() {
		_ = database.Drop(context.Background())
	})

	repository := NewAuctionRepository(ctx, database)

	auction, createErr := auction_entity.CreateAuction(
		"Playstation 5",
		"Games",
		"A brand new Playstation 5 console",
		auction_entity.New,
	)
	require.Nil(t, createErr)

	// Step 1: create the auction and assert it starts open (Active).
	if insertErr := repository.CreateAuction(ctx, auction); insertErr != nil {
		t.Fatalf("unexpected error creating auction: %v", insertErr.Message)
	}

	persisted, findErr := repository.FindAuctionById(ctx, auction.Id)
	require.Nil(t, findErr)
	assert.Equal(t, auction_entity.Active, persisted.Status, "auction must start as Active")

	// Steps 2 and 3: wait past AUCTION_DURATION and assert the background
	// goroutine closed the auction on its own, without any manual update.
	assert.Eventually(t, func() bool {
		current, err := repository.FindAuctionById(ctx, auction.Id)
		if err != nil {
			return false
		}
		return current.Status == auction_entity.Completed
	}, 5*time.Second, 200*time.Millisecond, "auction must be closed automatically after AUCTION_DURATION")
}
