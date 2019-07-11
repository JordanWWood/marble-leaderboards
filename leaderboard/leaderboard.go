package leaderboard

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
	"strings"
)

type leaderboardRequest struct {
	Game     string `uri:"game" binding:""`
	Mode     string `uri:"mode" binding:""`
	Filter   string `uri:"filter" binding:""`
	Instance string `uri:"instance" binding:""`
}

type mongoResult struct {
	ID     string           `bson:"_id"`
	Scores map[string]int32 `bson:"scores"`
}

var Client *mongo.Client

func Register(r *gin.Engine, client *mongo.Client) {
	Client = client

	util.CachedGET(r, "/leaderboard", leaderboardHandler)
	util.CachedGET(r, "/leaderboard/:game", leaderboardHandler)
	util.CachedGET(r, "/leaderboard/:game/:mode", leaderboardHandler)
	util.CachedGET(r, "/leaderboard/:game/:mode/:filter", leaderboardHandler)
	util.CachedGET(r, "/leaderboard/:game/:mode/:filter/:instance", leaderboardHandler)
}

func leaderboardHandler(r *gin.Context) []byte {
	var err error
	var collection *mongo.Collection
	var cur *mongo.Cursor

	var request leaderboardRequest
	if err = r.ShouldBindUri(&request); err != nil {
		r.JSON(400, gin.H{"error": err})
		return nil
	}

	page, err := strconv.Atoi(r.Query("page"))
	length, err := strconv.Atoi(r.Query("length"))
	if err != nil {
		page = 1
		length = 100
	}

	{
		pipeline := `
		[
			%s
			{ 
				"$project" : {
					"value" : 1.0, 
					"score_field" : 1.0, 
					"player_uuid" : 1.0, 
					"winners" : 1.0, 
					"losers" : 1.0
				}
			}, 
			{ 
				"$group" : {
					"_id" : {
						"uuid" : "$player_uuid", 
						"score" : "$score_field"
					}, 
					"value" : {
						"$sum" : "$value"
					}
				}
			}, 
			{ 
				"$group" : {
					"_id" : "$_id.uuid", 
					"scores" : {
						"$push" : {
							"score" : "$_id.score", 
							"value" : "$value"
						}
					}
				}
			}, 
			{ 
				"$project" : {
					"_id" : 1.0, 
					"scores" : {
						"$arrayToObject" : {
							"$map" : {
								"input" : "$scores", 
								"as" : "el", 
								"in" : {
									"k" : "$$el.score", 
									"v" : "$$el.value"
								}
							}
						}
					}
				}
			},
 			%s
        	{ 
            	"$skip" : %d
        	}, 
        	{ 
            	"$limit" : %d
        	}
    	]`

		match := `
		{ 
			"$match" : {
				%s
				%s
				%s
				"analytic_event_type" : "Score"
			}
		}, `

		instanceMatch := ""
		if request.Instance != "" {
			instanceMatch = fmt.Sprintf("\"instance_id\": \"%s\",", request.Instance)
		}

		modeMatch := ""
		if request.Mode != "" {
			modeMatch = fmt.Sprintf("\"game_mode_id\": \"%s\",", request.Mode)
		}

		gameMatch := ""
		if request.Game != "" {
			gameMatch = fmt.Sprintf("\"game_id\": \"%s\",", request.Game)
		}

		match = fmt.Sprintf(match, instanceMatch, modeMatch, gameMatch)

		sort := ""
		if request.Filter != "" {
			filters := strings.Split(request.Filter, ",")
			sort = `{ 
            	"$sort" : {
                	%s
            	}
        	},`
			sortTemplate := ``

			for i, filter := range filters {
				if (strings.HasPrefix(filter, "-")) {
					sortTemplate += "\"scores." + filter[1:] + "\" : 1.0"
				} else {
					sortTemplate += "\"scores." + filter + "\" : -1.0"
				}

				if (i+1 != len(filters)) {
					sortTemplate += ","
				}
			}
			sort = fmt.Sprintf(sort, sortTemplate)
		}

		pipeline = fmt.Sprintf(pipeline, match, sort, (page*length)-length, length)

		collection = Client.Database("Analytics").Collection("Events")
		opts := options.Aggregate()
		opts.SetAllowDiskUse(true)
		if cur, err = collection.Aggregate(r, mdb.MongoPipeline(pipeline), opts); err != nil {
			log.Fatal(err)
		}

		var results [] mongoResult
		for cur.Next(r) {
			var result mongoResult
			err := cur.Decode(&result)
			if err != nil {
				log.Fatal(err)
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
}
