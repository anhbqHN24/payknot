package database

import (
	"context"
	"log"
	"os"

	"github.com/go-redis/redis/v8"
)

var RDB *redis.Client
var Ctx = context.Background()

func ConnectRedis() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Fatal("REDIS_URL is not set")
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Unable to parse REDIS_URL: %v", err)
	}

	RDB = redis.NewClient(opt)

	_, err = RDB.Ping(Ctx).Result()
	if err != nil {
		log.Fatalf("Unable to connect to Redis: %v", err)
	}

	log.Println("Redis connection successful")
}
