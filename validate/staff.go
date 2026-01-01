package validate

import (
	"cinema_manager/constants"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func CreateStaff() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.CreateStaffInput

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
		c.Locals("inputCreateStaff", input)

		// Continue to next handler
		return c.Next()
	}
}

func EditStaff(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}
		var input model.CreateStaffInput

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
		c.Locals("inputEditStaff", input)
		c.Locals("inputStaffId", uint(valueKey))

		// Continue to next handler
		return c.Next()
	}
}

func StaffChangePassword() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.StaffChangePassword

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

		if input.CurrentPassword == input.NewPassword {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Mật khẩu mới không được trùng mật khẩu hiện tại", errors.New("newPassword invalid"), "newPassword")
		}
		if input.NewPassword != input.RepeatPassword {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Mật khẩu xác nhận không trùng khớp", errors.New("repeatPassword invalid"), "repeatPassword")
		}

		// Save input to context locals
		c.Locals("inputStaffChangePassword", input)

		// Continue to next handler
		return c.Next()
	}
}
func ActiveStaff() fiber.Handler {
	return func(c *fiber.Ctx) error {
		_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
		}
		isActive := c.Params("isActive")
		staffId := c.Params("staffId")
		valueKeyIsActive, err := strconv.ParseBool(isActive)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_BOOL, errors.New("params invalid"))
		}
		valueKeyStaffId, err := strconv.Atoi(staffId)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}

		// Save input to context locals
		c.Locals("isActive", valueKeyIsActive)
		c.Locals("staffId", valueKeyStaffId)

		// Continue to next handler
		return c.Next()
	}
}
