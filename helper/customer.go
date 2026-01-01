package helper

import (
	"cinema_manager/database"
	"cinema_manager/model"
	"errors"

	"gorm.io/gorm"
)

func CheckByPhoneNumberCustomer(phoneNumber string, id *uint) (bool, error) {
	db := database.DB
	var count int64
	if id == nil {
		if err := db.Model(&model.Customer{}).Where(model.Customer{Phone: phoneNumber}).Count(&count).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil
			}
			return false, err
		}
	}
	if id != nil {
		if err := db.Model(&model.Customer{}).Where("phone_number = ? and id != ?", phoneNumber, *id).Count(&count).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil
			}
			return false, err
		}
	}
	return count > 0, nil
}
func CheckByEmailCustomer(email string, id *uint) (bool, error) {
	db := database.DB
	var count int64
	if id == nil {
		if err := db.Model(&model.Customer{}).Where(model.Customer{Email: email}).Count(&count).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil
			}
			return false, err
		}
	}
	if id != nil {
		if err := db.Model(&model.Customer{}).Where("email = ? and id != ?", email, *id).Count(&count).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil
			}
			return false, err
		}
	}
	return count > 0, nil
}
