package main

import (
	"LeaderboardsBackend/game"
	"LeaderboardsBackend/leaderboard"
	"LeaderboardsBackend/user"
	"LeaderboardsBackend/util"
	"context"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"log"
	"os"
	"time"
)

func main() {
	r := gin.Default()
	gin.ForceConsoleColor()

	client, _ := mongo.NewClient(options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	client.Connect(ctx)
	err := client.Ping(ctx, readpref.Primary())
	if err != nil {
		log.Fatal("Failed to connect to mongo: ", err)
	}

	defer client.Disconnect(ctx)
	util.SetupCache()

	r.GET("/", HomeHandler)
	r.Group("/v1")
	{
		game.Register(r, client)
		leaderboard.Register(r, client)
		user.Register(r, client)
	}

	r.Run()
}

func HomeHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "Home",
	})
}
