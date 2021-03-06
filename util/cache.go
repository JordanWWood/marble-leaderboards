package util

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"log"
	"os"
	"strings"
	"time"
)

var client *redis.Client

func SetupCache() {
	client = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS"),
		Password: "",
		DB:       0,
	})

	_, err := client.Ping().Result()
	if err != nil {
		log.Println("Failed to connect to redis... caching is offline. ", err)
	}
}

func CachedGET(r *gin.Engine, path string, action func(*gin.Context) []byte) {
	r.GET(path, func(c *gin.Context) {
		defer handlePanic()

		key := "leaderboards" + strings.Replace(path, "/", ".", -1) + "."
		for i, param := range c.Params {
			key += param.Value
			if i+1 != len(c.Params) {
				key += "."
			}
		}

		i := 0;
		for k, v := range c.Request.URL.Query() {
			key += fmt.Sprint(".", k, ".")
			for _, val := range v {
				key += val
			}

			i++;
		}

		val, err := client.Get(key).Result()
		if err != nil {
			response := action(c)
			if err == redis.Nil {
				err := client.Set(key, string(response), 5 * time.Minute).Err()
				if err != nil {
					log.Println("Failed to insert into cache: ", err)
				}
			}
			fmt.Fprint(c.Writer, string(response))

			return
		}

		fmt.Fprint(c.Writer, val)
	})
}

func handlePanic() {

}
