package postgres

import "database/sql"

// var stmt *sql.Stmt

func handleError(err error)  {
	if err != nil {
		panic(err)
	}
}

type Queries struct {
	DB *sql.DB
	GetProductData *sql.Stmt
	EmailAlreadyExist *sql.Stmt
	SignUpUser *sql.Stmt
	CreateDefaultWishlist *sql.Stmt
	Login *sql.Stmt
	GetUserAllWishListsNames *sql.Stmt
	GetUserWishListData *sql.Stmt
}
var queries Queries


func GetProductDataQuery(db *sql.DB) *Queries {
	var err error

	queries.DB = db

	queries.GetProductData, err = db.Prepare(`select 
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
	where t_productId.myproductid = $1`)
	handleError(err)
	
	queries.EmailAlreadyExist, err = db.Prepare("SELECT email FROM shop.t_users WHERE email = $1")
	handleError(err)
	
	queries.SignUpUser, err = db.Prepare("INSERT into shop.t_users(email, password) Values($1, $2) RETURNING id")
	handleError(err)
	
	queries.CreateDefaultWishlist, err = db.Prepare("INSERT into shop.t_wishlist(foreign_user_id, wishlistname) Values($1, $2)")
	handleError(err)
	
	queries.Login, err = db.Prepare("SELECT id, email, password from shop.t_users WHERE email = $1")
	handleError(err)
	
	queries.GetUserAllWishListsNames, err = db.Prepare(`SELECT json_agg(wishlistname) as "wishListNjson_aggames", json_agg(id) as "wishListIds" from shop.t_wishList WHERE foreign_user_id = $1 GROUP BY foreign_user_id`)
	handleError(err)
	
	queries.GetUserWishListData, err = db.Prepare(`
    SELECT     
    t_titles.title,
    t_wishlist_products.id as "wishListId",
    t_wishlist_products.foreign_wishlist_id as "parentWishListId",
    t_wishlist_products.selectedImageUrl as "selectedImageUrl",
    t_wishlist_products.foreign_product_id as "productId",
    t_productId.myProductId as "longProductId",
    t_wishlist.wishlistname as "wishListName",
    minprice as "minPrice",
    maxprice as "maxPrice"
    From shop.t_wishlist_products 
    JOIN shop.t_wishlist ON t_wishlist.id = t_wishlist_products.foreign_wishlist_id
    JOIN shop.t_productId ON t_productId.id = t_wishlist_products.foreign_product_id
    JOIN shop.t_titles ON t_titles.foreign_id = t_wishlist_products.foreign_product_id
    JOIN shop.t_basicinfo ON t_basicinfo.foreign_id = t_wishlist_products.foreign_product_id
    where t_wishlist_products.foreign_wishlist_id = $1 ORDER BY t_wishlist_products.created_at DESC LIMIT $2`)
	handleError(err)

	return &queries
}