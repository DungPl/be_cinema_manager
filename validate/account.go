package validate

import (
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func AdminChangePassword() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.AdminChangePassword

		// Parse JSON từ request body vào struct
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid input %s", err.Error()),
			})
		}

		_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
		}

		// Validate input
		if err := validate.Struct(input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if input.NewPassword != input.RepeatPassword {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, constants.NEW_PASSWORD_NOT_SAME_REPEAT_PASSWORD, errors.New("newPassword not same repeatPassword"), "repeatedPassword")
		}

		// Save input to context locals
		c.Locals("inputAdminChangePassword", input)

		// Continue to next handler
		return c.Next()
	}
}
func ActiveAccount() fiber.Handler {
	return func(c *fiber.Ctx) error {
		_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
		}
		isActive := c.Query("active")
		accountId := c.Params("accountId")
		valueKeyIsActive, err := strconv.ParseBool(isActive)
		if err != nil {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_BOOL, errors.New("params invalid"), "active")
		}
		valueKeyAccountId, err := strconv.Atoi(accountId)
		if err != nil {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"), "accountId")
		}

		// Save input to context locals
		c.Locals("isActive", valueKeyIsActive)
		c.Locals("accountId", valueKeyAccountId)

		// Continue to next handler
		return c.Next()
	}
}

func CreateAccount() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.CreateAccountInput

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
		_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
		}
		// check type dữ liệu
		if input.Role != "" && !utils.IsValidValueOfConstant(input.Role, constants.ROLE) {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, constants.ROLE_NOT_EXISTS, errors.New("role invalid"), "role")
		}

		// Save input to context locals
		c.Locals("inputCreateAccount", input)

		// Continue to next handler
		return c.Next()
	}
}
func UpdateManagerCinema() fiber.Handler {
	return func(c *fiber.Ctx) error {

		param := c.Params("accountId")
		accountId, err := strconv.Atoi(param)
		if err != nil || accountId <= 0 {
			log.Printf("Invalid accountId: %v", param)
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "ID tài khoản không hợp lệ", errors.New("accountId must be a positive number"))
		}

		var input model.UpdateManagerCinemaInput
		if err := c.BodyParser(&input); err != nil {
			log.Printf("Body parse error: %v", err)
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Dữ liệu đầu vào không hợp lệ", err)
		}

		validate := validator.New()
		if err := validate.Struct(&input); err != nil {
			log.Printf("Validation errors: %v", err)
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Dữ liệu đầu vào không hợp lệ", err)
		}

		// Kiểm tra CinemaId nếu có
		if input.CinemaId != nil {
			db := database.DB
			var cinema model.Cinema
			if err := db.First(&cinema, *input.CinemaId).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return utils.ErrorResponse(c, fiber.StatusBadRequest, "Rạp không tồn tại", fmt.Errorf("cinemaId %d not found", *input.CinemaId))
				}
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi khi truy vấn cơ sở dữ liệu", err)
			}
		}

		// Lưu dữ liệu validated vào Locals để handler sử dụng
		c.Locals("accountId", uint(accountId))
		c.Locals("updateManagerCinemaInput", input)

		return c.Next()
	}
}
