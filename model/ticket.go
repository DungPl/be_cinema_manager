package model

import "time"

type Ticket struct {
	DTO
	BookingTime time.Time `gorm:"not null" validate:"required" json:"bookingTime"`
	Status      string    `gorm:"not null;default:'BOOKED'" json:"status"`
	TicketCode  string    `gorm:"size:20;uniqueIndex" json:"ticketCode"`
	Price       float64   `gorm:"not null" json:"price"`

	IssuedAt       time.Time  `json:"issuedAt"`
	UsedAt         *time.Time `json:"usedAt,omitempty"`
	CancelledAt    *time.Time `json:"cancelledAt"`
	ShowtimeSeatId uint       `json:"showtimeSeatId"`
	ShowtimeId     uint       `json:"showtimeId"`
	SeatId         uint       `json:"seatId"`
	OrderId        uint       `json:"orderId"`
	CustomerId     *uint      `gorm:"default:null" json:"customerId"`
	CreatedBy      uint       `json:"createdBy"`
	// Relationship – không expose vào JSON mặc định
	Showtime     Showtime     `gorm:"foreignKey:ShowtimeId" json:"-"`
	Seat         Seat         `gorm:"foreignKey:SeatId" json:"-"`
	Order        Order        `gorm:"foreignKey:OrderId" json:"-"`
	Customer     Customer     `gorm:"foreignKey:CustomerId;constraint:OnDelete:SET NULL" json:"-"`
	ShowtimeSeat ShowtimeSeat `gorm:"foreignKey:ShowtimeSeatId" json:"-"`
}
type CreateTicketInput struct {
	ShowtimeID   uint   `json:"showtimeId" validate:"required"`
	SeatIds      []uint `json:"seatIds" validate:"required"`
	CustomerName string `json:"customerName" validate:"omitempty"`
	Phone        string `json:"phone" validate:"omitempty"`
	Email        string `json:"email" validate:"omitempty,email"`
}
type FilterTicketInput struct {
	Pagination
	ShowtimeId uint       `json:"showtimeId" validate:"omitempty,gt=0"`
	Status     string     `json:"status" validate:"omitempty,oneof=BOOKED CANCELLED"`
	StartDate  *time.Time `json:"startDate" validate:"omitempty"`
	EndDate    *time.Time `json:"endDate" validate:"omitempty"`
}
