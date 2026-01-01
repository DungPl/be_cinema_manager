package model

import "time"

type Order struct {
	DTO
	PublicCode    string     `gorm:"unique;size:20"`       // Mã đơn hàng công khai (ví dụ: ORD-XXXXXX)
	CustomerID    *uint      `json:"customerId,omitempty"` // Có thể null nếu khách vãng lai (guest)
	Customer      *Customer  `json:"customer,omitempty"`
	ShowtimeID    uint       `json:"showtimeId"`
	Showtime      Showtime   `json:"showtime"`
	TotalAmount   float64    `json:"totalAmount"`   // Tổng tiền
	Status        string     `json:"status"`        // PENDING, PAID, CANCELLED, REFUNDED
	PaymentMethod string     `json:"paymentMethod"` // CARD, CASH, MOMO, VNPAY...
	CreatedAt     time.Time  `json:"createdAt"`
	PaidAt        *time.Time `json:"paidAt,omitempty"`
	Tickets       []Ticket   `gorm:"foreignKey:OrderId"`
	CustomerName  string     `json:"customerName"`
	Phone         string     `json:"phone"`
	Email         string     `json:"email"`
	CreatedBy     uint       `json:"createdBy"` // One-to-Many với Ticket
	CancelledAt   *time.Time `json:"cancelledAt"`
}
