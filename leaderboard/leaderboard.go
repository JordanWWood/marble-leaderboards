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
	User     string `uri:"user" binding:""`
}

type mongoResult struct {
	Entries    []mongoEntry `bson:"entries"`
	TotalCount int32         `bson:"total_count"`
}

type mongoEntry struct {
	ID       string           `bson:"uuid"`
	Name     string           `bson:"name"`
	Scores   map[string]int32 `bson:"scores"`
	Position int32            `bson:"position"`
}

var Client *mongo.Client

func Register(r *gin.Engine, client *mongo.Client) {
	Client = client

	util.CachedGET(r, "/leaderboard", leaderboardHandler)
	util.CachedGET(r, "/leaderboard/:game", leaderboardHandler)
	util.CachedGET(r, "/leaderboard/:game/:mode", leaderboardHandler)
	util.CachedGET(r, "/leaderboard/:game/:mode/:filter", leaderboardHandler)
	util.CachedGET(r, "/leaderboard/:game/:mode/:filter/user/:user", leaderboardHandler)
	util.CachedGET(r, "/leaderboard/:game/:mode/:filter/instance/:instance", leaderboardHandler)
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

	pipeline := `
		[
			%s
			{ 
				"$project" : {
					"value" : 1.0, 
					"score_field" : 1.0, 
					"player_uuid" : 1.0,
					"player_name" : 1.0, 
					"winners" : 1.0, 
					"losers" : 1.0
				}
			}, 
			{ 
				"$group" : {
					"_id" : {
						"uuid" : "$player_uuid", 
						"name" : "$player_name", 
						"score" : "$score_field"
					}, 
					"value" : {
						"$sum" : "$value"
					}
				}
			}, 
			{ 
				"$group" : {
					"_id" : { 
						"uuid" : "$_id.uuid", 
						"name" : "$_id.name" 
					}, 
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
					"uuid": "$_id.uuid",
    				"name": "$_id.name", 
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
				"$group" : {
					"_id" : null, 
					"items" : {
						"$push" : "$$ROOT"
					}
				}
			}, 
			{ 
				"$unwind" : {
					"path" : "$items", 
					"includeArrayIndex" : "items.position", 
					"preserveNullAndEmptyArrays" : false
				}
			}, 
			{ 
				"$replaceRoot" : {
					"newRoot" : "$items"
				}
			}, 
			%s
			{ 
				"$group" : {
					"_id" : null, 
					"entries" : {
						"$push" : "$$ROOT"
					}, 
					"count" : {
						"$sum" : 1.0
					}
				}
			}, 
			{ 
				"$project" : {
					"entries" : {
						"$slice" : [
							"$entries", 
							%d, 
							%d
						]
					}, 
					"total_count" : "$count"
				}
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
		sort = `
			{
				"$match" : {
					%s
				}
			},
			{ 
            	"$sort" : {
                	%s
            	}
        	},`
		sortTemplate := ``
		matchTemplate := ``

		for i, filter := range filters {
			if strings.HasPrefix(filter, "-") {
				if i == 0 {
					matchTemplate += "\"scores." + filter[1:] + "\" : { \"$exists\" : true, \"$ne\" : null }"
				}

				sortTemplate += "\"scores." + filter[1:] + "\" : 1.0"
			} else {
				if i == 0 {
					matchTemplate += "\"scores." + filter + "\" : { \"$exists\" : true, \"$ne\" : null }"
				}

				sortTemplate += "\"scores." + filter + "\" : -1.0"
			}

			if i+1 != len(filters) {
				sortTemplate += ","
			}
		}
		sort = fmt.Sprintf(sort, matchTemplate, sortTemplate)
	}

	userMatch := ``
	if request.User != "" {
		userMatch += `
			{ 
				"$match" : {
					"uuid" : "%s"
				}
			},`

		userMatch = fmt.Sprintf(userMatch, request.User)
	}

	pipeline = fmt.Sprintf(pipeline, match, sort, userMatch, (page*length)-length, length)

	collection = Client.Database("Analytics").Collection("Events")
	opts := options.Aggregate()
	opts.SetAllowDiskUse(true)
	if cur, err = collection.Aggregate(r, mdb.MongoPipeline(pipeline), opts); err != nil {
		r.JSON(500, gin.H{"err": err})
		return nil
	}

	var results [] mongoResult
	for cur.Next(r) {
		var result mongoResult
		err := cur.Decode(&result)
		if err != nil {
			r.JSON(500, gin.H{"err": err})
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
