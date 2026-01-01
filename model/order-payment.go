package model

type Payment struct {
	DTO
	OrderId     uint    `gorm:"not null" json:"orderId"`
	Amount      float64 `gorm:"not null" json:"amount"`
	PaymentCode string  `gorm:"unique" json:"paymentCode"`
	Status      string  `gorm:"default:PENDING" json:"status"`
	Method      string  `json:"method"` // VNPAY, MOMO

	Order Order `gorm:"foreignKey:OrderId" json:"-"`
}
type CreatePaymentInput struct {
	OrderId uint   `json:"orderId" validate:"required,gt=0"`
	Method  string `json:"method" validate:"required,oneof=VNPAY MOMO"`
}
