package validate

import (
	"cinema_manager/constants"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

func isValidEmail(email string) bool {
	const emailRegex = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	re := regexp.MustCompile(emailRegex)
	return re.MatchString(email)
}

// Hàm kiểm tra số điện thoại Việt Nam (10 số, bắt đầu bằng 0 hoặc +84)
func isValidPhoneVN(phone string) bool {
	// Loại bỏ khoảng trắng, dấu gạch
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")

	// Kiểm tra +84 (11 số) hoặc 0 (10 số)
	if strings.HasPrefix(phone, "+84") && len(phone) == 12 {
		return true
	}
	if strings.HasPrefix(phone, "0") && len(phone) == 10 {
		return true
	}
	return false
}
func RegisterCustomer() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.RegisterCustomerInput

		// Parse JSON từ request body vào struct
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid input %s", err.Error()),
			})
		}
		if input.Email == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Email không được để trống",
				"field": "email",
			})
		}
		if !isValidEmail(input.Email) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Email không hợp lệ",
				"field": "email",
			})
		}

		// Kiểm tra số điện thoại hợp lệ (10 số, bắt đầu bằng 0 hoặc +84)
		if input.Phone == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Số điện thoại không được để trống",
				"field": "phone",
			})
		}
		if !isValidPhoneVN(input.Phone) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Số điện thoại không hợp lệ (10 số, bắt đầu bằng 0 hoặc +84)",
				"field": "phone",
			})
		}

		// Kiểm tra mật khẩu ít nhất 6 ký tự (validator tag đã có, nhưng kiểm tra thủ công thêm)
		if len(input.Password) < 6 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Mật khẩu phải ít nhất 6 ký tự",
				"field": "password",
			})
		}
		// Validate input
		if err := validate.Struct(input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Save input to context locals
		c.Locals("RegisterCustomer", input)

		// Continue to next handler
		return c.Next()
	}
}

func EditCustomer(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}
		var input model.EditCustomerInput

		// Parse JSON từ request body vào struct
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid input %s", err.Error()),
			})
		}

		// Validate input
		if err := validate.Struct(input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Save input to context locals
		c.Locals("inputEditCustomer", input)
		c.Locals("inputCustomerId", uint(valueKey))

		// Continue to next handler
		return c.Next()
	}
}
func ChangePasswordCustomer() fiber.Handler {
	return func(c *fiber.Ctx) error {

		var input model.CustomerChangePassword
		if input.CurrentPassword == input.NewPassword {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Mật khẩu mới không được trùng mật khẩu hiện tại", errors.New("newPassword invalid"), "newPassword")
		}
		if input.NewPassword != input.RepeatPassword {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Mật khẩu xác nhận không trùng khớp", errors.New("repeatPassword invalid"), "repeatPassword")
		}
		// Parse JSON từ request body vào struct
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid input %s", err.Error()),
			})
		}

		// Validate input
		if err := validate.Struct(input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Save input to context locals
		c.Locals("inputChangePasswordCustomer", input)

		return c.Next()
	}
}
func ForgetPassword() fiber.Handler {
	return func(c *fiber.Ctx) error {

		var input model.ForgotPasswordRequest
		if err := c.BodyParser(input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid input %s", err.Error()),
			})
		}

		// Validate request
		validate := validator.New()
		if err := validate.Struct(input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		c.Locals("EmailForgotPassword", input)
		return c.Next()
	}
}
func RestPassword() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.ResetPasswordRequest
		if err := c.BodyParser(input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid input %s", err.Error()),
			})
		}

		// Validate request
		validate := validator.New()
		if err := validate.Struct(input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		c.Locals("ResetPassword", input)
		return c.Next()
	}
}
