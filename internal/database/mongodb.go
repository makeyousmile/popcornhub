package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"popcornhub/internal/models"
)

type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func ConnectMongoDB(ctx context.Context, uri, dbName string) (*MongoDB, error) {
	clientOpts := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(50).
		SetMinPoolSize(10).
		SetMaxConnIdleTime(5 * time.Minute)

	client, err := mongo.Connect(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create mongo client: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping mongo: %w", err)
	}

	db := client.Database(dbName)
	mg := &MongoDB{
		Client:   client,
		Database: db,
	}

	if err := mg.ensureIndexes(ctx); err != nil {
		return nil, fmt.Errorf("failed to create indexes: %w", err)
	}

	if err := mg.seedInitialMedia(ctx); err != nil {
		slog.Warn("Failed to seed initial media catalog", "error", err)
	}

	slog.Info("Connected to MongoDB successfully", "database", dbName)
	return mg, nil
}

func (m *MongoDB) ensureIndexes(ctx context.Context) error {
	usersCol := m.Database.Collection("users")
	groupsCol := m.Database.Collection("groups")
	roomsCol := m.Database.Collection("watch_rooms")

	// 1. Unique index on users.email
	_, err := usersCol.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("create users.email index: %w", err)
	}

	// 2. Unique index on groups.invite_code
	_, err = groupsCol.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "invite_code", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("create groups.invite_code index: %w", err)
	}

	// 3. Index on groups.members.user_id
	_, err = groupsCol.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "members.user_id", Value: 1}},
	})
	if err != nil {
		return fmt.Errorf("create groups.members.user_id index: %w", err)
	}

	// 4. Index on watch_rooms.group_id
	_, err = roomsCol.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "group_id", Value: 1}},
	})
	if err != nil {
		return fmt.Errorf("create watch_rooms.group_id index: %w", err)
	}

	slog.Info("MongoDB indexes verified and created")
	return nil
}

func (m *MongoDB) seedInitialMedia(ctx context.Context) error {
	col := m.Database.Collection("media_catalog")
	count, err := col.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	sampleMedia := []interface{}{
		models.MediaItem{
			ID:          bson.NewObjectID(),
			Title:       "Big Buck Bunny (Открытый фильм)",
			StreamURL:   "https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4",
			DurationSec: 596,
			PosterURL:   "https://upload.wikimedia.org/wikipedia/commons/thumb/c/c5/Big_buck_bunny_poster_big.jpg/640px-Big_buck_bunny_poster_big.jpg",
			Metadata: models.MediaMetadata{
				Genre: []string{"Анимация", "Комедия"},
				Year:  2008,
			},
		},
		models.MediaItem{
			ID:          bson.NewObjectID(),
			Title:       "Слезы стали (Tears of Steel)",
			StreamURL:   "https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/TearsOfSteel.mp4",
			DurationSec: 734,
			PosterURL:   "https://upload.wikimedia.org/wikipedia/commons/thumb/e/e0/Tears_of_Steel_poster.jpg/640px-Tears_of_Steel_poster.jpg",
			Metadata: models.MediaMetadata{
				Genre: []string{"Sci-Fi", "Экшен"},
				Year:  2012,
			},
		},
		models.MediaItem{
			ID:          bson.NewObjectID(),
			Title:       "Sintel (Синтел)",
			StreamURL:   "https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/Sintel.mp4",
			DurationSec: 888,
			PosterURL:   "https://upload.wikimedia.org/wikipedia/commons/thumb/8/8f/Sintel_poster.jpg/640px-Sintel_poster.jpg",
			Metadata: models.MediaMetadata{
				Genre: []string{"Фэнтези", "Анимация"},
				Year:  2010,
			},
		},
		models.MediaItem{
			ID:          bson.NewObjectID(),
			Title:       "Elephants Dream (Мечта Слонов)",
			StreamURL:   "https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ElephantsDream.mp4",
			DurationSec: 653,
			PosterURL:   "https://upload.wikimedia.org/wikipedia/commons/thumb/e/ee/Elephants_Dream_poster.jpg/640px-Elephants_Dream_poster.jpg",
			Metadata: models.MediaMetadata{
				Genre: []string{"Sci-Fi", "Анимация"},
				Year:  2006,
			},
		},
	}

	_, err = col.InsertMany(ctx, sampleMedia)
	if err != nil {
		return err
	}
	slog.Info("Seeded initial media catalog with open sample movies", "count", len(sampleMedia))
	return nil
}

func (m *MongoDB) Close(ctx context.Context) error {
	return m.Client.Disconnect(ctx)
}
