package user

import (
	"LeaderboardsBackend/util"
	json2 "encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
)

type userRequest struct {
	ID   string `uri:"id" binding:"uuid"`
	Game string `uri:"game" binding:""`
	Mode string `uri:"mode" binding:""`
}

type gameModeUserResponse struct {
	Name        string
	EventTotals map[string]map[string]map[string]int32
}

var Client *mongo.Client

func Register(r *gin.Engine, client *mongo.Client) {
	Client = client

	util.CachedGET(r, "/user", userHandler)
	util.CachedGET(r, "/user/profile/:id", userHandler)
	util.CachedGET(r, "/user/profile/:id/:game", userHandler)
	util.CachedGET(r, "/user/profile/:id/:game/:mode", userHandler)
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
	}

	matchGame := ``

	if (request.Game != "") {
		matchGame += ", \"game_id\" : \"" + request.Game + "\""
	}

	if request.Mode != "" {
		matchGame += ", \"game_mode_id\" : \"" + request.Mode + "\""
	}

	pipeline = fmt.Sprintf(pipeline, request.ID, "TODO" , request.ID, request.Game)
	log.Println(pipeline)

	var gameModeUserResponse gameModeUserResponse
	gameModeUserResponse.EventTotals = make(map[string]map[string]map[string]int32)
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
					gameModeUserResponse.EventTotals[result.GameID] = make(map[string]map[string]int32)
				}
				if (gameModeUserResponse.EventTotals[result.GameID][result.GameModeID] == nil) {
					gameModeUserResponse.EventTotals[result.GameID][result.GameModeID] =  make(map[string]int32)
				}

				gameModeUserResponse.EventTotals[result.GameID][result.GameModeID]["Deaths"]++
			} else {
				gameModeUserResponse.EventTotals[result.GameID][result.GameModeID]["Kills"]++
			}
		}

		if result.AnalyticEventType == "Score" {
			if (gameModeUserResponse.EventTotals[result.GameID] == nil) {
				gameModeUserResponse.EventTotals[result.GameID] = make(map[string]map[string]int32)
			}
			if (gameModeUserResponse.EventTotals[result.GameID][result.GameModeID] == nil) {
				gameModeUserResponse.EventTotals[result.GameID][result.GameModeID] =  make(map[string]int32)
			}
			gameModeUserResponse.EventTotals[result.GameID][result.GameModeID][result.ScoreField] += int32(result.Value)
		}
	})

	json, err := json2.MarshalIndent(gameModeUserResponse, "", "    ")
	if err != nil {
		log.Fatal(err)
		return nil
	}
	return json
}
