package middleware

import (
	"cinema_manager/helper"
	"cinema_manager/utils"
	"errors"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// func Protected() fiber.Handler {
// 	return jwtware.New(jwtware.Config{
// 		SigningKey:   jwtware.SigningKey{Key: []byte(config.Config("SECRET"))},
// 		ErrorHandler: jwtError,
// 	})

// }
func Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Cookies("access_token")

		if token == "" {
			// check header Authorization: Bearer xxx
			auth := c.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				token = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if token == "" {
			return utils.ErrorResponse(c, 401, "Missing token", errors.New("no token"))
		}

		jwtToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if err != nil || !jwtToken.Valid {
			return utils.ErrorResponse(c, 401, "Invalid token", err)
		}

		c.Locals("user", jwtToken)
		return c.Next()
	}
}
func OptionalJWT() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		//log.Printf("Authorization header: %s", authHeader)

		if authHeader == "" {
			//log.Println("No Authorization header")
			c.Locals("user", nil)
			return c.Next()
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		//log.Printf("Token string: %s", tokenString)

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil || !token.Valid {
			//log.Printf("Token invalid: %v", err)
			c.Locals("user", nil)
			return c.Next()
		}

		//log.Println("Token parsed successfully")
		c.Locals("user", token)
		return c.Next()
	}
}

func OptionalAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		claim, customer := helper.GetInfoCustomerFromToken(c)

		if claim.CustomerId == 0 {
			//log.Println("Guest user (customerId = 0)")
			c.Locals("customerId", uint(0))
			return c.Next()
		}

		//log.Printf("User authenticated - customerId: %d", claim.CustomerId)
		c.Locals("customerId", claim.CustomerId)

		// Nếu helper đã query được customer → gán vào Locals
		if customer.ID > 0 {
			c.Locals("customer", &customer)
			//log.Printf("Customer found in DB: %v", customer.Email)
		}

		return c.Next()
	}
}

// func jwtError(c *fiber.Ctx, err error) error {
// 	if err.Error() == "missing or malformed JWT" {
// 		return c.Status(fiber.StatusBadRequest).
// 			JSON(fiber.Map{"status": "error", "message": "Missing or malformed JWT", "data": nil})
// 	}
// 	return c.Status(fiber.StatusUnauthorized).
// 		JSON(fiber.Map{"status": "error", "message": "Invalid or expired JWT", "data": nil})
// }
