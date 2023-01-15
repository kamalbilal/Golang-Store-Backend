package _err

import (
	"github.com/gin-gonic/gin"
)

func AbortRequestWithError(c *gin.Context, status int, data gin.H, stopAll bool)  {
	c.AbortWithStatusJSON(status, data)
}