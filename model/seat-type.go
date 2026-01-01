package model

type SeatType struct {
	DTO
	Type          string  `gorm:"not null" validate:"required" json:"type"` // NORMAL VIP COUPLE
	PriceModifier float64 `json:"priceModifier"`
}
