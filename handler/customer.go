package handler

import (
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jinzhu/copier"
	"github.com/jordan-wright/email"
	"gorm.io/gorm"
)

func RegisterCustomer(c *fiber.Ctx) error {
	db := database.DB

	// Lấy input từ locals (đã validate ở middleware)
	customerInput, ok := c.Locals("RegisterCustomer").(model.RegisterCustomerInput)
	if !ok {
		return utils.ErrorResponseHaveKey(c, fiber.StatusInternalServerError, "Lỗi parse dữ liệu", nil, "general")
	}
	var existingUser model.Customer
	if err := database.DB.Where("user_name = ?", customerInput.UserName).First(&existingUser).Error; err == nil {
		return utils.ErrorResponseHaveKey(c, fiber.StatusConflict, "Tên đăng nhập đã được sử dụng", nil, "username")
	}
	// 1. Kiểm tra số điện thoại đã tồn tại
	isCheckPhoneNumberCustomer, err := helper.CheckByPhoneNumberCustomer(customerInput.Phone, nil)
	if err != nil {
		return utils.ErrorResponseHaveKey(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err, "phone")
	}
	if isCheckPhoneNumberCustomer {
		return utils.ErrorResponseHaveKey(c, fiber.StatusConflict, "Số điện thoại đã tồn tại", nil, "phone")
	}

	// 2. Kiểm tra email đã tồn tại
	isCheckEmailCustomer, err := helper.CheckByEmailCustomer(customerInput.Email, nil)
	if err != nil {
		return utils.ErrorResponseHaveKey(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err, "email")
	}
	if isCheckEmailCustomer {
		return utils.ErrorResponseHaveKey(c, fiber.StatusConflict, "Email đã tồn tại", nil, "email")
	}

	// 3. Hash password
	hash, err := helper.HashPassword(customerInput.Password)
	if err != nil {
		return utils.ErrorResponseHaveKey(c, fiber.StatusInternalServerError, constants.CAN_NOT_HASH_PASSWORD, err, "password")
	}

	// 4. Tạo customer mới
	newCustomer := new(model.Customer)
	copier.Copy(&newCustomer, &customerInput)
	newCustomer.Password = hash
	newCustomer.IsActive = true

	// 5. Lưu vào DB
	if err := db.Create(&newCustomer).Error; err != nil {
		// Xử lý lỗi unique constraint (email/phone trùng)
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			if strings.Contains(err.Error(), "email") {
				return utils.ErrorResponseHaveKey(c, fiber.StatusConflict, "Email đã tồn tại", nil, "email")
			}
			if strings.Contains(err.Error(), "phone") {
				return utils.ErrorResponseHaveKey(c, fiber.StatusConflict, "Số điện thoại đã tồn tại", nil, "phone")
			}
			// Nếu có unique khác (username)
			if strings.Contains(err.Error(), "username") {
				return utils.ErrorResponseHaveKey(c, fiber.StatusConflict, "Tên đăng nhập đã tồn tại", nil, "username")
			}
		}

		// Lỗi khác (server, DB, ...)
		return utils.ErrorResponseHaveKey(c, fiber.StatusInternalServerError, constants.ERROR_CREATE, err, "general")
	}

	// Thành công
	return utils.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"status":  "success",
		"message": "Đăng ký thành công",
		"data":    newCustomer,
	})
}

func CustomerLogin(c *fiber.Ctx) error {
	type LoginRequest struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required"`
	}
	loginRequest := new(LoginRequest)

	if err := c.BodyParser(loginRequest); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.MISSING_LOGIN_INPUT, err)
	}

	// Manual validation
	if loginRequest.Email == "" || loginRequest.Password == "" {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.MISSING_LOGIN_INPUT, errors.New("email and password are required"))
	}
	email := loginRequest.Email
	password := loginRequest.Password
	customerModel, err := new(model.Customer), *new(error)

	customerModel, err = helper.GetCustomerByEmail(email)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err)
	}
	if customerModel == nil {
		return utils.ErrorResponse(c, fiber.StatusConflict, constants.INVALID_EMAIL, errors.New("customer not exists"))
	}

	if !helper.CheckPasswordHash(password, customerModel.Password) {
		return utils.ErrorResponse(c, fiber.StatusNotFound, constants.INVALID_PASSWORD, errors.New("password does not match email"))
	}

	if !customerModel.IsActive {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.ACCOUNT_NOT_ACTIVE, errors.New("active false"))
	}

	tokenClaim := model.TokenClaim{
		CustomerId: customerModel.ID,
		Username:   customerModel.Email,
	}
	token, err := helper.GenerateAccessToken(tokenClaim)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err)
	}

	refreshToken, err := helper.GenerateRefreshToken(tokenClaim)

	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err)
	}

	tokenData := model.TokenData{
		AccessToken:  token,
		RefreshToken: refreshToken,
	}
	//log.Println("CLAIMS:", tokenClaim)
	return utils.SuccessResponse(c, fiber.StatusOK, tokenData)
}
func EditCustomer(c *fiber.Ctx) error {
	db := database.DB
	customerId := c.Locals("inputCustomerId").(uint)
	customerInput, ok := c.Locals("inputEditCustomer").(model.EditCustomerInput)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	isCheckPhoneNumberCustomer, err := helper.CheckByPhoneNumberCustomer(*customerInput.PhoneNumber, nil)
	if err != nil {
		return utils.ErrorResponseHaveKey(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err, "phoneNumber")
	}
	if isCheckPhoneNumberCustomer {
		return utils.ErrorResponseHaveKey(c, fiber.StatusConflict, constants.PHONE_NUMBER_CUSTOMER_EXISTS, errors.New("phoneNumber exists"), "phoneNumber")
	}
	// isCheckEmailCustomer, err := helper.CheckByEmailCustomer(customerInput., nil)
	// if err != nil {
	// 	return utils.ErrorResponseHaveKey(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err, "email")
	// }
	// if isCheckEmailCustomer {
	// 	return utils.ErrorResponseHaveKey(c, fiber.StatusConflict, constants.PHONE_NUMBER_CUSTOMER_EXISTS, errors.New("email exists"), "email")
	// }
	tx := db.Begin()

	var customer model.Customer
	tx.First(&customer, customerId)
	copier.Copy(&customer, &customerInput)

	if err := tx.Model(&model.Customer{DTO: model.DTO{ID: customerId}}).Updates(customer).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_EDIT, err)
	}
	tx.Commit()
	return utils.SuccessResponse(c, fiber.StatusOK, customer)

}
func ForgotPassword(c *fiber.Ctx) error {
	db := database.DB
	EmailInput, ok := c.Locals("EmailForgotPassword").(model.ForgotPasswordRequest)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}

	var customer model.Customer
	if err := db.Where("email = ?", EmailInput.Email).First(&customer).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Không tìm thấy khách hàng"})
	}
	// Tạo token khôi phục
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Không thể tạo token"})
	}
	token := hex.EncodeToString(tokenBytes)

	// Lưu token vào cơ sở dữ liệu
	resetToken := model.PasswordResetToken{
		CustomerId: customer.ID,
		Token:      token,
		ExpiresAt:  time.Now().Add(1 * time.Hour), // Hết hạn sau 1 giờ
	}
	if err := db.Create(&resetToken).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Không thể lưu token"})
	}

	// Gửi email với liên kết khôi phục
	resetLink := fmt.Sprintf("http://your-app.com/reset-password?token=%s", token)
	e := email.NewEmail()
	e.From = "your-app@example.com"
	e.To = []string{EmailInput.Email}
	e.Subject = "Khôi phục mật khẩu"
	e.Text = []byte(fmt.Sprintf("Nhấp vào liên kết để đặt lại mật khẩu: %s", resetLink))
	err := e.Send("smtp.gmail.com:587", smtp.PlainAuth("", "your-email@gmail.com", "your-app-password", "smtp.gmail.com"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Không thể gửi email"})
	}

	return c.JSON(fiber.Map{"message": "Liên kết khôi phục đã được gửi tới email"})
}
func ResetPassword(c *fiber.Ctx) error {
	db := database.DB
	ResetPassword, ok := c.Locals("ResetPassword").(model.ResetPasswordRequest)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	// Kiểm tra token
	var resetToken model.PasswordResetToken
	if err := db.Where("token = ? AND expires_at > ?", ResetPassword.Token, time.Now()).First(&resetToken).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Token không hợp lệ hoặc đã hết hạn"})
	}

	// Tìm khách hàng
	var customer model.Customer
	if err := db.First(&customer, resetToken.CustomerId).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Không tìm thấy khách hàng"})
	}

	// Băm mật khẩu mới

	hash, err := helper.HashPassword(ResetPassword.NewPassword)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.CAN_NOT_HASH_PASSWORD, err)
	}

	// Cập nhật mật khẩu
	customer.Password = hash
	if err := db.Save(&customer).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Không thể cập nhật mật khẩu"})
	}
	db.Delete(&resetToken)

	return c.JSON(fiber.Map{"message": "Đặt lại mật khẩu thành công"})
}
func ChangePasswordCustomer(c *fiber.Ctx) error {
	db := database.DB
	changePasswordInput, ok := c.Locals("inputChangePasswordCustomer").(model.CustomerChangePassword)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	customerInfo, _ := helper.GetInfoCustomerFromToken(c)
	customerId := customerInfo.AccountId
	var customer model.Customer
	db.First(&customer, customerId)
	if !helper.CheckPasswordHash(changePasswordInput.CurrentPassword, customer.Password) {
		return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, constants.INVALID_PASSWORD, errors.New("currentPassword invalid"), "currentPassword")
	}
	newPasswordHash, err := helper.HashPassword(changePasswordInput.NewPassword)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.CAN_NOT_HASH_PASSWORD, err)
	}
	customer.Password = newPasswordHash
	db.Save(&customer)

	return utils.SuccessResponse(c, fiber.StatusOK, customer)
}
func GetCurrentCustomer(c *fiber.Ctx) error {
	// Ưu tiên dùng customer từ Locals (nếu middleware đã query)
	if customer, ok := c.Locals("customer").(*model.Customer); ok && customer != nil {
		//log.Println("Returning customer from Locals")
		return utils.SuccessResponse(c, fiber.StatusOK, customer)
	}

	// Fallback: query lại từ customerId
	customerId, ok := c.Locals("customerId").(uint)
	if !ok || customerId == 0 {
		//log.Println("No customerId or guest")
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Chưa đăng nhập", nil)
	}

	log.Printf("Querying customer from DB with ID: %d", customerId)
	var customer model.Customer
	if err := database.DB.First(&customer, customerId).Error; err != nil {
		//log.Printf("Customer not found: %v", err)
		return utils.ErrorResponse(c, fiber.StatusNotFound, constants.NOT_FOUND_RECORDS, err)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, customer)
}

// handler/customer.go
func RefreshCustomerToken(c *fiber.Ctx) error {
	// Lấy refresh token từ cookie hoặc body
	refreshToken := c.Cookies("refresh_token")
	if refreshToken == "" {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Không tìm thấy refresh token", nil)
	}

	// Parse refresh token
	token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil || !token.Valid {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Refresh token không hợp lệ hoặc hết hạn", err)
	}

	// Lấy claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Claims không hợp lệ", nil)
	}

	customerIdFloat, ok := claims["customerId"].(float64)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Không tìm thấy customerId", nil)
	}
	customerId := uint(customerIdFloat)

	// Kiểm tra customer tồn tại
	var customer model.Customer
	if err := database.DB.First(&customer, customerId).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Khách hàng không tồn tại", err)
	}

	// Tạo access token mới
	tokenClaim := model.TokenClaim{
		CustomerId: customerId,
		Username:   customer.Email,
	}
	newAccessToken, err := helper.GenerateAccessToken(tokenClaim)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi tạo access token", err)
	}

	// Tạo refresh token mới (optional: rotate refresh token)
	newRefreshToken, err := helper.GenerateRefreshToken(tokenClaim)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi tạo refresh token", err)
	}

	// Set cookie mới (HttpOnly, Secure)
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    newRefreshToken,
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Strict",
		Path:     "/",
		Expires:  time.Now().Add(30 * 24 * time.Hour),
	})

	return utils.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"accessToken":  newAccessToken,
		"refreshToken": newRefreshToken, // Nếu rotate
	})
}
func GetCustomer(c *fiber.Ctx) error {
	db := database.DB

	filterInput := new(model.FilterCustomer)
	if err := c.QueryParser(filterInput); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INPUT, err)
	}
	condition := db.Model(&model.Customer{})
	if filterInput.SearchKey != "" {
		condition = condition.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(filterInput.SearchKey)+"%").
			Or("LOWER(phone_number) LIKE ?", "%"+strings.ToLower(filterInput.SearchKey)+"%")
	}
	if filterInput.Active != nil {
		condition = condition.Where("is_active = ?", filterInput.Active)
	}
	if filterInput.PhoneNumber != "" {
		condition = condition.Where("phone_number like ?", "%"+filterInput.PhoneNumber+"%")
	}

	var totalCount int64
	condition.Count(&totalCount)

	condition = utils.ApplyPagination(condition, filterInput.Limit, filterInput.Page)

	var Customers model.Customers
	condition.Preload("Account.Staff").Preload("Orders", func(db *gorm.DB) *gorm.DB {
		return db.Order("id DESC")
	}).Order("id ASC").Find(&Customers)
	response := &model.ResponseCustom{
		Rows:       Customers,
		Limit:      filterInput.Limit,
		Page:       filterInput.Page,
		TotalCount: totalCount,
	}
	return utils.SuccessResponse(c, fiber.StatusOK, response)

}
func GetCustomerById(c *fiber.Ctx) error {
	db := database.DB

	customerId := c.Locals("inputId").(int)
	var customer model.Customer
	db.First(&customer, customerId)
	return utils.SuccessResponse(c, fiber.StatusOK, customer)
}
