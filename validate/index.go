package validate

import (
	"cinema_manager/constants"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

var validate = validator.New()

func GetById(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}

		// Save input to context locals
		c.Locals("inputId", valueKey)

		// Continue to next handler
		return c.Next()
	}
}

func Delete() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.ArrayId

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

		if len(input.IDs) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Mảng ID cần xoá không được để trống"})
		}

		// Save input to context locals
		c.Locals("deleteIds", input)

		// Continue to next handler
		return c.Next()
	}
}
