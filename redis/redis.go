package redis

import (
	"fmt"
	"time"

	"github.com/go-redis/redis"
)

var client *redis.Client

func CreateClient() {
	client = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // use default Redis port
		Password: "",                // no password set
		DB:       0,                 // use default DB
	})

	_, err := client.Ping().Result()
	if err != nil {
		fmt.Println("Redis failed to connect:", err)
		return
	}

	fmt.Println("Redis Successfully Connected")
}

func GetKey(keyName string) (bool, []byte)  {
	val, err := client.Get(keyName).Bytes()
	if err != nil {
		return false, nil
	}
	return true, val
}
func SetKey(keyName string, value []byte, expireInSec int) bool  {
	err := client.Set(keyName, value, time.Duration(expireInSec)*time.Second).Err()
	if err != nil {
		panic(err)
	}
	return true
}
func IncreaseExpirationTime(keyName string, expireInSec int) bool {
	// Get current expiration time of key
	ttl, err := client.TTL(keyName).Result()
	if err != nil {
		return false
	}

	// Set new expiration time for key
	expiration := ttl + time.Duration(expireInSec)*time.Second
	err = client.Expire(keyName, expiration).Err()
	if err != nil {
		return false
	}

	return true
}

func GetRemainingExpiryTime(redisKeyName string) int {
	// Get remaining time-to-live of key
	ttl, err := client.TTL(redisKeyName).Result()
	if err != nil {
		fmt.Println("Error getting ttl of key:", err)
		return -1 
	}
	return int(ttl.Seconds())
}