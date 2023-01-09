package cookie

import (
	"time"

	"github.com/gin-gonic/gin"
)
func SetCookie(c *gin.Context, token string) {
    // Set the expiration time for the cookie.
    expires := time.Now().Add(time.Hour * 240) // expires in 240 hours

    // Set the cookie with the given options.
    c.SetCookie("token", token, int(expires.Unix()), "/", "", false, true)
}