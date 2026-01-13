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
	"github.com/jinzhu/copier"
)

func GetStaffs(c *fiber.Ctx) error {
	filterInput := new(model.FilterStaff)
	if err := c.QueryParser(filterInput); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INPUT, err)
	}
	db := database.DB

	condition := db.Model(&model.Staff{})
	if filterInput.SearchKey != "" {
		condition = condition.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(filterInput.SearchKey)+"%").
			Or("LOWER(phone_number) LIKE ?", "%"+strings.ToLower(filterInput.SearchKey)+"%")
	}
	if filterInput.Active != nil {
		condition = condition.Where("is_active = ?", filterInput.Active)
	}
	var totalCount int64
	condition.Count(&totalCount)

	condition = utils.ApplyPagination(condition, filterInput.Limit, filterInput.Page)

	var staffs model.Staffs
	condition.Preload("Account").Preload("Account.Cinema").Order("id ASC").Find(&staffs)
	response := &model.ResponseCustom{
		Rows:       staffs,
		Limit:      filterInput.Limit,
		Page:       filterInput.Page,
		TotalCount: totalCount,
	}
	return utils.SuccessResponse(c, fiber.StatusOK, response)
}

func GetStaffById(c *fiber.Ctx) error {
	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)

	if !isAdmin {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
	}
	staffId := c.Locals("inputId").(int)
	db := database.DB
	var staff model.Staff
	db.Preload("Account").First(&staff, staffId)
	return utils.SuccessResponse(c, fiber.StatusOK, staff)
}

func CreateStaff(c *fiber.Ctx) error {
	db := database.DB
	staffInput, ok := c.Locals("inputCreateStaff").(model.CreateStaffInput)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}

	isCheckIdentificationStaff, err := helper.CheckByIdentificationCardStaff(staffInput.IdentificationCard, nil)
	if err != nil {
		return utils.ErrorResponseHaveKey(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err, "identificationCard")
	}
	if isCheckIdentificationStaff {
		return utils.ErrorResponseHaveKey(c, fiber.StatusConflict, constants.IDENTIFICATION_CARD_EXISTS, errors.New("IdentificationCard exists"), "identificationCard")
	}
	newStaff := new(model.Staff)

	copier.Copy(&newStaff, &staffInput)
	newStaff.IsActive = true

	if err := db.Create(&newStaff).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_CREATE, err)
	}

	db.Preload("Account").First(&newStaff, newStaff.ID)

	return utils.SuccessResponse(c, fiber.StatusOK, newStaff)
}

func EditStaff(c *fiber.Ctx) error {
	db := database.DB
	staffId := c.Locals("inputStaffId").(uint)
	staffInput, ok := c.Locals("inputEditStaff").(model.CreateStaffInput)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	isCheckIdentificationStaff, err := helper.CheckByIdentificationCardStaff(staffInput.IdentificationCard, &staffId)
	if err != nil {
		return utils.ErrorResponseHaveKey(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err, "identificationCard")
	}
	if isCheckIdentificationStaff {
		return utils.ErrorResponseHaveKey(c, fiber.StatusConflict, constants.IDENTIFICATION_CARD_EXISTS, errors.New("IdentificationCard exists"), "identificationCard")
	}

	tx := db.Begin()

	var staff model.Staff
	tx.Preload("Account").First(&staff, staffId)
	copier.Copy(&staff, &staffInput)

	if err := tx.Model(&model.Staff{DTO: model.DTO{ID: staffId}}).Updates(staff).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_EDIT, err)
	}

	tx.Preload("Account").First(&staff, staffId)

	tx.Commit()
	return utils.SuccessResponse(c, fiber.StatusOK, staff)
}

func DeleteStaff(c *fiber.Ctx) error {
	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.NOT_ADMIN, errors.New("permission invalid"))
	}
	db := database.DB
	arrayId := c.Locals("deleteIds").(model.ArrayId)
	ids := arrayId.IDs

	if err := db.Model(&model.Staff{}).Where("id in ?", ids).Update("is_active", false).Error; err != nil {
		fmt.Println("Error:", err)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, ids)
}

func StaffChangePassword(c *fiber.Ctx) error {
	db := database.DB
	changePasswordInput, ok := c.Locals("inputStaffChangePassword").(model.StaffChangePassword)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	dataInfo, _, _, _, _ := helper.GetInfoAccountFromToken(c)
	accountId := dataInfo.AccountId
	var account model.Account
	db.First(&account, accountId)

	if !helper.CheckPasswordHash(changePasswordInput.CurrentPassword, account.Password) {
		return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, constants.INVALID_PASSWORD, errors.New("currentPassword invalid"), "currentPassword")
	}
	newPasswordHash, err := helper.HashPassword(changePasswordInput.NewPassword)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.CAN_NOT_HASH_PASSWORD, err)
	}
	account.Password = newPasswordHash
	db.Save(&account)

	return utils.SuccessResponse(c, fiber.StatusOK, account)
}

func ActiveStaff(c *fiber.Ctx) error {
	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
	}
	isActive, ok := c.Locals("isActive").(bool)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	staffId, ok := c.Locals("staffId").(int)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	db := database.DB
	var staff model.Staff
	if err := db.First(&staff, uint(staffId)).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, constants.NOT_FOUND_RECORDS, err)
	}
	staff.IsActive = isActive
	db.Save(&staff).Scan(&staff)

	return utils.SuccessResponse(c, fiber.StatusOK, staff)
}
