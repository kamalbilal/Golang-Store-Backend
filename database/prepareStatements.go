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
	GetUserAllWishListsNamesIds *sql.Stmt
	GetUserWishListData *sql.Stmt
	GetUserCertainWishListData *sql.Stmt
	GetUserData *sql.Stmt
	GetUserCartData *sql.Stmt
	DeleteProductFromCart *sql.Stmt
	AddProductToWishlist *sql.Stmt
	CheckProductExistInUserCart *sql.Stmt
	UpdateProductInCart *sql.Stmt
	AddProductInCart *sql.Stmt
	IncrementCartCount *sql.Stmt
	CreateNewListInWishList *sql.Stmt
	UpdateWishlistName *sql.Stmt
	
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
	
	queries.CreateDefaultWishlist, err = db.Prepare("INSERT into shop.t_wishlist(foreign_user_id, wishlistname, created_at) Values($1, $2, floor(extract(epoch from now())::integer))")
	handleError(err)
	
	queries.Login, err = db.Prepare("SELECT id, email, password from shop.t_users WHERE email = $1")
	handleError(err)
	
	queries.GetUserAllWishListsNamesIds, err = db.Prepare(`SELECT json_agg(wishlistname) as "wishListNames", json_agg(id) as "wishListIds" from shop.t_wishList WHERE foreign_user_id = $1 GROUP BY foreign_user_id`)
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
	
	queries.GetUserCertainWishListData, err = db.Prepare(`
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
    where t_wishlist_products.foreign_user_id = $1 AND t_wishlist_products.foreign_wishlist_id = $2 ORDER BY t_wishlist_products.created_at DESC LIMIT $3 OFFSET $4`)
	handleError(err)

	queries.GetUserData, err = db.Prepare(`SELECT email from shop.t_users WHERE id = $1`)
	handleError(err)
	
	queries.GetUserCartData, err = db.Prepare(`SELECT 
    title,
    t_cart.id as "cartId",
    t_cart.foreign_product_id as "productId",
    t_productId.myProductId as "longProductId",
    cartname as "cartName",
    selectedImageUrl as "selectedImageUrl",
    price as "selectedPrice",
    t_cart.quantity as "selectedQuantity",
    t_cart.discount as "selectedDiscount",
    selectedproperties as "selectedProperties",
    shippingdetails as "selectedShippingDetails",
    shippingprice as "selectedShippingPrice",
    minprice as "minPrice",
    maxprice as "maxPrice",
    multiunitname as "multiUnitName",
    oddunitname as "oddUnitName",
    maxpurchaselimit as "maxPurchaseLimit",
    buylimittext as "buyLimitText",
    quantityavaliable as "quantityAvaliable",
    byname as "priceList_InNames",
    bynumber as "priceList_InNumbers",
    bydata as "priceList_Data"
    FROM shop.t_cart 
    JOIN shop.t_productId ON t_productId.id = t_cart.foreign_product_id
    JOIN shop.t_titles ON t_titles.foreign_id = t_cart.foreign_product_id
    JOIN shop.t_basicinfo ON t_basicinfo.foreign_id = t_cart.foreign_product_id
    JOIN shop.t_pricelist ON t_pricelist.foreign_id = t_cart.foreign_product_id
    WHERE foreign_user_id = $1`)
	handleError(err)

	queries.DeleteProductFromCart, err = db.Prepare(`DELETE from shop.t_cart WHERE foreign_product_id = $1 and foreign_user_id = $2 and id = $3 RETURNING id`)
	handleError(err)
	
	queries.AddProductToWishlist, err = db.Prepare(`INSERT into shop.t_wishlist_products(foreign_user_id, foreign_product_id, foreign_wishlist_id, selectedImageUrl) Values($1, $2, $3, $4) ON CONFLICT (foreign_user_id, foreign_product_id) DO UPDATE SET foreign_wishlist_id = $5, selectedImageUrl = $6, created_at = floor(extract(epoch from NOW())::integer) RETURNING id`)
	handleError(err)
	
	queries.CheckProductExistInUserCart, err = db.Prepare(`SELECT id from shop.t_cart WHERE cartName = $1 and foreign_user_id = $2`)
	handleError(err)
	
	queries.UpdateProductInCart, err = db.Prepare(`UPDATE shop.t_cart SET quantity = $1, price = $2, shippingPrice = $3, discount = $4, selectedProperties = $5, shippingDetails = $6, selectedImageUrl = $7 WHERE foreign_user_id = $8 and foreign_product_id = $9 and cartName = $10 RETURNING id`)
	handleError(err)
	
	queries.AddProductInCart, err = db.Prepare(`INSERT into shop.t_cart(foreign_product_id, foreign_user_id, cartName, quantity, price, shippingPrice, discount, selectedProperties, shippingDetails, selectedImageUrl) Values($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`)
	handleError(err)
	
	queries.IncrementCartCount, err = db.Prepare(`UPDATE shop.t_users SET cartCount = cartCount + 1 WHERE id = $1`)
	handleError(err)
	
	queries.CreateNewListInWishList, err = db.Prepare(`INSERT into shop.t_wishlist(foreign_user_id, wishlistname, created_at) Values($1, $2, floor(extract(epoch from now())::integer)) RETURNING id`)
	handleError(err)
	
	queries.UpdateWishlistName, err = db.Prepare(`UPDATE shop.t_wishlist SET wishlistname = $1 WHERE foreign_user_id = $2 and id = $3 and wishlistname = $4`)
	handleError(err)
	
	return &queries
}