package statistics

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

var Client *mongo.Client

func Register(r *gin.Engine, client *mongo.Client) {
	Client = client

	util.CachedGET(r, "/stats/favorite/:time", favoriteHandler)
}

type favoriteRequest struct {
	Time int64 `uri:"time" binding:""`
}

type favoriteResult struct {
	ID    string `bson:"_id"`
	Count int32  `bson:"count"`
}

func favoriteHandler(c *gin.Context) []byte {
	var err error
	var collection *mongo.Collection
	var cur *mongo.Cursor

	var request favoriteRequest
	if err = c.ShouldBindUri(&request); err != nil {
		c.JSON(400, gin.H{"error": err})
		return nil
	}

	pipeline := `[
        { 
            "$match" : {
                "time_code" : {
                    "$gt" : %d
                }, 
                "analytic_event_type" : "Finish"
            }
        }, 
        { 
            "$group" : {
                "_id" : "$game_mode_id", 
                "count" : {
                    "$sum" : 1.0
                }
            }
        }, 
        { 
            "$sort" : {
                "count" : -1.0
            }
        }, 
        { 
            "$limit" : 1.0
        }
    ]`
	pipeline = fmt.Sprintf(pipeline, request.Time)

	collection = Client.Database("Analytics").Collection("Events")
	opts := options.Aggregate()
	opts.SetAllowDiskUse(true)
	if cur, err = collection.Aggregate(c, mdb.MongoPipeline(pipeline), opts); err != nil {
		c.JSON(500, gin.H{"error": err})
		return nil
	}

	var result favoriteResult
	for cur.Next(c) {
		err := cur.Decode(&result)
		if err != nil {
			log.Fatal(err)
		}
	}

	json, err := json2.MarshalIndent(result, "", "    ")
	if err != nil {
		c.JSON(500, gin.H{"error": err})
		return nil
	}

	return json
}
