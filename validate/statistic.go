package validate

import (
	"cinema_manager/utils"

	"github.com/gofiber/fiber/v2"
)

func StatisticTotalOrderByMonth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		month := c.Query("month")

		isDate := utils.IsValidMMYYYY(month)
		if !isDate {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "month invalid",
			})
		}

		// Save input to context locals
		c.Locals("inputMonth", month)

		// Continue to next handler
		return c.Next()
	}
}
