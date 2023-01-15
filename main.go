package main

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_db "kamal/database"
	_err "kamal/errors"
	limiter "kamal/rateLimiter"
	redis "kamal/redis"
	myCookie "kamal/setCookie"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/jackc/pgtype"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

func loadEnv(keyName string) string {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	apiKey := os.Getenv(keyName)
	return apiKey
}

func main() {
	defer fmt.Printf("\n-----------END-----------\n")
	redis.CreateClient()

	db, err := _db.ConnectToDatabase()
	if err != nil {
		panic(err)
	}
	defer db.Close()

	queries := _db.GetProductDataQuery(db)

	defer queries.GetProductData.Close()
	defer queries.Login.Close()
	defer queries.SignUpUser.Close()
	defer queries.CreateDefaultWishlist.Close()
	defer queries.EmailAlreadyExist.Close()

	fmt.Println("Successfully connected to the database!")
	
	router := gin.Default()
	
	setupRoutes(router, db, queries)
	router.Run("localhost:8080")

}

// router.Use(cors.New(cors.Config{
// 	AllowOrigins: []string{"http://localhost:3000", "http://localhost:3001"},
// 	AllowCredentials: true,
// }))
func setupRoutes(router *gin.Engine, db *sql.DB, queries *_db.Queries) {
	COOKIESIGNEDSECRET := loadEnv("COOKIESIGNEDSECRET")
	JWTSECRET := loadEnv("JWTSECRET")
	router.Use(cors.Default())

	store := cookie.NewStore([]byte(COOKIESIGNEDSECRET))
	router.Use(sessions.Sessions("mysession", store))
	router.POST("/getProductData", func(c *gin.Context) {
		getProductData(c, queries)
	})
	router.POST("/getwishlist", func(c *gin.Context) {
		getWishlist(c, JWTSECRET, queries)
	})
	router.POST("/signup", func(c *gin.Context) {
		signup(c, JWTSECRET, queries)
	})
	router.POST("/login", func(c *gin.Context) {
		login(c, JWTSECRET, queries)
	})
	router.POST("/logout", func(c *gin.Context) {
		logout(c)
	})
}

type getProductDataPayload struct {
	Id int `json:"id"`
}
type getProductDataDB struct {
	Display bool `json:"_display"`
	Link string `json:"link"`
	MinPrice float32 `json:"minPrice"`
	MaxPrice float32 `json:"maxPrice"`
	DiscountNumber float32 `json:"discountNumber"`
	Discount string `json:"discount"`
	MinPriceAfterDiscount float32 `json:"minPrice_AfterDiscount"`
	MaxPriceAfterDiscount float32 `json:"maxPrice_AfterDiscount"`
	MultiUnitName string `json:"multiUnitName"`
	OddUnitName string `json:"oddUnitName"`
	MaxPurchaseLimit int `json:"maxPurchaseLimit"`
	BuyLimitText string `json:"buyLimitText"`
	QuantityAvaliable int `json:"quantityAvaliable"`
	ComingSoon bool `json:"comingSoon"`
	ProductId int `json:"productId"`
	LongProductId int `json:"longProductId"`
	Title string `json:"title"`
	Images pgtype.JSONB `json:"images"`
	SizesColors pgtype.JSONB `json:"sizesColors"`
	PriceListInNames pgtype.JSONB `json:"priceList_InNames"`
	PriceListInNumbers pgtype.JSONB `json:"priceList_InNumbers"`
	PriceListData pgtype.JSONB `json:"priceList_Data"`
	Specs pgtype.JSONB `json:"specs"`
	Shipping pgtype.JSONB `json:"shipping"`
	ModifiedDescriptionContent string `json:"modified_description_content"`
}

func getProductData(c *gin.Context, queries *_db.Queries)  {
	// rate limiter
	ip := c.ClientIP()
	var currentRoute = "getProductData"
	currentRate, remainingTime := limiter.GetLimitRate(&ip, &currentRoute)
	if currentRate >= 50 {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": true, "code": "To many requests", "waitForSeconds": remainingTime})
		return
	}
	limiter.SetLimit(&ip, &currentRoute, currentRate + 1)

	var productId getProductDataPayload
	if err := c.ShouldBindJSON(&productId); err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": true, "code": "productId params not found or are of invalid type"})
		return
	}

	if productId.Id == 0 {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": true, "code": "Required field are empty"})
		return
	}

	var data getProductDataDB

	redisKeyName := "getProductData-" + strconv.Itoa(productId.Id)
	exist, val := redis.GetKey(redisKeyName)
	if exist {
		fmt.Println("From Redis")
		dec := gob.NewDecoder(bytes.NewReader(val))
		if err := dec.Decode(&data); err != nil {
			fmt.Println("Error decoding struct:", err)
			return
		}
		redis.IncreaseExpirationTime(redisKeyName, 20) // increase 20 seconds again
		c.AbortWithStatusJSON(http.StatusOK, data)
		return
	}

	fmt.Println("From Database")

	err := queries.GetProductData.QueryRow(productId.Id).Scan(&data.Display,
		&data.Link,
		&data.MinPrice,
		&data.MaxPrice,
		&data.DiscountNumber,
		&data.Discount,
		&data.MinPriceAfterDiscount,
		&data.MaxPriceAfterDiscount,
		&data.MultiUnitName,
		&data.OddUnitName,
		&data.MaxPurchaseLimit,
		&data.BuyLimitText,
		&data.QuantityAvaliable,
		&data.ComingSoon,
		&data.ProductId,
		&data.LongProductId,
		&data.Title,
		&data.Images,
		&data.SizesColors,
		&data.PriceListInNames,
		&data.PriceListInNumbers,
		&data.PriceListData,
		&data.Specs,
		&data.Shipping,
		&data.ModifiedDescriptionContent)

	if err != nil {
		// c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		fmt.Println(err.Error())
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Something wrong!"})
		return
	}

	// redis
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		fmt.Println("Error encoding struct:", err)
		return
	}

	redis.SetKey(redisKeyName, buf.Bytes(), 20)


	c.AbortWithStatusJSON(http.StatusOK, data)
}

type signupPayload struct {
	Email string `json:"email"`
	Password string `json:"password"`
	HashedPassword string
}

func signup(c *gin.Context, JWTSECRET string, queries *_db.Queries) {
	// rate limiter
	ip := c.ClientIP()
	var currentRoute = "signup"
	currentRate, remainingTime := limiter.GetLimitRate(&ip, &currentRoute)
	if currentRate >= 5 {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": true, "code": "To many requests", "waitForSeconds": remainingTime})
		return
	}
	limiter.SetLimit(&ip, &currentRoute, currentRate + 1)

	var signup signupPayload
	if err := c.ShouldBindJSON(&signup); err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": true, "code": "productId params not found or are of invalid type"})
		return
	}

	if signup.Email == "" || signup.Password == "" {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": true, "code": "Required field are empty"})
		return
	}

	var emailAlreadyExist sql.NullString
	err := queries.EmailAlreadyExist.QueryRow(signup.Email).Scan(&emailAlreadyExist)
	if err != nil {
		// handle error
		// do not write "return" here
		fmt.Println(err.Error())
	}

	if emailAlreadyExist.Valid {
		// email already exists
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{ "error": true, "success": false, "reason": "Email already exist." })
		return
	} else {
		// email does not exist
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(signup.Password), bcrypt.DefaultCost)
		if err != nil {
			fmt.Println(err)
		}

		signup.HashedPassword = string(hashedPassword)

		tx, err := queries.DB.Begin()
		if err != nil {
			fmt.Println("Error beginning transaction:", err)
			tx.Rollback()
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{ "error": true, "success": false, "reason": "Something's wrong here" })
			return
		}

		var id int
		err2 := tx.Stmt(queries.SignUpUser).QueryRow(signup.Email, signup.HashedPassword).Scan(&id)
		if err2 != nil {
			fmt.Println(err2.Error())
			tx.Rollback()
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{ "error": true, "success": false, "reason": "Could not sign up, Something's wrong." })
			return
		}
		
		_, err3 := tx.Stmt(queries.CreateDefaultWishlist).Exec(id, "Default")
		if err3 != nil {
			fmt.Println(err3.Error())
			tx.Rollback()
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{ "error": true, "success": false, "reason": "Could not exec command, Something's wrong." })
			return
		}

		err = tx.Commit()
        if err != nil {
            fmt.Println("Error committing transaction:", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{ "error": true, "success": false, "reason": "Something's wrong" })
			return
        }

		// jwt
		claims := jwt.MapClaims{"id": id}
	
		// Set the expiration time for the JWT.
		claims["exp"] = time.Now().Add(time.Hour * 240).Unix()
	
		// Create a new signer using the specified secret key.
		signer := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		secret := []byte(JWTSECRET)
	
		// Sign and get the complete encoded token as a string.
		token, err := signer.SignedString(secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{ "error": true, "success": false, "reason": "Server error" })
			return
		}
	
		myCookie.SetCookie(c, token)
		c.AbortWithStatusJSON(http.StatusCreated, gin.H{  "error": false, "success": true, "email": signup.Email })
	}
}

type loginPayload struct {
	Email string `json:"email"`
	Password string `json:"password"`
}
type loginDB struct {
	Id int `json:"id"`
	Email string `json:"email"`
	HashedPassword string `json:"password"`
}

func login(c *gin.Context, JWTSECRET string, queries *_db.Queries) {
	// rate limiter
	ip := c.ClientIP()
	var currentRoute = "login"
	currentRate, remainingTime := limiter.GetLimitRate(&ip, &currentRoute)
	if currentRate >= 5 {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": true, "code": "To many requests", "waitForSeconds": remainingTime})
		return
	}
	limiter.SetLimit(&ip, &currentRoute, currentRate + 1)

	var login loginPayload
	if err := c.ShouldBindJSON(&login); err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": true, "code": "productId params not found or are of invalid type"})
		return
	}

	if login.Email == "" || login.Password == "" {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": true, "code": "Required field are empty"})
		return
	}
	
	var loginDBData loginDB
	err := queries.Login.QueryRow(login.Email).Scan(&loginDBData.Id, &loginDBData.Email, &loginDBData.HashedPassword)
	if err != nil {
		fmt.Println(err.Error())
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{ "error": true, "success": false, "reason": "Credentials Error 1" })
		return
	}

	err3 := bcrypt.CompareHashAndPassword([]byte(loginDBData.HashedPassword), []byte(login.Password))
	if err3 != nil {
		// password is invalid
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{ "error": true, "success": false, "reason": "Credentials Error" })
		return
	} else {
		// password is valid
		
		// jwt
		claims := jwt.MapClaims{"id": loginDBData.Id}
	
		// Set the expiration time for the JWT.
		claims["exp"] = time.Now().Add(time.Hour * 240).Unix()
	
		// Create a new signer using the specified secret key.
		signer := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		secret := []byte(JWTSECRET)
	
		// Sign and get the complete encoded token as a string.
		token, err := signer.SignedString(secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{ "error": true, "success": false, "reason": "Server error" })
			return
		}
	
		myCookie.SetCookie(c, token)
		c.AbortWithStatusJSON(http.StatusCreated, gin.H{  "error": false, "success": true, "email": loginDBData.Email })
	}

}

func logout(c *gin.Context)  {
	var cookieName = "token"
	cookie := myCookie.CookieExist(c, &cookieName)
	if cookie.Exists == false {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": true, "success": false, "code": "cookie not found"})
		return
	} else {
		myCookie.RemoveCookie(c, &cookieName)
		c.AbortWithStatusJSON(http.StatusOK, gin.H{"success": true})
	}
}

type UserWishListNames struct {
	WishListNames pgtype.JSON `json:"wishListNames"`
	WishListIds pgtype.JSON `json:"wishListIds"`
	WishListData map[string][]WishListData `json:"wishListData"`
}

type WishListData struct {
    Title          string `json:"title"`
    WishListId     int    `json:"wishListId"`
    ParentWishList int    `json:"parentWishListId"`
    SelectedImageUrl	string `json:"selectedImageUrl"`
    ProductId      int    `json:"productId"`
    LongProductId  int    `json:"longProductId"`
    WishListName   string `json:"wishListName"`
    MinPrice       float32    `json:"minPrice"`
    MaxPrice       float32    `json:"maxPrice"`
}

func getWishlist(c *gin.Context, JWTSECRET string, queries *_db.Queries)  {
	// rate limiter
	ip := c.ClientIP()
	var currentRoute = "getWishlist"
	currentRate, remainingTime := limiter.GetLimitRate(&ip, &currentRoute)
	if currentRate >= 100 {
		_err.AbortRequestWithError(c, http.StatusTooManyRequests, gin.H{"error": true, "code": "To many requests", "waitForSeconds": remainingTime}, true)
		return
	}
	limiter.SetLimit(&ip, &currentRoute, currentRate + 1)

	cookie, err := c.Cookie("token")
	if err != nil {
		_err.AbortRequestWithError(c, http.StatusNotFound, gin.H{ "error": true, "code": "Error Code 3" }, true)
		return
	}

	// fmt.Println(cookie)
	// Create a new JWT token using the token string and the secret key
	token, err := jwt.Parse(cookie, func(t *jwt.Token) (interface{}, error) {
		return []byte(JWTSECRET), nil
	})

	if err != nil {
		_err.AbortRequestWithError(c, http.StatusNotFound, gin.H{ "error": true, "code": "Error Code 5" }, true)
		return
	}

	// Check if the JWT token is valid
	if !token.Valid {
		_err.AbortRequestWithError(c, http.StatusNotFound, gin.H{ "error": true, "code": "Error Code 7" }, true)
		return
	}
	
	// If the JWT token is valid, get the id from the claims
	var idTemp float64
	var id int
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		_err.AbortRequestWithError(c, http.StatusNotFound, gin.H{ "error": true, "code": "Error Code 8" }, true)
		return
	}
	// fmt.Println(claims)
	idTemp, ok = claims["id"].(float64)
	if !ok {
		_err.AbortRequestWithError(c, http.StatusNotFound, gin.H{ "error": true, "code": "Error Code 9" }, true)
		return
	}
	
	id = int(idTemp)
	var userWishList UserWishListNames

	// redis get
	redisKeyName := "getWishlist-" + strconv.Itoa(id)
	exist, val := redis.GetKey(redisKeyName)
	if exist {
		fmt.Println("From Redis")
		dec := gob.NewDecoder(bytes.NewReader(val))
		if err := dec.Decode(&userWishList); err != nil {
			fmt.Println("Error decoding struct:", err)
		} else {
			redis.IncreaseExpirationTime(redisKeyName, 20) // increase 20 seconds again
			c.AbortWithStatusJSON(http.StatusOK, userWishList)
			return
		}
	}
	// redis end

	fmt.Println("From Database")
	err2 := queries.GetUserAllWishListsNames.QueryRow(id).Scan(&userWishList.WishListNames, &userWishList.WishListIds)
	if err2 != nil {
		fmt.Println(err2.Error())
		_err.AbortRequestWithError(c, http.StatusNotFound, gin.H{ "error": true, "code": "Error Code 10" }, true)
		return
	}
	
    var wishListIdsData []int
    err = json.Unmarshal(userWishList.WishListIds.Bytes, &wishListIdsData)
    if err != nil {
		fmt.Println(err.Error())
		_err.AbortRequestWithError(c, http.StatusNotFound, gin.H{ "error": true, "code": "Error Code 11" }, true)
		return
    }
	
	var wishListNamesData []string
    err = json.Unmarshal(userWishList.WishListNames.Bytes, &wishListNamesData)
    if err != nil {
		fmt.Println(err.Error())
		_err.AbortRequestWithError(c, http.StatusNotFound, gin.H{ "error": true, "code": "Error Code 12" }, true)
		return
    }

	objData := make(map[string][]WishListData)
	
	for index, element := range wishListIdsData {
		var arrData []WishListData
		rows, err2 := queries.GetUserWishListData.Query(element, 5)

		if err2 != nil {
			fmt.Println(err2.Error())
			_err.AbortRequestWithError(c, http.StatusNotFound, gin.H{ "error": true, "code": "Error Code 13" }, true)
			return
		}
		
		for rows.Next() {
			var userWishListData WishListData
			if err := rows.Scan(&userWishListData.Title, 
				&userWishListData.WishListId,
				&userWishListData.ParentWishList,
				&userWishListData.SelectedImageUrl,
				&userWishListData.ProductId,
				&userWishListData.LongProductId,
				&userWishListData.WishListName,
				&userWishListData.MinPrice,
				&userWishListData.MaxPrice); err != nil {
				fmt.Println(err)
				_err.AbortRequestWithError(c, http.StatusNotFound, gin.H{ "error": true, "code": "Error Code 14" }, true)
				return
			}
			arrData = append(arrData, userWishListData)
		}
		if arrData != nil {
			objData[wishListNamesData[index]] = arrData
		}
	}
	userWishList.WishListData = objData

	// redis set
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(userWishList); err != nil {
		fmt.Println("Error encoding struct:", err)
	}

	redis.SetKey(redisKeyName, buf.Bytes(), 20)

	c.AbortWithStatusJSON(http.StatusOK, userWishList)

}