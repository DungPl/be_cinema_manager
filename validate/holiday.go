package validate

import (
	"cinema_manager/constants"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

func CreateHoliday() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.CreateHolidayInput
		if err := c.BodyParser(&input); err != nil {
			return utils.ErrorResponse(c, 400, "Dữ liệu không hợp lệ", err)
		}
		if err := validate.Struct(input); err != nil {
			return utils.ErrorResponse(c, 400, "Validation failed", err)
		}

		// Parse date
		date, err := time.Parse("2006-01-02", input.Date)
		if err != nil {
			return utils.ErrorResponse(c, 400, "date sai định dạng", err)
		}

		// Default recurring
		isRecurring := true
		if input.IsRecurring != nil {
			isRecurring = *input.IsRecurring
		}
		c.Locals("createInput", model.Holiday{
			Name:        input.Name,
			Date:        date,
			Type:        input.Type,
			IsRecurring: isRecurring,
		})

		return c.Next()
	}
}
func UpdateHoliday(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}
		var input model.UpdateHolidayInput
		if err := c.BodyParser(&input); err != nil {
			return utils.ErrorResponse(c, 400, "Dữ liệu không hợp lệ", err)
		}
		if err := validate.Struct(input); err != nil {
			return utils.ErrorResponse(c, 400, "Validation failed", err)
		}

		// Parse date if provided
		var date *time.Time
		if input.Date != nil {
			d, err := time.Parse("2006-01-02", *input.Date)
			if err != nil {
				return utils.ErrorResponse(c, 400, "date sai định dạng", err)
			}
			date = &d
		}

		c.Locals("updateInput", struct {
			Name        *string
			Date        *time.Time
			Type        *string
			IsRecurring *bool
		}{
			Name:        input.Name,
			Date:        date,
			Type:        input.Type,
			IsRecurring: input.IsRecurring,
		})
		c.Locals("holidayId", valueKey)
		return c.Next()
	}
}
