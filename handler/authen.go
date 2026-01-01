package handler

import (
	"cinema_manager/constants"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func Login(c *fiber.Ctx) error {
	type LoginInput struct {
		UserName string `json:"username"`
		Password string `json:"password"`
	}

	loginInput := new(LoginInput)

	if err := c.BodyParser(loginInput); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.MISSING_LOGIN_INPUT, err)
	}

	// Manual validation
	if loginInput.UserName == "" || loginInput.Password == "" {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.MISSING_LOGIN_INPUT, errors.New("username and password are required"))
	}

	username := loginInput.UserName
	password := loginInput.Password
	accountModel, err := new(model.Account), *new(error)

	accountModel, err = helper.GetUserByUsername(username)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err)
	}
	if accountModel == nil {
		return utils.ErrorResponse(c, fiber.StatusConflict, constants.INVALID_USERNAME, errors.New("username not exists"))
	}

	if !helper.CheckPasswordHash(password, accountModel.Password) {
		return utils.ErrorResponse(c, fiber.StatusNotFound, constants.INVALID_PASSWORD, errors.New("password does not match username"))
	}

	if !accountModel.Active {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.ACCOUNT_NOT_ACTIVE, errors.New("active false"))
	}

	tokenClaim := model.TokenClaim{
		AccountId: accountModel.ID,
		Username:  accountModel.Username,
	}
	token, err := helper.GenerateAccessToken(tokenClaim)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err)
	}

	refreshToken, err := helper.GenerateRefreshToken(tokenClaim)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err)
	}

	// tokenData := model.TokenData{
	// 	AccessToken:  token,
	// 	RefreshToken: refreshToken,
	// }

	// return utils.SuccessResponse(c, fiber.StatusOK, tokenData)
	// ✅ set access token vào HTTPOnly cookie
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    token,
		HTTPOnly: true,
		SameSite: "None",
		Secure:   false, // nếu deploy HTTPS thì true
		Path:     "/",
	})

	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		HTTPOnly: true,
		SameSite: "None",
		Secure:   false,
		Path:     "/",
	})

	return c.JSON(fiber.Map{
		"message": "login success",
		"account": fiber.Map{
			"id":       accountModel.ID,
			"username": accountModel.Username,
			"role":     accountModel.Role, // THÊM DÒNG NÀY (rất quan trọng!)
			// thêm các field khác nếu cần: name, email, ...
			"cinemaId": accountModel.CinemaId,
		},
	})
}

// func RefreshToken(c *fiber.Ctx) error {
// 	type RefreshTokenRequest struct {
// 		RefreshToken string `json:"refreshToken"`
// 	}

// 	var req RefreshTokenRequest
// 	if err := c.BodyParser(&req); err != nil {
// 		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
// 	}

// 	// Xác thực refresh token
// 	token, err := helper.ParseToken(req.RefreshToken)
// 	if err != nil || !token.Valid {
// 		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid refresh token"})
// 	}

// 	var tokenClaim model.TokenClaim

// 	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
// 		// Xác thực thành công, truy xuất thông tin từ payload
// 		accountId := claims["accountId"].(float64)
// 		username := claims["username"].(string)

// 		tokenClaim = model.TokenClaim{
// 			AccountId: uint(accountId),
// 			Username:  username,
// 		}
// 	} else {
// 		// Xác thực thất bại
// 		fmt.Println("Invalid Token:", err)
// 		return err
// 	}

// 	// Tạo mới access token và refresh token
// 	newAccessToken, err := helper.GenerateAccessToken(tokenClaim)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not generate access token"})
// 	}

// 	newRefreshToken, err := helper.GenerateRefreshToken(tokenClaim)
// 	if err != nil {
// 		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not generate refresh token"})
// 	}

// 	tokenData := model.TokenData{
// 		AccessToken:  newAccessToken,
// 		RefreshToken: newRefreshToken,
// 	}

//		return utils.SuccessResponse(c, fiber.StatusOK, tokenData)
//	}
func RefreshToken(c *fiber.Ctx) error {
	refreshCookie := c.Cookies("refresh_token")
	if refreshCookie == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "refresh token not found"})
	}

	token, err := helper.ParseToken(refreshCookie)
	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid refresh token"})
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token claims"})
	}

	accountIdFloat, ok := claims["accountId"].(float64)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid accountId in payload"})
	}
	username, ok := claims["username"].(string)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid username in payload"})
	}

	tokenClaim := model.TokenClaim{
		AccountId: uint(accountIdFloat),
		Username:  username,
	}

	newAccessToken, err := helper.GenerateAccessToken(tokenClaim)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not generate access token"})
	}

	newRefreshToken, err := helper.GenerateRefreshToken(tokenClaim)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not generate refresh token"})
	}

	// update lại cookie
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    newAccessToken,
		Expires:  time.Now().Add(24 * time.Hour), // 24 giờ
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   false,
		Path:     "/",
	})
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    newRefreshToken,
		Expires:  time.Now().Add(24 * time.Hour), // 24 giờ
		HTTPOnly: true,
		SameSite: "Lax",
		Secure:   false,
		Path:     "/",
	})

	return c.JSON(fiber.Map{
		"message": "refresh success",
	})
}
