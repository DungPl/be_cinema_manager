package validate

import (
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"

	"github.com/gofiber/fiber/v2"
)

func CreateTicket() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.CreateTicketInput
		accountInfo, _, _, _, isBanve := helper.GetInfoAccountFromToken(c)
		if !isBanve {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Không có quyền", nil)
		}
		db := database.DB
		var staffAccount model.Account
		if err := db.Preload("Cinema").
			Where("id =? AND active = true", accountInfo.AccountId).
			First(&staffAccount).Error; err != nil {
			return utils.ErrorResponse(c, 404, "ACCOUNT_NOT_FOUND", err)
		}

		if err := c.BodyParser(&input); err != nil {
			return utils.ErrorResponse(c, 400, "Dữ liệu không hợp lệ", err)
		}
		if err := validate.Struct(input); err != nil {
			return utils.ErrorResponse(c, 400, err.Error(), err)
		}
		c.Locals("input", input)
		return c.Next()
	}
}
