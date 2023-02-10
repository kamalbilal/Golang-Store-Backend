package main

import (
	"database/sql"
	"log"
	"os"

	_db "kamal/database"
	"kamal/other"
	"kamal/print"
	redis "kamal/redis"
	route "kamal/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
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

	queries := _db.GetProductDataQuery(&db)

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

	setupRoutes(router, &db, &queries, useCors)
	other.LogHeapData()
	router.Run("localhost:8080")
	
	// log.Fatal(http.ListenAndServeTLS(":8080", "certificate/certificate.crt", "certificate/private.key", router))

}


func setupRoutes(router *gin.Engine, db *sql.DB, queries *_db.Queries , useCors bool) {
	// COOKIESIGNEDSECRET := loadEnv("COOKIESIGNEDSECRET")
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

	// store := cookie.NewStore([]byte(COOKIESIGNEDSECRET))
	// router.Use(sessions.Sessions("mysession", store))

	router.POST("/getProductData", func(c *gin.Context) {
		route.GetProductData(c, queries)
	})
	router.POST("/getwishlist", func(c *gin.Context) {
		route.GetWishlist(c, JWTSECRET, queries)
	})
	router.POST("/getMoreWishlist", func(c *gin.Context) {
		route.GetCertainWishlist(c, JWTSECRET, queries)
	})
	router.POST("/signup", func(c *gin.Context) {
		route.Signup(c, JWTSECRET, queries)
	})
	router.POST("/login", func(c *gin.Context) {
		route.Login(c, JWTSECRET, queries)
	})
	router.POST("/logout", func(c *gin.Context) {
		route.Logout(c)
	})
	router.POST("/getUserData", func(c *gin.Context) {
		route.GetUserData(c, JWTSECRET, queries)
	})
	router.DELETE("/removefromcart", func(c *gin.Context) {
		route.DeleteProductFromCart(c, JWTSECRET, queries)
	})
	router.POST("/addtowishlist", func(c *gin.Context) {
		route.AddProductToWishList(c, JWTSECRET, queries)
	})
	router.POST("/addtocart", func(c *gin.Context) {
		route.AddProductToCart(c, JWTSECRET, queries)
	})
	router.POST("/createNewList", func(c *gin.Context) {
		route.CreateNewListInWishlist(c, JWTSECRET, queries)
	})
	router.POST("/updateWishListName", func(c *gin.Context) {
		route.UpdateWishListName(c, JWTSECRET, queries)
	})
	router.POST("/deleteWishList", func(c *gin.Context) {
		route.DeleteWishList(c, JWTSECRET, queries)
	})
	router.GET("/get", func(c *gin.Context) {
		route.Test(c, JWTSECRET, queries)
	})
}

