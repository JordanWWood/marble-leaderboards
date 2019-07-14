package util

import (
	"github.com/gin-gonic/gin"
	"github.com/simagix/keyhole/mdb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
)

type MongoResult struct {
	TimeCode          int64             `bson:"time_code"`
	ServerEventType   string            `bson:"server_event_type"`
	GameID            string            `bson:"game_id"`
	GameModeID        string            `bson:"game_mode_id"`
	AnalyticEventType string            `bson:"analytic_event_type"`
	WorldName         string            `bson:"world_name"`
	From              string            `bson:"from"`
	To                string            `bson:"to"`
	Value             int32             `bson:"value"`
	ScoreField        string            `bson:"score_field"`
	PlayerName        string            `bson:"player_name"`
	PlayerUUID        string            `bson:"player_uuid"`
	DeathEventType    string            `bson:"death_event_type"`
	DeathDetails      string            `bson:"death_details"`
	KillerName        string            `bson:"killer_name"`
	KillerUUID        string            `bson:"killer_uuid"`
	Winners           map[string]string `bson:"winners"`
	Losers            map[string]string `bson:"losers"`
	FinishEventType   string            `bson:"finish_event_type"`
}

func RunPipelineOnEvents(pipeline string, client *mongo.Client, ctx *gin.Context, action func(result MongoResult)) {
	var err error
	var collection *mongo.Collection
	var cur *mongo.Cursor

	collection = client.Database("Analytics").Collection("Events")
	opts := options.Aggregate()
	opts.SetAllowDiskUse(true)

	if cur, err = collection.Aggregate(ctx, mdb.MongoPipeline(pipeline), opts); err != nil {
		log.Fatal(err)
	}
	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var result MongoResult
		err := cur.Decode(&result)
		if err != nil {
			log.Fatal(err)
		}

		action(result)
	}
}
