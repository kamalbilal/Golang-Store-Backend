package limiter

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"kamal/redis"
)

func SetLimit(ip *string, route *string, rate int)  {
	redisKeyName := "rate-limit-" + *ip + "-" + *route
	fmt.Println(redisKeyName)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(rate); err != nil {
		fmt.Println("Error encoding struct:", err)
		return
	}

	redis.SetKey(redisKeyName, buf.Bytes(), 60 * 5)
}

func GetLimitRate(ip *string, route *string) (int, int) {
	var value int
	redisKeyName := "rate-limit-" + *ip + "-" + *route
	fmt.Println(redisKeyName)
	exist, val := redis.GetKey(redisKeyName)
	if exist {
		dec := gob.NewDecoder(bytes.NewReader(val))
		if err := dec.Decode(&value); err != nil {
			fmt.Println("Error decoding struct:", err)
			return 0, -1
		}
		remainingTime := redis.GetRemainingExpiryTime(redisKeyName)

		return value, remainingTime
	}
	return 0, -1
}