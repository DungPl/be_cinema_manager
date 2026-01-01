package model

type Seat struct {
	DTO
	Row         string   `gorm:"not null" validate:"required" json:"row"`          // e.g., "A", "B"
	Column      int      `gorm:"not null" validate:"required,min=1" json:"column"` // e.g., 1, 2
	Status      string   `gorm:"not null" validate:"required"  json:"status"`
	RoomId      uint     `json:"RoomId"`
	Room        Room     `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"-"`
	IsAvailable bool     `gorm:"default:true" json:"isAvailable"`
	SeatTypeId  uint     `json:"seatTypeId"`
	SeatType    SeatType `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"SeatType"`
	CoupleId    *uint    `json:"coupleId"`
}
