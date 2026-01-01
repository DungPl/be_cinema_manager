package handler

import (
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

func GetHoliday(c *fiber.Ctx) error {
	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin {
		return utils.ErrorResponse(c, 403, "Chỉ admin được phép", nil)
	}
	filter := new(model.HolidayFilter)
	if err := c.QueryParser(filter); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INPUT, err)
	}
	db := database.DB.Model(&model.Holiday{})
	if filter.Type != nil {
		db = db.Where("type = ?", *filter.Type)
	}
	if filter.Year != nil {
		start := time.Date(*filter.Year, 1, 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(1, 0, -1)
		db = db.Where("date BETWEEN ? AND ?", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}

	var total int64
	db.Count(&total)

	db = utils.ApplyPagination(db, filter.Limit, filter.Page)
	var holidays []model.Holiday
	db.Find(&holidays)
	response := &model.ResponseCustom{
		Rows:       holidays,
		Limit:      filter.Limit,
		Page:       filter.Page,
		TotalCount: total,
	}
	return utils.SuccessResponse(c, fiber.StatusOK, response)
}
func CreateHoliday(c *fiber.Ctx) error {
	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin {
		return utils.ErrorResponse(c, 403, "Chỉ admin được phép", nil)
	}

	input := c.Locals("createInput").(model.Holiday)

	if err := database.DB.Create(&input).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Không thể tạo ngày lễ", err)
	}

	return utils.SuccessResponse(c, 201, fiber.Map{
		"message": "Tạo ngày lễ thành công",
		"data":    input,
	})
}
func UpdateHoliday(c *fiber.Ctx) error {

	holidayId := c.Locals("holidayId").(int)
	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin {
		return utils.ErrorResponse(c, 403, "Chỉ admin được phép", nil)
	}
	var holiday model.Holiday
	if err := database.DB.First(&holiday, holidayId).Error; err != nil {
		return utils.ErrorResponse(c, 404, "Ngày lễ không tồn tại", err)
	}

	input := c.Locals("updateInput").(struct {
		Name        *string
		Date        *time.Time
		Type        *string
		IsRecurring *bool
	})

	if input.Name != nil {
		holiday.Name = *input.Name
	}
	if input.Date != nil {
		holiday.Date = *input.Date
	}
	if input.Type != nil {
		holiday.Type = *input.Type
	}
	if input.IsRecurring != nil {
		holiday.IsRecurring = *input.IsRecurring
	}

	if err := database.DB.Save(&holiday).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Không thể cập nhật", err)
	}

	return utils.SuccessResponse(c, 200, fiber.Map{
		"message": "Cập nhật thành công",
		"data":    holiday,
	})
}
func DeleteHoliday(c *fiber.Ctx) error {
	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin {
		return utils.ErrorResponse(c, 403, "Chỉ admin được phép", nil)
	}

	id, _ := strconv.ParseUint(c.Params("id"), 10, 32)
	var holiday model.Holiday
	if err := database.DB.First(&holiday, id).Error; err != nil {
		return utils.ErrorResponse(c, 404, "Không tìm thấy", err)
	}

	if err := database.DB.Delete(&holiday).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Xóa thất bại", err)
	}

	return utils.SuccessResponse(c, 200, fiber.Map{
		"message": "Xóa thành công",
	})
}
