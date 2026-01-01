package model

import "time"

type Holiday struct {
	DTO
	Name        string    `gorm:"size:100;not null" json:"name"`
	Date        time.Time `gorm:"type:date;not null" json:"date"`
	Type        string    `gorm:"size:20;not null" json:"type"` // solar / lunar
	IsRecurring bool      `gorm:"default:true" json:"isRecurring"`
}
type CreateHolidayInput struct {
	Name        string `json:"name" validate:"required,min=2,max=100"`
	Date        string `json:"date" validate:"required,datetime=2006-01-02"`
	Type        string `json:"type" validate:"required,oneof=solar lunar"`
	IsRecurring *bool  `json:"isRecurring" validate:"omitempty"`
}

type UpdateHolidayInput struct {
	Name        *string `json:"name" validate:"omitempty,min=2,max=100"`
	Date        *string `json:"date" validate:"omitempty,datetime=2006-01-02"`
	Type        *string `json:"type" validate:"omitempty,oneof=solar lunar"`
	IsRecurring *bool   `json:"isRecurring" validate:"omitempty"`
}
type HolidayFilter struct {
	Pagination
	Type *string `json:"type"`
	Year *int    `json:"year"`
}
