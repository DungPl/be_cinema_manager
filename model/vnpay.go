package model

type VNPayConfig struct {
	TmnCode    string
	HashSecret string
	BaseURL    string
	ReturnURL  string
	IPNURL     string
}

type PaymentRequest struct {
	Amount    int64  `json:"amount"`
	OrderInfo string `json:"orderInfo"`
	TxnRef    string `json:"txnRef"`
	IPAddr    string `json:"ipAddr"`
}

type PaymentResponse struct {
	IsSuccess bool   `json:"isSuccess"`
	TxnRef    string `json:"txnRef"`
	Amount    int64  `json:"amount"`
	Status    string `json:"status"` // 00=Success
	Message   string `json:"message"`
}
