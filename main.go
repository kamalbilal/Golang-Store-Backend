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

	db "kamal/database"
	redis "kamal/redis"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
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

func setupRoutes(router *gin.Engine, db *sql.DB) {
	COOKIESIGNEDSECRET := loadEnv("COOKIESIGNEDSECRET")
	router.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:35000", "http://localhost:35001"},
		AllowCredentials: true,
	}))

	store := cookie.NewStore([]byte(COOKIESIGNEDSECRET))
	router.Use(sessions.Sessions("mysession", store))
	router.POST("/getProductData", func(c *gin.Context) {
		getProductData(c, db)
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
	Images string `json:"images"`
	SizesColors string `json:"sizesColors"`
	PriceListInNames string `json:"priceList_InNames"`
	PriceListInNumbers string `json:"priceList_InNumbers"`
	PriceListData string `json:"priceList_Data"`
	Specs string `json:"specs"`
	Shipping string `json:"shipping"`
	ModifiedDescriptionContent string `json:"modified_description_content"`
}

func getProductData(c *gin.Context, db *sql.DB)  {
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

	exist, val := redis.GetKey("getProductData-" + strconv.Itoa(productId.Id))
	if exist {
		fmt.Println("From Redis")
		dec := gob.NewDecoder(bytes.NewReader(val))
		if err := dec.Decode(&data); err != nil {
			fmt.Println("Error decoding struct:", err)
			return
		}
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
	json_agg(t_mainimages.image_link_array) as "images",
	json_agg(t_properties.property_array) as "sizesColors",
	json_agg(t_pricelist.byname) as "priceList_InNames",
	json_agg(t_pricelist.bynumber) as "priceList_InNumbers",
	json_agg(t_pricelist.bydata)  as "priceList_Data",
	json_agg(t_specs.specs) as "specs",
	json_agg(t_shippingdetails.shipping) as "shipping",
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
	GROUP BY 
	t_productId.id, 
	t_basicInfo.display,
	t_basicInfo.product_link,
	t_basicInfo.minprice,
	t_basicInfo.maxprice,
	t_basicInfo.discountnumber,
	t_basicInfo.discount,
	t_basicInfo.minprice_afterdiscount,
	t_basicInfo.maxprice_afterdiscount,
	t_basicInfo.multiunitname,
	t_basicInfo.oddunitname,
	t_basicInfo.maxpurchaselimit,
	t_basicInfo.buylimittext,
	t_basicInfo.quantityavaliable,
	t_basicInfo.comingSoon,
	t_titles.title,
	t_modifieddescription.description
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Something wrong!"})
		return
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		fmt.Println("Error encoding struct:", err)
		return
	}

	redis.SetKey("getProductData-" + strconv.Itoa(productId.Id), buf.Bytes(), 0)


	c.JSON(http.StatusOK, data)
}