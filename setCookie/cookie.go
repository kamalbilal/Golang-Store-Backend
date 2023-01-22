package cookie

import (
	"time"

	"github.com/gin-gonic/gin"
)
func SetCookie(c *gin.Context, token string) {
    // Set the expiration time for the cookie.
    expires := time.Now().Add(time.Hour * 240) // expires in 240 hours

    // Set the cookie with the given options.
    // c.SetSameSite(http.SameSiteNoneMode)
    c.SetCookie("token", token, int(expires.Unix()), "/", "", false, true)
}

func RemoveCookie(c *gin.Context, cookieName *string)  {
    c.SetCookie(*cookieName, "", -1, "", "", false, true)
}

type cookieExistStruct struct {
    Exists bool    `json:"exist"`
    Value  string `json:"value"`
}
func CookieExist(c *gin.Context, cookieName *string) cookieExistStruct {
    cookie, err := c.Cookie(*cookieName)

    if err != nil || cookie == "" {
		return cookieExistStruct{Exists: false, Value: ""}
	}
    return cookieExistStruct{Exists: true, Value: cookie}
}