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
	Game string `uri:"game" binding:"uuid"`
	Mode string `uri:"mode" binding:"uuid"`
}

type gameModeUserResponse struct {
	Name        string
	Wins        int64
	Losses      int64
	EventTotals map[string]int64
}

var Client *mongo.Client

func Register(r *gin.Engine, client *mongo.Client) {
	Client = client

	util.CachedGET(r, "/user", userHandler)
	util.CachedGET(r, "/user/profile/:id", userHandler)
	util.CachedGET(r, "/user/profile/:id/:game", userHandler)
}

func userHandler(c *gin.Context) []byte {
	var request userRequest
	if err := c.ShouldBindUri(&request); err != nil {
		c.JSON(400, gin.H{"error": err})
		return nil
	}

	pipeline := `
	[
		%s
        {
			"$match" : {
				%s
			}
		}, 
		{ 
			"$project" : {
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

	if request.ID == "" {
		c.JSON(400, gin.H{"error": "No user provided"})
	} else if request.Game == "" {
		pipeline = fmt.Sprintf(pipeline, "", "\"player_uuid\": \""+request.ID+"\"")

		c.JSON(200, gin.H{"message": "TODO"})
		return nil;
	} else if request.Mode == "" {
		c.JSON(200, gin.H{"message": "TODO"})
		return nil;
	}

	initialProject := `
        { 
            "$project" : {
                "winners" : {
                    "$objectToArray" : "$winners"
                }, 
                "losers" : {
                    "$objectToArray" : "$losers"
                }, 
                "doc" : "$$ROOT"
            }
        },
    `

	pipelineText := `
        $or: [{player_uuid: "%s"},{killer_uuid: "%s"}],
	    game_id: "%s",
        game_mode_id: "%s"
	`
	pipelineText = fmt.Sprintf(pipelineText, request.ID, request.ID, request.Game, request.Mode)
	pipeline = fmt.Sprintf(pipeline, initialProject, pipelineText)

	var gameModeUserResponse gameModeUserResponse
	gameModeUserResponse.EventTotals = make(map[string]int64)
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
				gameModeUserResponse.EventTotals["Deaths"]++
			} else {
				gameModeUserResponse.EventTotals["Kills"]++
			}
		}

		if result.AnalyticEventType == "Score" {
			gameModeUserResponse.EventTotals[result.ScoreField] += int64(result.Value)
		}

		if result.AnalyticEventType == "Finish" {
			if _, ok := result.Winners[request.ID]; ok {
				gameModeUserResponse.Wins++
			} else {
				gameModeUserResponse.Losses++
			}
		}
	})

	json, err := json2.MarshalIndent(gameModeUserResponse, "", "    ")
	if err != nil {
		log.Fatal(err)
		return nil
	}
	return json
}
