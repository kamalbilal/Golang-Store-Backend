package main

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	db "kamal/database"
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

	db, err := db.ConnectToDatabase()
	if err != nil {
		panic(err)
	}
	defer db.Close()

	fmt.Println("Successfully connected to the database!")
	
	router := gin.Default()
	
	setupRoutes(router, db)
	router.Run("localhost:8080")

}

// router.Use(cors.New(cors.Config{
// 	AllowOrigins: []string{"http://localhost:3000", "http://localhost:3001"},
// 	AllowCredentials: true,
// }))
func setupRoutes(router *gin.Engine, db *sql.DB) {
	COOKIESIGNEDSECRET := loadEnv("COOKIESIGNEDSECRET")
	JWTSECRET := loadEnv("JWTSECRET")
	router.Use(cors.Default())

	store := cookie.NewStore([]byte(COOKIESIGNEDSECRET))
	router.Use(sessions.Sessions("mysession", store))
	router.POST("/getProductData", func(c *gin.Context) {
		getProductData(c, db)
	})
	router.POST("/signup", func(c *gin.Context) {
		signup(c, db, JWTSECRET)
	})
	router.POST("/login", func(c *gin.Context) {
		login(c, db, JWTSECRET)
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

func getProductData(c *gin.Context, db *sql.DB)  {
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
	fmt.Println(val)
	if exist {
		fmt.Println("From Redis")
		dec := gob.NewDecoder(bytes.NewReader(val))
		if err := dec.Decode(&data); err != nil {
			fmt.Println("Error decoding struct:", err)
			return
		}
		redis.IncreaseExpirationTime(redisKeyName, 20) // increase 20 seconds again
		c.JSON(http.StatusOK, data)
		return
	}

	fmt.Println("From Database")

	err := db.QueryRow(`select 
	t_basicInfo.display as "_display",
	t_basicInfo.product_link as "link",
	t_basicInfo.minprice as "minPrice",
	t_basicInfo.maxprice as "maxPrice",
	t_basicInfo.discountnumber as "discountNumber",
	t_basicInfo.discount as "discount",
	t_basicInfo.minprice_afterdiscount as "minPrice_AfterDiscount",
	t_basicInfo.maxprice_afterdiscount as "maxPrice_AfterDiscount",
	t_basicInfo.multiunitname as "multiUnitName",
	t_basicInfo.oddunitname as "oddUnitName",
	t_basicInfo.maxpurchaselimit as "maxPurchaseLimit",
	t_basicInfo.buylimittext as "buyLimitText",
	t_basicInfo.quantityavaliable as "quantityAvaliable",
	t_basicInfo.comingSoon as "comingSoon",
	t_productId.id as "productId",
	t_productId.myProductId as "longProductId",
	t_titles.title,
	t_mainimages.image_link_array as "images",
	t_properties.property_array as "sizesColors",
	t_pricelist.byname as "priceList_InNames",
	t_pricelist.bynumber as "priceList_InNumbers",
	t_pricelist.bydata  as "priceList_Data",
	t_specs.specs as "specs",
	t_shippingdetails.shipping as "shipping",
	t_modifieddescription.description as "modified_description_content"
	from shop.t_productId
	join shop.t_basicInfo on t_basicInfo.foreign_id = t_productId.id
	join shop.t_titles on t_titles.foreign_id = t_productId.id
	join shop.t_mainimages on t_mainimages.foreign_id = t_productId.id
	join shop.t_properties on t_properties.foreign_id = t_productId.id
	join shop.t_pricelist on t_pricelist.foreign_id = t_productId.id
	join shop.t_specs on t_specs.foreign_id = t_productId.id
	join shop.t_shippingdetails on t_shippingdetails.foreign_id = t_productId.id
	join shop.t_modifieddescription on t_modifieddescription.foreign_id = t_productId.id
	where t_productId.myproductid = $1
	;`, productId.Id).Scan(&data.Display,
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Something wrong!"})
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


	c.JSON(http.StatusOK, data)
}

type signupPayload struct {
	Email string `json:"email"`
	Password string `json:"password"`
	HashedPassword string
}

func signup(c *gin.Context, db *sql.DB, JWTSECRET string) {
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
	err := db.QueryRow(`SELECT email FROM shop.t_users WHERE email = $1`, signup.Email).Scan(&emailAlreadyExist)
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

		var id int
		err2 := db.QueryRow(`INSERT into shop.t_users(email, password) Values($1, $2) RETURNING id;`, signup.Email, signup.HashedPassword).Scan(&id)
		if err2 != nil {
			// do not write "return" here
			fmt.Println(err.Error())
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

func login(c *gin.Context, db *sql.DB, JWTSECRET string) {
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
	err := db.QueryRow(`SELECT id, email, password from shop.t_users WHERE email = $1;`, login.Email).Scan(&loginDBData.Id, &loginDBData.Email, &loginDBData.HashedPassword)
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