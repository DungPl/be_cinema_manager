package handler

import (
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

func Me(c *fiber.Ctx) error {
	dataInfo, _, _, _, _ := helper.GetInfoAccountFromToken(c)
	accountId := dataInfo.AccountId

	db := database.DB
	var account model.Account
	if err := db.Preload("Staff").First(&account, accountId).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, constants.NOT_FOUND_RECORDS, err)
	}
	return utils.SuccessResponse(c, fiber.StatusOK, account)
}

func AdminChangePassword(c *fiber.Ctx) error {
	changePasswordInput, ok := c.Locals("inputAdminChangePassword").(model.AdminChangePassword)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	db := database.DB
	accountId := changePasswordInput.AccountId
	var account model.Account
	if err := db.First(&account, accountId).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, constants.NOT_FOUND_RECORDS, err)
	}

	newPasswordHash, err := helper.HashPassword(changePasswordInput.NewPassword)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.CAN_NOT_HASH_PASSWORD, err)
	}
	account.Password = newPasswordHash
	db.Save(&account)

	return utils.SuccessResponse(c, fiber.StatusOK, account)
}

func ActiveAccount(c *fiber.Ctx) error {
	isActive, ok := c.Locals("isActive").(bool)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	accountId, ok := c.Locals("accountId").(int)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	db := database.DB
	var account model.Account
	if err := db.First(&account, uint(accountId)).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, constants.NOT_FOUND_RECORDS, err)
	}
	account.Active = isActive
	db.Save(&account).Scan(&account)

	return utils.SuccessResponse(c, fiber.StatusOK, account)
}

func GetAccounts(c *fiber.Ctx) error {
	filterInput := new(model.FilterAccount)
	if err := c.QueryParser(filterInput); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INPUT, err)
	}
	db := database.DB

	condition := db.Model(&model.Account{})
	if filterInput.SearchKey != "" {
		condition = condition.Where("LOWER(username) LIKE ?", "%"+strings.ToLower(filterInput.SearchKey)+"%")
	}
	if filterInput.Active != nil {
		condition = condition.Where("is_active = ?", filterInput.Active)
	}
	if filterInput.Role != nil {
		condition = condition.Where("role = ?", filterInput.Role)
	}
	if filterInput.IsUsed != nil {
		// lấy danh sách tài khoản đã được sử dụng
		usedAccounts := db.Model(&model.Staff{}).Select("account_id").Where("account_id IS NOT NULL ")
		if !*filterInput.IsUsed {
			// nếu IsUsed là false, lấy danh sách tài khoản chưa được sử dụng
			condition = condition.Where("id NOT IN (?)", usedAccounts)
		} else {
			// nếu IsUsed là true, lấy danh sách tài khoản đã được sử dụng
			condition = condition.Where("id IN (?)", usedAccounts)
		}
	}
	// if filterInput.IsUsed != nil && !*filterInput.IsUsed {
	// 	condition = condition.Where("is_active = ?", filterInput.IsUsed)
	// }
	var totalCount int64
	condition.Count(&totalCount)

	condition = utils.ApplyPagination(condition, filterInput.Limit, filterInput.Page)

	var accounts model.Accounts
	condition.Preload("Staff").Order("id ASC").Find(&accounts)
	response := &model.ResponseCustom{
		Rows:       accounts,
		Limit:      filterInput.Limit,
		Page:       filterInput.Page,
		TotalCount: totalCount,
	}
	return utils.SuccessResponse(c, fiber.StatusOK, response)
}

func CreateAccount(c *fiber.Ctx) error {
	db := database.DB
	accountInput, ok := c.Locals("inputCreateAccount").(model.CreateAccountInput)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}

	if accountInput.Password == "" {
		accountInput.Password = "123456"
	}

	hash, err := helper.HashPassword(accountInput.Password)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.CAN_NOT_HASH_PASSWORD, err)
	}

	newAccount := new(model.Account)
	copier.Copy(&newAccount, &accountInput)
	newAccount.Password = hash
	newAccount.Active = true
	if accountInput.CinemaId != nil {
		newAccount.CinemaId = accountInput.CinemaId
	}
	if err := db.Create(&newAccount).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_CREATE, err)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, newAccount)
}

// UpdateManagerCinema handler
func UpdateManagerCinema(c *fiber.Ctx) error {
	accountId, ok := c.Locals("accountId").(uint)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Không thể lấy ID tài khoản", errors.New("accountId missing in context"))
	}

	input, ok := c.Locals("updateManagerCinemaInput").(model.UpdateManagerCinemaInput)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Không thể lấy dữ liệu đầu vào", errors.New("input missing in context"))
	}

	// Kiểm tra quyền
	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.ACCOUNT_NOT_PERMISSION, errors.New("không có quyền cập nhật"))
	}

	db := database.DB
	tx := db.Begin()

	var account model.Account
	if err := tx.First(&account, accountId).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Tài khoản không tồn tại", fmt.Errorf("accountId %d not found", accountId))
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err)
	}

	// Kiểm tra role
	if account.Role != constants.ROLE_MANAGER {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Tài khoản không phải quản lý", errors.New("account is not a manager"))
	}

	oldCinemaId := account.CinemaId
	account.CinemaId = input.CinemaId

	if err := tx.Save(&account).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_UPDATE, err)
	}

	if err := tx.Commit().Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INTERNAL_ERROR, err)
	}

	log.Printf("Updated manager cinema: accountId=%d, username=%s, oldCinemaId=%v, newCinemaId=%v",
		accountId, account.Username, oldCinemaId, account.CinemaId)

	response := struct {
		AccountId uint   `json:"accountId"`
		Username  string `json:"username"`
		Role      string `json:"role"`
		CinemaId  *uint  `json:"cinemaId"`
	}{
		AccountId: account.ID,
		Username:  account.Username,
		Role:      account.Role,
		CinemaId:  account.CinemaId,
	}

	return utils.SuccessResponse(c, fiber.StatusOK, response)
}
