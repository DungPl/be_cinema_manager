package handler

import (
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func GetScheduleTemplate(c *fiber.Ctx) error {
	filterInput := new(model.FilterScheduleTemplateInput)
	if err := c.QueryParser(filterInput); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INPUT, err)
	}
	_, isAdmin, isQuanLy, _, _ := helper.GetInfoAccountFromToken(c)

	if !isAdmin && !isQuanLy {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền "))
	}
	db := database.DB
	// IMPORTANT: phải bắt đầu từ Model
	cond := db.Model(&model.ScheduleTemplate{})
	if filterInput.DayTypes != "" {
		dayTypes := strings.Split(filterInput.DayTypes, ",")
		for _, v := range dayTypes {
			cond = cond.Where("JSON_CONTAINS(day_types, JSON_QUOTE(?))", v)
		}
	}

	if filterInput.MovieTypes != "" {
		movieTypes := strings.Split(filterInput.MovieTypes, ",")
		for _, v := range movieTypes {
			cond = cond.Where("JSON_CONTAINS(movie_types, JSON_QUOTE(?))", v)
		}
	}

	if filterInput.TimeSlots != "" {
		timeSlots := strings.Split(filterInput.TimeSlots, ",")
		for _, v := range timeSlots {
			cond = cond.Where("JSON_CONTAINS(time_slots, JSON_QUOTE(?))", v)
		}
	}
	var totalCount int64
	cond.Count(&totalCount)

	cond = utils.ApplyPagination(cond, filterInput.Limit, filterInput.Page)

	var schedules []model.ScheduleTemplate
	cond.Find(&schedules)
	response := &model.ResponseCustom{
		Rows:       schedules,
		Limit:      filterInput.Limit,
		Page:       filterInput.Page,
		TotalCount: totalCount,
	}
	return utils.SuccessResponse(c, fiber.StatusOK, response)
}
func GetScheduleTemplateById(c *fiber.Ctx) error {
	scheduleId := c.Locals("scheduleTemplateId").(uint)
	db := database.DB
	var schedule model.ScheduleTemplate
	db.First(&schedule, scheduleId)
	return utils.SuccessResponse(c, fiber.StatusOK, schedule)

}
func CreateScheduleTemplate(c *fiber.Ctx) error {
	accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isManager {

		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền "))

	}

	input := c.Locals("createScheduleTemplateInput").(model.CreateScheduleTemplateInput)

	template := model.ScheduleTemplate{
		Name:        input.Name,
		Description: input.Description,
		DayTypes:    input.DayTypes,
		MovieTypes:  input.MovieTypes,
		TimeSlots:   input.TimeSlots,
		Formats:     input.Formats,
		MaxRooms:    input.MaxRooms,
		MaxPerDay:   input.MaxPerDay,
		Priority:    input.Priority,
		CreatedBy:   accountInfo.AccountId,
	}

	if err := database.DB.Create(&template).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Không thể tạo mẫu", err)
	}

	return utils.SuccessResponse(c, 201, fiber.Map{
		"message": "Tạo mẫu thành công",
		"data":    template,
	})
}
func UpdateSchedulerTemplate(c *fiber.Ctx) error {
	accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isManager {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền "))
	}
	scheduleTemplateId := c.Locals("scheduleTemplateId").(uint)
	input, ok := c.Locals("updateScheduleTemplateInput").(model.UpdateScheduleTemplateInput)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	db := database.DB
	tx := db.Begin()
	var template model.ScheduleTemplate
	if err := tx.First(&template, scheduleTemplateId).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.ErrorResponse(c, 404, "Mẫu không tồn tại", nil)
		}
		return utils.ErrorResponse(c, 500, "Lỗi server", err)
	}
	if input.Name != nil {
		template.Name = *input.Name
	}
	if input.Description != nil {
		template.Description = *input.Description
	}
	if input.DayTypes != nil {
		template.DayTypes = input.DayTypes
	}
	if input.MovieTypes != nil {
		template.MovieTypes = input.MovieTypes
	}
	if input.TimeSlots != nil {
		template.TimeSlots = input.TimeSlots
	}
	if input.Formats != nil {
		template.Formats = input.Formats
	}
	if input.MaxRooms != nil {
		template.MaxRooms = *input.MaxRooms
	}
	if input.MaxPerDay != nil {
		template.MaxPerDay = *input.MaxPerDay
	}
	if input.Priority != nil {
		template.Priority = *input.Priority
	}
	if isAdmin || isManager {
		template.CreatedBy = accountInfo.AccountId
	}
	if err := database.DB.Save(&template).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Không thể cập nhật", err)
	}
	tx.Commit()
	return utils.SuccessResponse(c, 200, fiber.Map{
		"message": "Cập nhật thành công",
		"data":    template,
	})
}
func DeleteScheduleTemplate(c *fiber.Ctx) error {
	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền "))
	}
	scheduleTemplateId, ok := c.Locals("scheduleTemplateId").(uint)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy ID lịch chiếu",
		})
	}

	db := database.DB
	tx := db.Begin()
	if err := tx.Where("id = ?", scheduleTemplateId).Delete(&model.ScheduleTemplate{}).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể xóa lịch chiếu: %s", err.Error()),
		})
	}
	// Commit giao dịch
	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể commit giao dịch: %s", err.Error()),
		})
	}
	return utils.SuccessResponse(c, 200, fiber.Map{
		"message": "Xóa mẫu thành công",
	})
}
