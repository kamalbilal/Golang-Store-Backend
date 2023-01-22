package limiter

import (
	"bytes"
	"encoding/gob"
	"kamal/print"
	"kamal/redis"
)

func SetLimit(ip *string, route *string, rate int, expireInSec int)  {
	redisKeyName := "rate-limit-" + *ip + "-" + *route
	print.Str(redisKeyName)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(rate); err != nil {
		print.Str("Error encoding struct:", err)
		return
	}

	redis.SetKey(redisKeyName, buf.Bytes(), expireInSec)
}

func GetLimitRate(ip *string, route *string) (int, int) {
	var value int
	redisKeyName := "rate-limit-" + *ip + "-" + *route
	print.Str(redisKeyName)
	exist, val := redis.GetKey(&redisKeyName)
	if exist {
		dec := gob.NewDecoder(bytes.NewReader(val))
		if err := dec.Decode(&value); err != nil {
			print.Str("Error decoding struct:", err)
			return 0, -1
		}
		remainingTime := redis.GetRemainingExpiryTime(redisKeyName)

		return value, remainingTime
	}
	return 0, -1
}