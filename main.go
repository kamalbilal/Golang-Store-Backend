package main

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_db "kamal/database"
	_err "kamal/errors"
	"kamal/print"
	limiter "kamal/rateLimiter"
	redis "kamal/redis"
	myCookie "kamal/setCookie"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
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
	defer print.Str("\n-----------END-----------\n")
	redis.CreateClient()

	db, err := _db.ConnectToDatabase()
	if err != nil {
		panic(err)
	}
	defer db.Close()

	queries := _db.GetProductDataQuery(db)

	defer queries.GetProductData.Close()
	defer queries.EmailAlreadyExist.Close()
	defer queries.SignUpUser.Close()
	defer queries.CreateDefaultWishlist.Close()
	defer queries.Login.Close()
	defer queries.GetUserAllWishListsNamesIds.Close()
	defer queries.GetUserWishListData.Close()
	defer queries.GetUserCertainWishListData.Close()
	defer queries.GetUserData.Close()
	defer queries.GetUserCartData.Close()

	print.Str("Successfully connected to the database!")

	router := gin.Default()
	var useCors = true

	setupRoutes(router, db, queries, useCors)
	router.Run("localhost:8080")
	
	// log.Fatal(http.ListenAndServeTLS(":8080", "certificate/certificate.crt", "certificate/private.key", router))

}


func setupRoutes(router *gin.Engine, db *sql.DB, queries *_db.Queries , useCors bool) {
	COOKIESIGNEDSECRET := loadEnv("COOKIESIGNEDSECRET")
	JWTSECRET := loadEnv("JWTSECRET")

	if useCors {
		config := cors.DefaultConfig()
		config.AllowMethods = []string{"GET", "DELETE", "POST"}
		config.AllowCredentials = true
		config.AllowOrigins = []string{"http://localhost:3000", "http://localhost:3001","https://localhost:3000", "https://localhost:3001"}
		router.Use(cors.New(config))
	} else {
		router.Use(cors.Default())
	}

	store := cookie.NewStore([]byte(COOKIESIGNEDSECRET))
	router.Use(sessions.Sessions("mysession", store))
	router.POST("/getProductData", func(c *gin.Context) {
		getProductData(c, queries)
	})
	router.POST("/getwishlist", func(c *gin.Context) {
		getWishlist(c, JWTSECRET, queries)
	})
	router.POST("/getMoreWishlist", func(c *gin.Context) {
		getCertainWishlist(c, JWTSECRET, queries)
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
	router.POST("/getUserData", func(c *gin.Context) {
		getUserData(c, JWTSECRET, queries)
	})
	router.DELETE("/removefromcart", func(c *gin.Context) {
		deleteProductFromCart(c, JWTSECRET, queries)
	})
	router.POST("/addtowishlist", func(c *gin.Context) {
		addProductToWishList(c, JWTSECRET, queries)
	})
	router.POST("/addtocart", func(c *gin.Context) {
		addProductToCart(c, JWTSECRET, queries)
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
		_err.AbortRequestWithError(c, &currentRoute, http.StatusTooManyRequests, gin.H{"error": true,"success": false, "code": "To many requests", "waitForSeconds": &remainingTime}, true)
		return
	}
	limiter.SetLimit(&ip, &currentRoute, currentRate + 1, 60 * 5)

	var productId getProductDataPayload
	if err := c.ShouldBindJSON(&productId); err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound, gin.H{"error": true,"success": false, "code": "productId params not found or are of invalid type"}, true)
		return
	}

	if productId.Id == 0 {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound, gin.H{"error": true,"success": false, "code": "Required field are empty"}, true)
		return
	}

	var data getProductDataDB

	redisKeyName := "getProductData-" + strconv.Itoa(productId.Id)
	exist, val := redis.GetKey(&redisKeyName)
	if exist {
		print.Str("From Redis")
		dec := gob.NewDecoder(bytes.NewReader(val))
		if err := dec.Decode(&data); err != nil {
			print.Str("Error decoding struct: " , err)
			return
		}
		redis.IncreaseExpirationTime(redisKeyName, 20) // increase 20 seconds again
		c.AbortWithStatusJSON(http.StatusOK, &data)
		return
	}

	print.Str("From Database")

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
		if err == sql.ErrNoRows {
			_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound, gin.H{"error": true, "code": "Product not found!"}, true)
			return
		}
		print.Str(err.Error())
		_err.AbortRequestWithError(c, &currentRoute, http.StatusInternalServerError, gin.H{"error": true, "code": "Something wrong!"}, true)
		return
	}

	// redis
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		print.Str("Error encoding struct: " , err)
		return
	}

	redis.SetKey(redisKeyName, buf.Bytes(), 20)


	c.AbortWithStatusJSON(http.StatusOK, &data)
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
		_err.AbortRequestWithError(c, &currentRoute, http.StatusTooManyRequests, gin.H{"error": true,"success": false, "code": "To many requests", "waitForSeconds": &remainingTime}, true)
		return
	}
	limiter.SetLimit(&ip, &currentRoute, currentRate + 1, 60)

	var signup signupPayload
	if err := c.ShouldBindJSON(&signup); err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound, gin.H{"error": true,"success": false, "code": "productId params not found or are of invalid type"}, true)
		return
	}

	if signup.Email == "" || signup.Password == "" {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound, gin.H{"error": true,"success": false, "code": "Required field are empty"}, true)
		return
	}

	var emailAlreadyExist sql.NullString
	err := queries.EmailAlreadyExist.QueryRow(signup.Email).Scan(&emailAlreadyExist)
	if err != nil {
		// handle error
		// do not write "return" here
		print.Str(err.Error())
	}

	if emailAlreadyExist.Valid {
		// email already exists
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{ "error": true, "success": false, "reason": "Email already exist." })
		return
	} else {
		// email does not exist
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(signup.Password), bcrypt.DefaultCost)
		if err != nil {
			print.Str(err.Error())
		}

		signup.HashedPassword = string(hashedPassword)

		tx, err := queries.DB.Begin()
		if err != nil {
			print.Str("Error beginning transaction: " , err)
			tx.Rollback()
			_err.AbortRequestWithError(c, &currentRoute, http.StatusInternalServerError, gin.H{ "error": true, "success": false, "reason": "Something's wrong here" }, true)
			return
		}

		var id int
		err2 := tx.Stmt(queries.SignUpUser).QueryRow(signup.Email, signup.HashedPassword).Scan(&id)
		if err2 != nil {
			print.Str(err2.Error())
			tx.Rollback()
			_err.AbortRequestWithError(c, &currentRoute, http.StatusInternalServerError, gin.H{ "error": true, "success": false, "reason": "Could not sign up, Something's wrong." }, true)
			return
		}
		
		_, err3 := tx.Stmt(queries.CreateDefaultWishlist).Exec(id, "Default")
		if err3 != nil {
			print.Str(err3.Error())
			tx.Rollback()
			_err.AbortRequestWithError(c, &currentRoute, http.StatusInternalServerError, gin.H{ "error": true, "success": false, "reason": "Could not exec command, Something's wrong." }, true)
			return
		}

		err = tx.Commit()
        if err != nil {
            print.Str("Error committing transaction: " , err)
			_err.AbortRequestWithError(c, &currentRoute, http.StatusInternalServerError, gin.H{ "error": true, "success": false, "reason": "Something's wrong" }, true)
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
			_err.AbortRequestWithError(c, &currentRoute, http.StatusBadRequest, gin.H{ "error": true,"success": false, "reason": "Server error" }, true)
			return
		}
	
		myCookie.SetCookie(c, token)
		c.AbortWithStatusJSON(http.StatusCreated, gin.H{  "error": false, "success": true, "email": &signup.Email })
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
		_err.AbortRequestWithError(c, &currentRoute, http.StatusTooManyRequests, gin.H{"error": true,"success": false, "code": "To many requests", "waitForSeconds": &remainingTime}, true)
		return
	}
	limiter.SetLimit(&ip, &currentRoute, currentRate + 1, 60)

	var login loginPayload
	if err := c.ShouldBindJSON(&login); err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound, gin.H{"error": true,"success": false, "code": "productId params not found or are of invalid type"}, true)
		return
	}

	if login.Email == "" || login.Password == "" {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound, gin.H{"error": true,"success": false, "code": "Required field are empty"}, true)
		return
	}
	
	var loginDBData loginDB
	err := queries.Login.QueryRow(login.Email).Scan(&loginDBData.Id, &loginDBData.Email, &loginDBData.HashedPassword)
	if err != nil {
		print.Str(err.Error())
		_err.AbortRequestWithError(c, &currentRoute, http.StatusUnauthorized, gin.H{ "error": true, "success": false, "reason": "Credentials Error 1" }, true)
		return
	}

	err3 := bcrypt.CompareHashAndPassword([]byte(loginDBData.HashedPassword), []byte(login.Password))
	if err3 != nil {
		// password is invalid
		_err.AbortRequestWithError(c, &currentRoute, http.StatusUnauthorized, gin.H{ "error": true, "success": false, "reason": "Credentials Error" }, true)
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
			_err.AbortRequestWithError(c, &currentRoute, http.StatusBadRequest, gin.H{ "error": true, "success": false, "reason": "Server error" }, true)
			return
		}
	
		myCookie.SetCookie(c, token)
		c.AbortWithStatusJSON(http.StatusCreated, gin.H{  "error": false, "success": true, "email": &loginDBData.Email })
	}

}

func logout(c *gin.Context)  {
	var cookieName = "token"
	var currentRoute = "logout"
	cookie := myCookie.CookieExist(c, &cookieName)
	if cookie.Exists == false {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound, gin.H{"error": true, "success": false, "code": "cookie not found"}, true)
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
	if currentRate >= 10 {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusTooManyRequests,  gin.H{"error": true,"success": false, "code": "To many requests", "waitForSeconds": remainingTime}, true)
		return
	}
	limiter.SetLimit(&ip, &currentRoute, currentRate + 1, 60)

	cookie, err := c.Cookie("token")
	if err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 3" }, true)
		return
	}

	// print.Str(cookie)
	// Create a new JWT token using the token string and the secret key
	token, err := jwt.Parse(cookie, func(t *jwt.Token) (interface{}, error) {
		return []byte(JWTSECRET), nil
	})

	if err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 5" }, true)
		return
	}

	// Check if the JWT token is valid
	if !token.Valid {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 7" }, true)
		return
	}
	
	// If the JWT token is valid, get the id from the claims
	var idTemp float64
	var id int
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 8" }, true)
		return
	}

	// print.Str(claims)
	idTemp, ok = claims["id"].(float64)
	if !ok {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 9" }, true)
		return
	}
	
	id = int(idTemp)
	var userWishList UserWishListNames

	// redis get
	redisKeyName := "getWishlist-" + strconv.Itoa(id)
	exist, val := redis.GetKey(&redisKeyName)
	if exist {
		print.Str("From Redis")
		dec := gob.NewDecoder(bytes.NewReader(val))
		if err := dec.Decode(&userWishList); err != nil {
			print.Str("Error decoding struct:", err)
		} else {
			redis.IncreaseExpirationTime(redisKeyName, 20) // increase 20 seconds again
			c.AbortWithStatusJSON(http.StatusOK, &userWishList)
			return
		}
	}
	// redis end

	print.Str("From Database")
	err2 := queries.GetUserAllWishListsNamesIds.QueryRow(id).Scan(&userWishList.WishListNames, &userWishList.WishListIds)
	if err2 != nil {
		print.Str(err2.Error())
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 10" }, true)
		return
	}
	
    var wishListIdsData []int
    err = json.Unmarshal(userWishList.WishListIds.Bytes, &wishListIdsData)
    if err != nil {
		print.Str(err.Error())
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 11" }, true)
		return
    }
	
	var wishListNamesData []string
    err = json.Unmarshal(userWishList.WishListNames.Bytes, &wishListNamesData)
    if err != nil {
		print.Str(err.Error())
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 12" }, true)
		return
    }

	objData := make(map[string][]WishListData)
	
	for index, element := range wishListIdsData {
		var arrData []WishListData
		rows, err2 := queries.GetUserWishListData.Query(element, 5)

		if err2 != nil {
			print.Str(err2.Error())
			_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 13" }, true)
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
				print.Str(err)
				_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 14" }, true)
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
		print.Str("Error encoding struct:", err)
	}

	redis.SetKey(redisKeyName, buf.Bytes(), 20)

	c.AbortWithStatusJSON(http.StatusOK, &userWishList)

}

type CertainWishlistPayload struct {
	PageNumber int
	WishlistId int
	WishlistName string
}

func getCertainWishlist(c *gin.Context, JWTSECRET string, queries *_db.Queries)  {
	// rate limiter
	ip := c.ClientIP()
	var currentRoute = "getCertainWishlist"
	currentRate, remainingTime := limiter.GetLimitRate(&ip, &currentRoute)
	if currentRate >= 10 {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusTooManyRequests,  gin.H{"error": true,"success": false, "code": "To many requests", "waitForSeconds": remainingTime}, true)
		return
	}
	limiter.SetLimit(&ip, &currentRoute, currentRate + 1, 60)

	var certainWishlistData CertainWishlistPayload

	if err := c.ShouldBindJSON(&certainWishlistData); err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{"error": true,"success": false, "code": "Params not found or are of invalid type"}, true)
		return
	}

	
	if certainWishlistData.WishlistId < 1 || certainWishlistData.PageNumber < 1 || certainWishlistData.WishlistName == "" {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{"error": true,"success": false, "code": "Required field are empty"}, true)
		return
	}

	cookie, err := c.Cookie("token")
	if err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 3" }, true)
		return
	}

	// print.Str(cookie)
	// Create a new JWT token using the token string and the secret key
	token, err := jwt.Parse(cookie, func(t *jwt.Token) (interface{}, error) {
		return []byte(JWTSECRET), nil
	})

	if err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 5" }, true)
		return
	}

	// Check if the JWT token is valid
	if !token.Valid {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 7" }, true)
		return
	}
	
	// If the JWT token is valid, get the id from the claims
	var idTemp float64
	var userId int
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 8" }, true)
		return
	}
	// print.Str(claims)
	idTemp, ok = claims["id"].(float64)
	if !ok {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 9" }, true)
		return
	}
	
	userId = int(idTemp)
	arrData := []WishListData{}

	redisFirstKey := "getCertainWishlist-userId-" + strconv.Itoa(userId) + "-wishlistId-" + strconv.Itoa(certainWishlistData.WishlistId)
	redisSecondKey := "page-" + strconv.Itoa(certainWishlistData.PageNumber)

	// redis get 
	exist, val, err := redis.HMGet(redisFirstKey, redisSecondKey)
	if err != nil && exist {
		// do not write "return" here
		_err.AbortRequestWithError(nil, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "err": err.Error(), "reason": "error getting HMGet from redis" }, false)
	}
	if val != "" {
		print.Str("From Redis")
		err := json.Unmarshal([]byte(val), &arrData)
		if err != nil {
			// do not write "return" here
			_err.AbortRequestWithError(nil, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "err": err.Error(), "reason": "error converting json string to struct from redis" }, false)
		} else {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"data": &arrData, "wishlistId": &certainWishlistData.WishlistId, "wishlistName" : &certainWishlistData.WishlistName, "pageNumber": &certainWishlistData.PageNumber })
			return
		}
	}
	// redis end

	print.Str("From Database")
	var LIMIT = 5
	rows, err2 := queries.GetUserCertainWishListData.Query(userId, certainWishlistData.WishlistId, LIMIT, LIMIT * (certainWishlistData.PageNumber - 1))

	if err2 != nil {
		print.Str(err2.Error())
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 13" }, true)
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
			print.Str(err)
			_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 14" }, true)
			return
		}
		arrData = append(arrData, userWishListData)
	}

	// redis set
	jsonArrayData, err := json.Marshal(arrData)
	if err != nil {
		// do not write return here
		_err.AbortRequestWithError(nil, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "err": err.Error(), "reason": "error converting array to json for redis" }, false)

	}
	
	data := make(map[string]interface{})
	data[redisSecondKey] = &jsonArrayData
	
	// print.Str(redisFirstKey)
	// print.Str(redisSecondKey)
	
	err3 := redis.HMSet(redisFirstKey, &data, 20)
	if err3 != nil {
		// do not write return here
		_err.AbortRequestWithError(nil, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "err": err3.Error(), "reason": "error setting HMSet in redis" }, false)
	}
	
	c.AbortWithStatusJSON(http.StatusOK, gin.H{"data": &arrData, "wishlistId": &certainWishlistData.WishlistId, "wishlistName" : &certainWishlistData.WishlistName, "pageNumber": &certainWishlistData.PageNumber })
}

type UserData struct {
	Email string `json:"email"`
}

type UserCart struct {
    Title string `json:"title"`
    CartId int `json:"cartId"`
    ProductId int `json:"productId"`
    LongProductId int `json:"longProductId"`
    CartName string `json:"cartName"`
    SelectedImageUrl string `json:"selectedImageUrl"`
    SelectedPrice float32 `json:"selectedPrice"`
    SelectedQuantity int `json:"selectedQuantity"`
    SelectedDiscount float32 `json:"selectedDiscount"`
    SelectedProperties pgtype.JSONB `json:"selectedProperties"`
    SelectedShippingDetails pgtype.JSONB `json:"selectedShippingDetails"`
    SelectedShippingPrice float32 `json:"selectedShippingPrice"`
    MinPrice float32 `json:"minPrice"`
    MaxPrice float32 `json:"maxPrice"`
    MultiUnitName string `json:"multiUnitName"`
    OddUnitName string `json:"oddUnitName"`
    MaxPurchaseLimit int `json:"maxPurchaseLimit"`
    BuyLimitText string `json:"buyLimitText"`
    QuantityAvaliable int `json:"quantityAvaliable"`
    PriceListInNames pgtype.JSONB `json:"priceList_InNames"`
    PriceListInNumbers pgtype.JSONB `json:"priceList_InNumbers"`
    PriceListData pgtype.JSONB `json:"priceList_Data"`
}

type UserWishListNamesIds struct {
	WishListNames pgtype.JSON `json:"wishListNames"`
	WishListIds pgtype.JSON `json:"wishListIds"`
}


func getUserData(c *gin.Context, JWTSECRET string, queries *_db.Queries) {
	// rate limiter
	ip := c.ClientIP()
	var currentRoute = "getUserData"
	currentRate, remainingTime := limiter.GetLimitRate(&ip, &currentRoute)
	if currentRate >= 10 {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusTooManyRequests,  gin.H{"error": true,"success": false, "code": "To many requests", "waitForSeconds": remainingTime}, true)
		return
	}
	limiter.SetLimit(&ip, &currentRoute, currentRate + 1, 60)

	cookie, err := c.Cookie("token")
	if err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 3" }, true)
		return
	}

	print.Str(cookie)

	token, err := jwt.Parse(cookie, func(t *jwt.Token) (interface{}, error) {
		return []byte(JWTSECRET), nil
	})

	if err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 5" }, true)
		return
	}

	// Check if the JWT token is valid
	if !token.Valid {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 7" }, true)
		return
	}
	
	// If the JWT token is valid, get the id from the claims
	var idTemp float64
	var userId int
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 8" }, true)
		return
	}
	// print.Str(claims)
	idTemp, ok = claims["id"].(float64)
	if !ok {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 9" }, true)
		return
	}
	
	userId = int(idTemp)

	print.Str(userId)
	
	var userData UserData
	data := make(map[string]interface{})
	err = queries.GetUserData.QueryRow(userId).Scan(&userData.Email)
	if err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 10" }, true)
		return
	}
	data["userData"] = &userData
	
	rows, err := queries.GetUserCartData.Query(userId)
	if err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 11" }, true)
		return
	}
	
	arrData := []UserCart{}
	for rows.Next() {
		var userCart UserCart
		if err := rows.Scan(&userCart.Title,
			&userCart.CartId,
			&userCart.ProductId,
			&userCart.LongProductId,
			&userCart.CartName,
			&userCart.SelectedImageUrl,
			&userCart.SelectedPrice,
			&userCart.SelectedQuantity,
			&userCart.SelectedDiscount,
			&userCart.SelectedProperties,
			&userCart.SelectedShippingDetails,
			&userCart.SelectedShippingPrice,
			&userCart.MinPrice,
			&userCart.MaxPrice,
			&userCart.MultiUnitName,
			&userCart.OddUnitName,
			&userCart.MaxPurchaseLimit,
			&userCart.BuyLimitText,
			&userCart.QuantityAvaliable,
			&userCart.PriceListInNames,
			&userCart.PriceListInNumbers,
			&userCart.PriceListData); err != nil {
			print.Str(err)
			_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 12" }, true)
			return
		}
		arrData = append(arrData, userCart)
	}
	
	data["userCart"] = &arrData

	var userWishList UserWishListNamesIds
	err2 := queries.GetUserAllWishListsNamesIds.QueryRow(userId).Scan(&userWishList.WishListNames, &userWishList.WishListIds)
	if err2 != nil {
		print.Str(err2.Error())
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 12" }, true)
		return
	}

	data["userWishList"] = &userWishList

	c.AbortWithStatusJSON(http.StatusOK, gin.H{ "success": true, "error": false, "data": &data })
}

type DeleteProductFromCartPayload struct {
	ProductId int
	CartId int
}

func deleteProductFromCart(c *gin.Context, JWTSECRET string, queries *_db.Queries) {
	// rate limiter
	ip := c.ClientIP()
	var currentRoute = "deleteProductFromCart"
	currentRate, remainingTime := limiter.GetLimitRate(&ip, &currentRoute)
	if currentRate >= 10 {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusTooManyRequests,  gin.H{"error": true,"success": false, "code": "To many requests", "waitForSeconds": remainingTime}, true)
		return
	}
	limiter.SetLimit(&ip, &currentRoute, currentRate + 1, 60)


	var deleteProductFromCartData DeleteProductFromCartPayload

	if err := c.ShouldBindJSON(&deleteProductFromCartData); err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{"error": true,"success": false, "code": "Params not found or are of invalid type"}, true)
		return
	}

	
	if deleteProductFromCartData.ProductId < 1 || deleteProductFromCartData.CartId < 1 {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{"error": true,"success": false, "code": "Required field are empty"}, true)
		return
	}

	cookie, err := c.Cookie("token")
	if err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 3" }, true)
		return
	}

	print.Str(cookie)

	token, err := jwt.Parse(cookie, func(t *jwt.Token) (interface{}, error) {
		return []byte(JWTSECRET), nil
	})

	if err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 5" }, true)
		return
	}

	// Check if the JWT token is valid
	if !token.Valid {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 7" }, true)
		return
	}

	// If the JWT token is valid, get the id from the claims
	var idTemp float64
	var userId int
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 8" }, true)
		return
	}
	// print.Str(claims)
	idTemp, ok = claims["id"].(float64)
	if !ok {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 9" }, true)
		return
	}

	userId = int(idTemp)

	print.Str("From Database")
	var deletedId int
	err2 := queries.DeleteProductFromCart.QueryRow(deleteProductFromCartData.ProductId, userId, deleteProductFromCartData.CartId).Scan(&deletedId)
	if err2 != nil {
		print.Str(err2.Error())
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 10" }, true)
		return
	}

	c.AbortWithStatusJSON(http.StatusOK, gin.H{ "error": false, "success": true, "deletedId": deletedId  })
}

type AddProductToWishlistPayload struct {
    ProductId        int
    CartId           int
    WishListId   	 int
    SelectedImageUrl string
}

func addProductToWishList(c *gin.Context, JWTSECRET string, queries *_db.Queries) {
	// rate limiter
	ip := c.ClientIP()
	var currentRoute = "addProductToWishList"
	currentRate, remainingTime := limiter.GetLimitRate(&ip, &currentRoute)
	if currentRate >= 20 {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusTooManyRequests,  gin.H{"error": true,"success": false, "code": "To many requests", "waitForSeconds": remainingTime}, true)
		return
	}
	limiter.SetLimit(&ip, &currentRoute, currentRate + 1, 60)


	var addProductToWishlistData AddProductToWishlistPayload

	if err := c.ShouldBindJSON(&addProductToWishlistData); err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{"error": true,"success": false, "code": "Params not found or are of invalid type"}, true)
		return
	}

	
	if addProductToWishlistData.ProductId < 1 || addProductToWishlistData.CartId < 1 || addProductToWishlistData.WishListId < 1 || addProductToWishlistData.SelectedImageUrl == "" {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{"error": true,"success": false, "code": "Required field are empty"}, true)
		return
	}

	cookie, err := c.Cookie("token")
	if err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 3" }, true)
		return
	}

	print.Str(cookie)

	token, err := jwt.Parse(cookie, func(t *jwt.Token) (interface{}, error) {
		return []byte(JWTSECRET), nil
	})

	if err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 5" }, true)
		return
	}

	// Check if the JWT token is valid
	if !token.Valid {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 7" }, true)
		return
	}

	// If the JWT token is valid, get the id from the claims
	var idTemp float64
	var userId int
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 8" }, true)
		return
	}
	// print.Str(claims)
	idTemp, ok = claims["id"].(float64)
	if !ok {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 9" }, true)
		return
	}

	userId = int(idTemp)

	print.Str(userId)
	print.Str("From Database")

	tx, err := queries.DB.Begin()
		if err != nil {
			print.Str("Error beginning transaction: " , err)
			tx.Rollback()
			_err.AbortRequestWithError(c, &currentRoute, http.StatusInternalServerError, gin.H{ "error": true, "success": false, "reason": "Something's wrong here" }, true)
			return
		}

		var id int
		err2 := tx.Stmt(queries.AddProductToWishlist).QueryRow(userId, addProductToWishlistData.ProductId, addProductToWishlistData.WishListId, addProductToWishlistData.SelectedImageUrl, addProductToWishlistData.WishListId).Scan(&id)
		if err2 != nil {
			print.Str(err2.Error())
			tx.Rollback()
			_err.AbortRequestWithError(c, &currentRoute, http.StatusInternalServerError, gin.H{ "error": true, "success": false, "reason": "Could not sign up, Something's wrong." }, true)
			return
		}
		
		_, err3 := tx.Stmt(queries.DeleteProductFromCart).Exec(addProductToWishlistData.ProductId, userId, addProductToWishlistData.CartId)
		if err3 != nil {
			print.Str(err3.Error())
			tx.Rollback()
			_err.AbortRequestWithError(c, &currentRoute, http.StatusInternalServerError, gin.H{ "error": true, "success": false, "reason": "Could not exec command, Something's wrong." }, true)
			return
		}

		err = tx.Commit()
        if err != nil {
            print.Str("Error committing transaction: " , err)
			_err.AbortRequestWithError(c, &currentRoute, http.StatusInternalServerError, gin.H{ "error": true, "success": false, "reason": "Something's wrong" }, true)
			return
        }

	c.AbortWithStatusJSON(http.StatusOK, gin.H{ "error": false, "success": true, "id": id  })
}

type AddProductToCartPayload struct {
	ProductId int `binding:"required"`
	CartName string `binding:"required"`
  	Price float32 `binding:"required"`
	ShippingPrice float32
	Discount float32 
	Quantity int `binding:"required"`
	SelectedImageUrl string `binding:"required"`
	SelectedProperties pgtype.JSON `binding:"required"`
	ShippingDetails pgtype.JSON `binding:"required"`
}

func addProductToCart(c *gin.Context, JWTSECRET string, queries *_db.Queries)  {
	// rate limiter
	ip := c.ClientIP()
	var currentRoute = "addProductToCart"
	currentRate, remainingTime := limiter.GetLimitRate(&ip, &currentRoute)
	if currentRate >= 20 {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusTooManyRequests,  gin.H{"error": true,"success": false, "code": "To many requests", "waitForSeconds": remainingTime}, true)
		return
	}
	limiter.SetLimit(&ip, &currentRoute, currentRate + 1, 60)

	var addProductToCartData AddProductToCartPayload

	if err := c.ShouldBindWith(&addProductToCartData, binding.JSON); err != nil {
		print.Str(err)
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{"error": true,"success": false, "code": "Params not found or are of invalid type"}, true)
		return
	}
	
	// if addProductToCartData.ProductId < 1 || addProductToCartData.CartName == "" || addProductToCartData.Price < 0 || addProductToCartData.ShippingPrice < 0  {
	// 	_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{"error": true,"success": false, "code": "Required field are empty"}, true)
	// 	return
	// }

	cookie, err := c.Cookie("token")
	if err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 3" }, true)
		return
	}

	print.Str(cookie)

	token, err := jwt.Parse(cookie, func(t *jwt.Token) (interface{}, error) {
		return []byte(JWTSECRET), nil
	})

	if err != nil {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 5" }, true)
		return
	}

	// Check if the JWT token is valid
	if !token.Valid {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 7" }, true)
		return
	}

	// If the JWT token is valid, get the id from the claims
	var idTemp float64
	var userId int
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 8" }, true)
		return
	}
	// print.Str(claims)
	idTemp, ok = claims["id"].(float64)
	if !ok {
		_err.AbortRequestWithError(c, &currentRoute, http.StatusNotFound,  gin.H{ "error": true,"success": false, "code": "Error Code 9" }, true)
		return
	}

	userId = int(idTemp)

	print.Str(userId)
}