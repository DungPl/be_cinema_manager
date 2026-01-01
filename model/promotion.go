package model

import "time"

type Promotion struct {
	DTO
	Code          string    `gorm:"unique;not null" json:"code"`
	Name          string    `gorm:"not null" json:"name"`
	Description   string    `gorm:"type:text" json:"description"`
	DiscountType  string    `gorm:"not null" json:"discountType"` //percentage','fixed','special
	DiscountValue float64   `gorm:"type:decimal(10,2);not null" json:"discountValue"`
	StartDate     time.Time `gorm:"not null" json:"StartDate"`
	EndDate       time.Time `gorm:"not null" json:"EndDate"`
	MaxUsage      int       `gorm:"default:0" json:"maxUsage"`
	Status        string    `gorm:"default:'active';not null" json:"status"` //active','inactive','expired

	CinemaId uint   `json:"cinemaId"`
	Cinema   Cinema `gorm:"foreignKey:CinemaId" json:"cinema"`
}
type Promotions []Promotion
type PromotionCondition struct {
	DTO

	ConditionType  string `gorm:"not null" json:"conditionType"` //'showtime','movie','seat_type','user_type'
	ConditionValue string `gorm:"not null" json:"conditionValue"`

	PromotionId uint      `gorm:"not null;index" json:"promtionId"`
	Promtion    Promotion `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;foreignKey:PromotionId" json:"Promotion"`
}

type PromotionUsage struct {
	DTO
	PromotionId     uint      `gorm:"not null;index" json:"promotionId"`
	TicketId        uint      `gorm:"not null;index" json:"ticketId"`
	CustomerId      uint      `gorm:"index" json:"customerId"`
	AppliedAt       time.Time `gorm:"not null"`
	DiscountApplied float64   `gorm:"type:decimal(10,2);not null"`
}
