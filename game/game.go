package game

import (
	"LeaderboardsBackend/util"
	json2 "encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/simagix/keyhole/mdb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
)

type gameRequest struct {
	Game string `uri:"game" binding:""`
	Mode string `uri:"mode" binding:""`
}

type instanceRequest struct {
	ID string `uri:"id" binding:""`
}

type fullGameResponse struct {
	Map       string
	StartTime int64
	EndTime   int64
	Winners   map[string]string
	Losers    map[string]string
	Scores    [] score
	Deaths    [] death
}

type score struct {
	ScoreType string
	Value     int32
	User      string
	UUID      string
	Time      int64
}

type death struct {
	Killer     string
	KillerUUID string
	Victim     string
	VictimUUID string
	Reason     string
	Time       int64
}

type gameListResult struct {
	Game interface{} `bson:"_id"`
	GameModes [] string `bson:"gamemodes"`
}

var Client *mongo.Client

func Register(r *gin.Engine, client *mongo.Client) {
	Client = client

	util.CachedGET(r, "/game/list", gameListHandler)
	util.CachedGET(r, "/game/list/:game", gameListHandler)
	util.CachedGET(r, "/game/list/:game/:mode", gameListHandler)
	util.CachedGET(r, "/game/instance/:id", instanceHandler)
}

func gameListHandler(c *gin.Context) []byte {
	var err error
	var collection *mongo.Collection
	var cur *mongo.Cursor

	var request gameRequest
	if err := c.ShouldBindUri(&request); err != nil {
		c.JSON(400, gin.H{"error": err})
		return nil
	}

	pipeline := `[
        { 
            "$group" : {
                "_id" : "$game_id",
                "gamemodes" : {
                    "$addToSet" : "$game_mode_id"
                }
            }
        }
    ]`
	collection = Client.Database("Analytics").Collection("Events")
	opts := options.Aggregate()
	opts.SetAllowDiskUse(true)
	if cur, err = collection.Aggregate(c, mdb.MongoPipeline(pipeline), opts); err != nil {
		log.Fatal(err)
	}

	var results [] gameListResult
	for cur.Next(c) {
		var result gameListResult
		err := cur.Decode(&result)
		if err != nil {
			log.Fatal(err)
		}

		if result.Game != nil {
			results = append(results, result)
		}
	}

	json, err := json2.MarshalIndent(results, "", "    ")
	if err != nil {
		log.Fatal(err)
		return nil
	}

	return json
}

func instanceHandler(c *gin.Context) []byte {
	var err error

	var request instanceRequest
	if err = c.ShouldBindUri(&request); err != nil {
		c.JSON(400, gin.H{"error": err})
		return nil
	}

	pipeline := `
	[{ 
			"$match" : {
				"instance_id" : "%s"
			}
		}, 
		{ 
			"$project" : {
				"time_code" : 1.0, 
				"server_event_type" : 1.0,
				"world_name": 1.0,
				"analytic_event_type" : 1.0, 
				"from" : 1.0, 
				"to" : 1.0, 
				"value" : 1.0, 
				"score_field" : 1.0, 
				"player_name" : 1.0, 
				"player_uuid" : 1.0, 
				"death_event_type" : 1.0, 
				"death_details" : 1.0, 
				"killer_name" : 1.0, 
				"killer_uuid" : 1.0, 
				"winners" : 1.0, 
				"losers" : 1.0, 
				"finish_event_type" : 1.0
			}
	}]`
	pipeline = fmt.Sprintf(pipeline, request.ID)


	var fullGameResponse fullGameResponse

	util.RunPipelineOnEvents(pipeline, Client, c, func(result util.MongoResult) {
		if result.ServerEventType == "Game" && result.AnalyticEventType == "Finish" {
			fullGameResponse.EndTime = result.TimeCode
			fullGameResponse.Losers = result.Losers
			fullGameResponse.Winners = result.Winners
		}

		if result.ServerEventType == "Game" && result.AnalyticEventType == "ServerStateChange" && result.To == "INGAME" {
			fullGameResponse.StartTime = result.TimeCode
		}

		if result.ServerEventType == "Game" && result.AnalyticEventType == "Score" {
			score := score{
				ScoreType: result.ScoreField,
				Value:     result.Value,
				User:      result.PlayerName,
				UUID:      result.PlayerUUID,
				Time:      result.TimeCode,
			}

			fullGameResponse.Scores = append(fullGameResponse.Scores, score)
		}

		if result.ServerEventType == "Game" && result.AnalyticEventType == "Death" {
			death := death{
				Killer:     result.KillerName,
				KillerUUID: result.KillerUUID,
				Victim:     result.PlayerName,
				VictimUUID: result.PlayerUUID,
				Reason:     result.DeathDetails,
				Time:       result.TimeCode,
			}

			fullGameResponse.Deaths = append(fullGameResponse.Deaths, death)
		}
	})

	json, err := json2.MarshalIndent(fullGameResponse, "", "    ")
	if err != nil {
		log.Fatal(err)
		return nil
	}
	return json
}
