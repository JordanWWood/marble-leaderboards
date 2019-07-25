package user

import (
	"LeaderboardsBackend/util"
	json2 "encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/simagix/keyhole/mdb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"strconv"
)

type userRequest struct {
	ID   string `uri:"id" binding:"uuid"`
	Game string `uri:"game" binding:""`
	Mode string `uri:"mode" binding:""`
}

type gameModeUserResponse struct {
	Name        string
	EventTotals map[string]map[string]map[string]int64
}

type recentGameBson struct {
	GameID     string            `bson:"game_id"`
	GameModeID string            `bson:"game_mode_id"`
	InstanceID string            `bson:"instance_id"`
	Winners    map[string]string `bson:"winners"`
	Losers     map[string]string `bson:"losers"`
	EndTime    int64             `bson:"end_time"`
}

var Client *mongo.Client

func Register(r *gin.Engine, client *mongo.Client) {
	Client = client

	util.CachedGET(r, "/user", userHandler)
	util.CachedGET(r, "/user/recent/:id", recentHandler)
	util.CachedGET(r, "/user/profile/:id", userHandler)
	util.CachedGET(r, "/user/profile/:id/:game", userHandler)
	util.CachedGET(r, "/user/profile/:id/:game/:mode", userHandler)
}

func recentHandler(c *gin.Context) []byte {
	var err error
	var collection *mongo.Collection
	var cur *mongo.Cursor

	var request userRequest
	if err := c.ShouldBindUri(&request); err != nil {
		c.JSON(400, gin.H{"error": err})
		return nil
	}

	page, err := strconv.Atoi(c.Query("page"))
	if err != nil {
		page = 1
	}

	length, err := strconv.Atoi(c.Query("length"))
	if err != nil {
		length = 100
	}

	pipeline := `
	[
        { 
            "$match" : {
                "analytic_event_type" : "Finish"
            }
        }, 
        { 
            "$project" : {
                "game_id" : "$game_id", 
                "game_mode_id" : "$game_mode_id", 
                "instance_id" : "$instance_id", 
                "winners" : "$winners", 
                "losers" : "$losers", 
                "end_time" : "$time_code"
            }
        }, 
        { 
            "$group" : {
                "_id" : {
                    "player" : {
                        "$or" : [
                            {
                                "$gt" : [
                                    "$winners.%s", 
                                    null
                                ]
                            }, 
                            {
                                "$gt" : [
                                    "$losers.%s", 
                                    null
                                ]
                            }
                        ]
                    }, 
                    "game_id" : "$game_id", 
                    "game_mode_id" : "$game_mode_id", 
                    "instance_id" : "$instance_id", 
                    "winners" : "$winners", 
                    "losers" : "$losers", 
                    "end_time" : "$end_time"
                }
            }
        }, 
        { 
            "$match" : {
                "_id.player" : true
            }
        }, 
        { 
            "$replaceRoot" : {
                "newRoot" : "$_id"
            }
        }, 
        { 
            "$project" : {
                "game_id" : 1.0, 
                "game_mode_id" : 1.0, 
                "instance_id" : 1.0, 
                "winners" : 1.0, 
                "losers" : 1.0,
				"end_time": 1.0
            }
        },
		{
			"$sort": {
				"end_time": -1.0
			}
		},
		{
			"$skip": %d
		},
		{
			"$limit": %d
		}
    ]`

	pipeline = fmt.Sprintf(pipeline, request.ID, request.ID, (page*length)-length, length)

	collection = Client.Database("Analytics").Collection("Events")
	opts := options.Aggregate()
	opts.SetAllowDiskUse(true)
	if cur, err = collection.Aggregate(c, mdb.MongoPipeline(pipeline), opts); err != nil {
		c.JSON(500, gin.H{"err": err})
		return nil
	}

	var results [] recentGameBson
	for cur.Next(c) {
		var result recentGameBson

		err := cur.Decode(&result)
		if err != nil {
			c.JSON(500, gin.H{"err": err})
			return nil
		}

		results = append(results, result)
	}

	json, err := json2.MarshalIndent(results, "", "    ")
	if err != nil {
		log.Fatal(err)
		return nil
	}

	return json
}

func userHandler(c *gin.Context) []byte {
	var request userRequest
	if err := c.ShouldBindUri(&request); err != nil {
		c.JSON(400, gin.H{"error": err})
		return nil
	}

	pipeline := `
	[
        { 
            "$match" : {
                "$or" : [
                    {
                        "player_uuid" : "%s"
                    }, 
                    {
                        "killer_uuid" : "%s"
                    }
                ]
				%s
            }
        }, 
        { 
            "$project" : {
                "game_id" : 1.0, 
                "game_mode_id" : 1.0, 
                "server_event_type" : 1.0, 
                "world_name" : 1.0, 
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
        }
    ]`

	if request.ID == "" {
		c.JSON(400, gin.H{"error": "No user provided"})
		return nil;
	}

	matchGame := ``

	if (request.Game != "") {
		matchGame += ", \"game_id\" : \"" + request.Game + "\""
	}

	if request.Mode != "" {
		matchGame += ", \"game_mode_id\" : \"" + request.Mode + "\""
	}

	pipeline = fmt.Sprintf(pipeline, request.ID, request.ID, matchGame)

	var gameModeUserResponse gameModeUserResponse
	gameModeUserResponse.EventTotals = make(map[string]map[string]map[string]int64)
	util.RunPipelineOnEvents(pipeline, Client, c, func(result util.MongoResult) {
		if gameModeUserResponse.Name == "" {
			if result.PlayerUUID == request.ID {
				gameModeUserResponse.Name = result.PlayerName
			} else {
				gameModeUserResponse.Name = result.KillerName
			}
		}

		if result.AnalyticEventType == "Death" {
			if result.PlayerUUID == request.ID {
				if (gameModeUserResponse.EventTotals[result.GameID] == nil) {
					gameModeUserResponse.EventTotals[result.GameID] = make(map[string]map[string]int64)
				}
				if (gameModeUserResponse.EventTotals[result.GameID][result.GameModeID] == nil) {
					gameModeUserResponse.EventTotals[result.GameID][result.GameModeID] = make(map[string]int64)
				}

				gameModeUserResponse.EventTotals[result.GameID][result.GameModeID]["deaths"]++
			} else {
				gameModeUserResponse.EventTotals[result.GameID][result.GameModeID]["kills"]++
			}
		}

		if result.AnalyticEventType == "Score" {
			if (gameModeUserResponse.EventTotals[result.GameID] == nil) {
				gameModeUserResponse.EventTotals[result.GameID] = make(map[string]map[string]int64)
			}
			if (gameModeUserResponse.EventTotals[result.GameID][result.GameModeID] == nil) {
				gameModeUserResponse.EventTotals[result.GameID][result.GameModeID] = make(map[string]int64)
			}
			gameModeUserResponse.EventTotals[result.GameID][result.GameModeID][result.ScoreField] += int64(result.Value)
		}
	})

	json, err := json2.MarshalIndent(gameModeUserResponse, "", "    ")
	if err != nil {
		c.JSON(500, gin.H{"err": err})
		return nil
	}
	return json
}
