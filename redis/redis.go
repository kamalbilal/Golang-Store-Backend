package redis

import (
	"errors"
	"kamal/print"
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
		print.Str("Redis failed to connect:", err)
		return
	}

	print.Str("Redis Successfully Connected")
}

func GetKey(keyName *string) (bool, []byte)  {
	val, err := client.Get(*keyName).Bytes()
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

func HMSet(firstKeyName string, value *map[string]interface{}, expireInSec int)  error  {
	
	err := client.HMSet(firstKeyName, *value).Err()
	if err != nil {
		return err
	}

	// set key expiration time to 60 seconds
	err2 := client.Expire(firstKeyName, time.Duration(expireInSec)*time.Second).Err()
	if err != nil {
		return err2
	}
	return nil
}

func HMexists(firstKeyName string, secondKeyName string) bool {
	exists, err := client.HExists(firstKeyName, secondKeyName).Result()
	if err != nil {
		return false
	} else {
		return exists
	}
}

func HMGet(firstKeyName string, secondKeyName string) (bool, string, error)  {
	exists := HMexists(firstKeyName, secondKeyName)
	if exists {
		val, err := client.HGet(firstKeyName, secondKeyName).Result()
		if err != nil {
			return true, "" , err
		}
		
		return true, val , nil
	}
	return false, "" , errors.New("key not found")
}

func IncreaseExpirationTime(keyName string, expireInSec int) bool {
	// Get current expiration time of key
	_, err := client.TTL(keyName).Result()
	if err != nil {
		return false
	}

	// Set new expiration time for key
	expiration := time.Duration(expireInSec)*time.Second
	// expiration := ttl + time.Duration(expireInSec)*time.Second   // use this to add 20 secs + previous time
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
		print.Str("Error getting ttl of key:", err)
		return -1 
	}
	return int(ttl.Seconds())
}