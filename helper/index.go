package helper

import (
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var JwtSecret = []byte(os.Getenv("JWT_SECRET"))

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	return string(bytes), err
}
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	log.Println(hash, "super_password_hash")
	return err == nil
}

func GetUserByUsername(u string) (*model.Account, error) {
	db := database.DB
	var account model.Account
	if err := db.Where(&model.Account{Username: u}).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &account, nil
}
func GetCustomerByEmail(e string) (*model.Customer, error) {
	db := database.DB
	var customer model.Customer
	if err := db.Where(&model.Customer{Email: e}).First(&customer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &customer, nil
}
func Valid(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func GenerateAccessToken(tokenClaim model.TokenClaim) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["username"] = tokenClaim.Username
	claims["customerId"] = tokenClaim.CustomerId
	claims["accountId"] = tokenClaim.AccountId
	claims["exp"] = time.Now().Add(time.Minute * 60).Unix()

	t, err := token.SignedString(JwtSecret)
	return t, err
}

func GenerateRefreshToken(tokenClaim model.TokenClaim) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["username"] = tokenClaim.Username
	claims["accountId"] = tokenClaim.AccountId
	claims["exp"] = time.Now().Add(time.Hour * 24 * 7).Unix()

	t, err := token.SignedString(JwtSecret)
	return t, err
}

func ParseToken(tokenString string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Xác thực thuật toán ký là HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return JwtSecret, nil
	})

	return token, err
}

func GetInfoAccountFromToken(c *fiber.Ctx) (model.TokenClaim, bool, bool, bool, bool) {
	token := c.Locals("user").(*jwt.Token)
	tokenClaim := token.Claims.(jwt.MapClaims)
	accountId := uint(tokenClaim["accountId"].(float64))
	username := tokenClaim["username"].(string)
	var account model.Account
	db := database.DB
	if err := db.Preload("Staff").First(&account, accountId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("Account not found: id=%d", accountId)
			utils.ErrorResponse(c, fiber.StatusUnauthorized, "Tài khoản không tồn tại", err)
		} else {
			log.Printf("Database query error for account: id=%d, error=%v", accountId, err)
			utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err)
		}
		return model.TokenClaim{}, false, false, false, false
	}
	accountInfo := model.TokenClaim{
		AccountId: uint(accountId),
		Username:  username,
		CinemaId:  account.CinemaId,
	}

	db.Preload("Staff").First(&account, accountId)

	return accountInfo,
		account.Role == constants.ROLE_ADMIN,
		account.Role == constants.ROLE_MANAGER,
		account.Role == constants.ROLE_MODERATOR,
		account.Role == constants.ROLE_STAFF
}
func GetInfoCustomerFromToken(c *fiber.Ctx) (model.TokenClaim, model.Customer) {
	var emptyCustomer model.Customer
	var guestClaim = model.TokenClaim{
		CustomerId: 0,
		Username:   "",
	}

	u := c.Locals("user")
	if u == nil {
		log.Println("No user in Locals → guest")
		return guestClaim, emptyCustomer
	}

	userToken, ok := u.(*jwt.Token)
	if !ok || userToken == nil {
		log.Println("Invalid token type → guest")
		return guestClaim, emptyCustomer
	}

	claims, ok := userToken.Claims.(jwt.MapClaims)
	if !ok {
		log.Println("Invalid claims type → guest")
		return guestClaim, emptyCustomer
	}

	log.Printf("Claims: %v", claims)

	// Ưu tiên tìm "customerId", fallback "accountId"
	customerIdFloat := float64(0)
	if cid, ok := claims["customerId"].(float64); ok {
		customerIdFloat = cid
	} else if aid, ok := claims["accountId"].(float64); ok {
		customerIdFloat = aid
	}

	if customerIdFloat == 0 {
		log.Println("No valid customerId/accountId in claims → guest")
		return guestClaim, emptyCustomer
	}

	username, _ := claims["username"].(string)

	tokenClaim := model.TokenClaim{
		CustomerId: uint(customerIdFloat),
		Username:   username,
	}

	log.Printf("customerId from token: %d", tokenClaim.CustomerId)

	if tokenClaim.CustomerId == 0 {
		return guestClaim, emptyCustomer
	}

	var customer model.Customer
	db := database.DB
	if err := db.First(&customer, tokenClaim.CustomerId).Error; err != nil {
		log.Printf("Customer not found (id=%d): %v", tokenClaim.CustomerId, err)
		return guestClaim, emptyCustomer
	}

	c.Locals("customer", &customer)
	log.Printf("Customer found: ID=%d, Email=%s", customer.ID, customer.Email)

	return tokenClaim, customer
}
