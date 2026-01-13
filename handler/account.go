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

func ToggleActiveAccount(c *fiber.Ctx) error {
	isActive, ok := c.Locals("isActive").(bool)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("parse isActive fail"))
	}

	accountId, ok := c.Locals("accountId").(uint)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("parse accountId fail"))
	}

	db := database.DB

	var account model.Account
	if err := db.First(&account, accountId).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, constants.NOT_FOUND_RECORDS, err)
	}
	// Update
	account.Active = isActive
	if err := db.Save(&account).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Cập nhật thất bại", err)
	}

	// Reload để trả về dữ liệu mới nhất
	db.First(&account, accountId)

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
func UpdateAccount(c *fiber.Ctx) error {
	db := database.DB
	input, ok := c.Locals("inputUpdateAccount").(model.UpdateAccountInput)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	accountId, ok := c.Locals("accountId").(uint)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy ID tài khoản",
		})
	}

	var account model.Account
	if err := db.First(&account, accountId).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Không tìm thấy chuỗi rạp", err)
	}
	updateMap := map[string]interface{}{}

	if input.Username != nil {
		// Check unique username
		var count int64
		db.Model(&model.Account{}).Where("username = ? AND id != ?", *input.Username, accountId).Count(&count)
		if count > 0 {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Username đã tồn tại", nil)
		}
		updateMap["username"] = *input.Username
	}

	if input.Active != nil {
		updateMap["active"] = *input.Active
	}

	if input.CinemaId != nil {
		updateMap["cinema_id"] = *input.CinemaId
	}

	if input.Role != nil {
		if !utils.IsValidValueOfConstant(*input.Role, constants.ROLE) {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Role không hợp lệ", nil)
		}
		updateMap["role"] = *input.Role
	}

	// Thực hiện update
	if len(updateMap) == 0 {
		return utils.SuccessResponse(c, fiber.StatusOK, "Không có thay đổi nào")
	}

	if err := db.Model(&account).Updates(updateMap).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Cập nhật thất bại", err)
	}

	// Reload để trả về dữ liệu mới
	db.First(&account, accountId)

	return utils.SuccessResponse(c, fiber.StatusOK, account)
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

// Tạo Staff + Account cùng lúc (transaction)
func CreateStaffWithAccount(c *fiber.Ctx) error {
	db := database.DB
	input, ok := c.Locals("inputCreateStaffWithAccount").(model.CreateStaffWithAccountInput)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi lấy dữ liệu", nil)
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. Tạo Account trước
	passwordHash, err := helper.HashPassword(input.Password)
	if err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Không thể mã hóa mật khẩu", err)
	}

	account := model.Account{
		Username: input.Username,
		Password: passwordHash,
		Role:     input.Role,
		CinemaId: input.CinemaId,
		Active:   input.Active, // nếu không gửi thì mặc định true trong struct
	}

	if err := tx.Create(&account).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Không thể tạo tài khoản", err)
	}

	// 2. Tạo Staff và liên kết với Account vừa tạo
	staff := model.Staff{
		FirstName:          input.Firstname,
		LastName:           input.Lastname,
		PhoneNumber:        input.PhoneNumber,
		Email:              input.Email,
		IdentificationCard: input.IdentificationCard,
		Role:               input.Position,
		AccountId:          &account.ID, // liên kết
		IsActive:           true,        // mặc định active
	}

	if err := tx.Create(&staff).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Không thể tạo nhân viên", err)
	}

	// Preload để trả về đầy đủ thông tin
	tx.Preload("Account").First(&staff, staff.ID)

	if err := tx.Commit().Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi commit transaction", err)
	}

	return utils.SuccessResponse(c, fiber.StatusCreated, staff)
}

// Cập nhật thông tin tài khoản liên kết với staff
func UpdateStaffAccount(c *fiber.Ctx) error {
	db := database.DB
	staffId := c.Locals("staffId").(uint)
	input, ok := c.Locals("updateStaffAccountInput").(model.UpdateStaffAccountInput)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi lấy dữ liệu", nil)
	}

	var staff model.Staff
	if err := db.Preload("Account").First(&staff, staffId).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Không tìm thấy nhân viên", err)
	}

	if *staff.AccountId == 0 || staff.Account == nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Nhân viên này chưa có tài khoản", nil)
	}

	updateMap := map[string]interface{}{}

	if input.Role != nil {
		updateMap["role"] = *input.Role
	}
	if input.CinemaId != nil {
		updateMap["cinema_id"] = *input.CinemaId
	}
	if input.Active != nil {
		updateMap["active"] = *input.Active
	}

	if len(updateMap) == 0 {
		return utils.SuccessResponse(c, fiber.StatusOK, "Không có thay đổi nào")
	}

	if err := db.Model(&staff.Account).Updates(updateMap).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Cập nhật tài khoản thất bại", err)
	}

	// Reload để trả về thông tin mới
	db.Preload("Account").First(&staff, staffId)

	return utils.SuccessResponse(c, fiber.StatusOK, staff)
}
