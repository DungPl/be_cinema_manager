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
func CreateStaffWithAccount() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.CreateStaffWithAccountInput
		if err := c.BodyParser(&input); err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Dữ liệu không hợp lệ", err)
		}

		if err := validate.Struct(input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		// Kiểm tra quyền (chỉ admin)
		_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, nil)
		}

		// Kiểm tra username đã tồn tại
		var count int64
		database.DB.Model(&model.Account{}).Where("username = ?", input.Username).Count(&count)
		if count > 0 {
			return utils.ErrorResponseHaveKey(c, fiber.StatusConflict, "Username đã tồn tại", nil, "username")
		}

		// Kiểm tra CCCD đã tồn tại
		exists, err := helper.CheckByIdentificationCardStaff(input.IdentificationCard, nil)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi kiểm tra CCCD", err)
		}
		if exists {
			return utils.ErrorResponseHaveKey(c, fiber.StatusConflict, constants.IDENTIFICATION_CARD_EXISTS, nil, "identificationCard")
		}

		c.Locals("inputCreateStaffWithAccount", input)
		return c.Next()
	}
}

// validate.UpdateStaffAccount
func UpdateStaffAccount(staffIdKey string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		staffId, err := strconv.ParseUint(c.Params(staffIdKey), 10, 32)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "staffId không hợp lệ", err)
		}

		var input model.UpdateStaffAccountInput
		if err := c.BodyParser(&input); err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Dữ liệu không hợp lệ", err)
		}

		if err := validate.Struct(input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Kiểm tra quyền
		_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, nil)
		}

		c.Locals("staffId", uint(staffId))
		c.Locals("updateStaffAccountInput", input)
		return c.Next()
	}
}
func ToggleActiveAccount() fiber.Handler {
	return func(c *fiber.Ctx) error {
		_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
		}

		// Parse body JSON
		type ToggleActiveInput struct {
			Active bool `json:"active" `
		}

		var input ToggleActiveInput
		if err := c.BodyParser(&input); err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.ERROR_INPUT, err)
		}

		// Validate
		if err := validate.Struct(input); err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, err.Error(), nil)
		}

		// Lưu vào Locals
		c.Locals("isActive", input.Active)

		// Parse accountId từ params
		accountIdStr := c.Params("accountId")
		accountId, err := strconv.ParseUint(accountIdStr, 10, 64)
		if err != nil {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("accountId invalid"), "accountId")
		}
		c.Locals("accountId", uint(accountId))

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
func UpdateAccount(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := strconv.Atoi(c.Params(key))
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, err)
		}

		var input model.UpdateAccountInput
		if err := c.BodyParser(&input); err != nil {
			log.Printf("Body parse error: %v", err)
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Dữ liệu đầu vào không hợp lệ", err)
		}

		validate := validator.New()
		if err := validate.Struct(&input); err != nil {
			log.Printf("Validation errors: %v", err)
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Dữ liệu đầu vào không hợp lệ", err)
		}
		// Chỉ Admin được update
		_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Chỉ Admin mới có quyền cập nhật tài khoản", nil)
		}

		var account model.Account
		if err := database.DB.First(&account, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Không tìm thấy chuỗi rạp"})
			}
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi DB", err)
		}

		// Tìm tài khoản cần update

		c.Locals("inputUpdateAccount", input)
		c.Locals("accountId", uint(id))
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
