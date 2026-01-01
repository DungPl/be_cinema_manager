package helper

import (
	"cinema_manager/database"
	"cinema_manager/model"
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

func ValidToken(t *jwt.Token, id string) bool {
	n, err := strconv.Atoi(id)
	if err != nil {
		return false
	}

	claims := t.Claims.(jwt.MapClaims)
	uid := int(claims["user_id"].(float64))

	return uid == n
}

func ValidUser(id string, p string) bool {
	db := database.DB
	var account model.Account
	db.First(&account, id)
	if account.Username == "" {
		return false
	}
	if !CheckPasswordHash(p, account.Password) {
		return false
	}
	return true
}

// GetUser get a user
func GetUser(c *fiber.Ctx) error {
	id := c.Params("id")
	db := database.DB
	var account model.Account
	db.Find(&account, id)
	if account.Username == "" {
		return c.Status(404).JSON(fiber.Map{"status": "error", "message": "No user found with ID", "data": nil})
	}
	return c.JSON(fiber.Map{"status": "success", "message": "User found", "data": account})
}

func CheckByIdentificationCardStaff(identificationCard string, id *uint) (bool, error) {
	db := database.DB
	var count int64
	if id == nil {
		if err := db.Model(&model.Staff{}).Where(model.Staff{IdentificationCard: identificationCard}).Count(&count).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil
			}
			return false, err
		}
	}
	if id != nil {
		if err := db.Model(&model.Staff{}).Where("identification_card = ? and id != ?", identificationCard, *id).Count(&count).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil
			}
			return false, err
		}
	}
	return count > 0, nil
}
