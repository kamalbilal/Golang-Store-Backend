package _err

import (
	"github.com/gin-gonic/gin"
)

func AbortRequestWithError(c *gin.Context, routeName *string, status int, data gin.H, stopAll bool)  {
	if c != nil {
		c.AbortWithStatusJSON(status, data)
	}
}