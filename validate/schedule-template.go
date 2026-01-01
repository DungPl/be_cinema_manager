package validate

import (
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

func CreateScheduleTemplate() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.CreateScheduleTemplateInput
		_, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin && !isManager {

			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền "))

		}
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Không thể phân tích yêu cầu: %s", err.Error()),
			})
		}

		// Validate input
		validate := validator.New()
		_ = validate.RegisterValidation("timeslot", func(fl validator.FieldLevel) bool {
			v := fl.Field().String()
			regex := regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
			return regex.MatchString(v)
		})
		if err := validate.Struct(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Truyền vào Locals
		c.Locals("createScheduleTemplateInput", input)

		return c.Next()
	}
}
func ValidScheduleTemplateId(c *fiber.Ctx) error {
	idParam := c.Params("scheduleTemplateId")
	if idParam == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ScheduleTemplate ID không được bỏ trống",
		})
	}

	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ScheduleTemplate ID không hợp lệ",
		})
	}

	// check tồn tại trong DB
	db := database.DB
	var st model.ScheduleTemplate
	if err := db.First(&st, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "ScheduleTemplate không tồn tại",
		})
	}

	// set vào locals để handler dùng
	c.Locals("scheduleTemplateId", uint(id))

	return c.Next()
}
func UpdateSchedulerTemplate(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}
		var input model.UpdateScheduleTemplateInput
		_, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin && !isManager {

			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền "))

		}
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Không thể phân tích yêu cầu: %s", err.Error()),
			})
		}

		// Validate input
		validate := validator.New()
		_ = validate.RegisterValidation("timeslot", func(fl validator.FieldLevel) bool {
			v := fl.Field().String()
			regex := regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
			return regex.MatchString(v)
		})
		if err := validate.Struct(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Truyền vào Locals
		c.Locals("updateScheduleTemplateInput", input)
		c.Locals("scheduleTemplateId", uint(valueKey))
		return c.Next()
	}
}
func DeleteScheduleTemplate(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}
		// Kiểm tra lịch chiếu mẫu  tồn tại
		var scheduleTemplate model.ScheduleTemplate
		if err := database.DB.Where("id = ? ", valueKey).First(&scheduleTemplate).Error; err != nil {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Lịch chiếu ma mẫu không tồn tại", err, "valueKey")
		}
		c.Locals("scheduleTemplateId", valueKey)
		return c.Next()
	}
}
func AutoGenerateSchedule() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.AutoGenerateScheduleTemplateInput
		if err := c.BodyParser(&input); err != nil {
			return utils.ErrorResponse(c, 400, "Dữ liệu không hợp lệ", err)
		}
		if err := validate.Struct(input); err != nil {
			return utils.ErrorResponse(c, 400, "Validation failed", err)
		}

		// Parse ngày
		startDate, err := time.Parse("2006-01-02", input.StartDate)
		if err != nil {
			return utils.ErrorResponse(c, 400, "startDate sai định dạng", err)
		}
		endDate, err := time.Parse("2006-01-02", input.EndDate)
		if err != nil {
			return utils.ErrorResponse(c, 400, "endDate sai định dạng", err)
		}
		if endDate.Before(startDate) {
			return utils.ErrorResponse(c, 400, "endDate phải sau startDate", nil)
		}

		// Kiểm tra quyền
		accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin && !isManager {
			return utils.ErrorResponse(c, 403, "Không có quyền", nil)
		}

		// Lấy phim
		var movie model.Movie
		if err := database.DB.First(&movie, input.MovieId).Error; err != nil {
			return utils.ErrorResponse(c, 404, "Phim không tồn tại", err)
		}

		// Kiểm tra phòng
		for _, roomID := range input.RoomIds {
			var room model.Room
			if err := database.DB.Preload("Formats").First(&room, roomID).Error; err != nil {
				return utils.ErrorResponseHaveKey(c, 400, "Phòng không tồn tại", err, "roomIds")
			}
			if isManager && room.CinemaId != *accountInfo.CinemaId {
				return utils.ErrorResponseHaveKey(c, 403, "Không có quyền với phòng này", nil, "roomIds")
			}
			if room.Status != "active" {
				return utils.ErrorResponseHaveKey(c, 400, "Phòng không hoạt động", nil, "roomIds")
			}
		}

		// Lấy template (nếu có)
		var template *model.ScheduleTemplate
		if input.TemplateId != nil {
			var t model.ScheduleTemplate
			if err := database.DB.First(&t, *input.TemplateId).Error; err != nil {
				return utils.ErrorResponse(c, 404, "Template không tồn tại", err)
			}
			template = &t
		} else if input.TemplateName != nil {
			var t model.ScheduleTemplate
			if err := database.DB.Where("name = ?", *input.TemplateName).First(&t).Error; err != nil {
				return utils.ErrorResponse(c, 404, "Template không tồn tại", err)
			}
			template = &t
		}

		// Gợi ý template nếu không chọn
		if template == nil {
			template = helper.SuggestTemplate(&movie, startDate, endDate, input.IsVietnamese)
		}
		isVn := false
		if input.IsVietnamese != nil {
			isVn = *input.IsVietnamese
		}
		// Áp dụng template vào input
		// Tạo input đầy đủ từ template
		fullInput := model.AutoGenerateScheduleInput{
			MovieID:      input.MovieId,
			RoomIDs:      input.RoomIds,
			StartDate:    input.StartDate,
			EndDate:      input.EndDate,
			TimeSlots:    template.TimeSlots,
			Formats:      template.Formats,
			IsVietnamese: isVn,
		}

		// Giới hạn phòng
		if len(fullInput.RoomIDs) > template.MaxRooms {
			fullInput.RoomIDs = fullInput.RoomIDs[:template.MaxRooms]
		}

		// Lọc format hợp lệ với phòng
		validFormats := helper.GetValidFormats(input.RoomIds, input.Formats)
		if len(validFormats) == 0 {
			return utils.ErrorResponse(c, 400, "Không có định dạng nào hợp lệ với các phòng", nil)
		}
		input.Formats = validFormats

		// Truyền dữ liệu
		c.Locals("input", fullInput)
		c.Locals("movie", movie)
		c.Locals("startDate", startDate)
		c.Locals("endDate", endDate)
		c.Locals("accountInfo", accountInfo)
		c.Locals("useTemplate", true)
		c.Locals("template", &template)
		return c.Next()
	}
}
