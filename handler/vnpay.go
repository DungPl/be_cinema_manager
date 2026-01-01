package handler

import (
	"cinema_manager/model"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"log"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// VNPay Service
type VNPay struct {
	Config model.VNPayConfig
}

func NewVNPay() *VNPay {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ Không tìm thấy file .env, dùng biến môi trường hệ thống...")
	}
	return &VNPay{
		Config: model.VNPayConfig{
			TmnCode:    os.Getenv("VNP_TMNCODE"),
			HashSecret: os.Getenv("VNP_HASHSECRET"),
			BaseURL:    os.Getenv("VNP_URL"),
			ReturnURL:  os.Getenv("APP_URL") + "/vnpay/return",
			IPNURL:     os.Getenv("APP_URL") + "/vnpay/ipn",
		},
	}
}

// Tạo Payment URL
func (v *VNPay) BuildPaymentUrl(req model.PaymentRequest) (string, error) {
	// Params
	params := url.Values{}
	params.Add("vnp_Version", "2.1.0")
	params.Add("vnp_Command", "pay")
	params.Add("vnp_TmnCode", v.Config.TmnCode)
	params.Add("vnp_Amount", strconv.FormatInt(req.Amount*100, 10)) // VND * 100
	params.Add("vnp_CreateDate", time.Now().Format("20060102150405"))
	params.Add("vnp_CurrCode", "VND")
	params.Add("vnp_IpAddr", req.IPAddr)
	params.Add("vnp_Locale", "vn")
	params.Add("vnp_OrderInfo", v.encodeOrderInfo(req.OrderInfo))
	params.Add("vnp_OrderType", "other")
	params.Add("vnp_ReturnUrl", v.Config.ReturnURL)
	params.Add("vnp_TxnRef", req.TxnRef)
	params.Add("vnp_ExpireDate", time.Now().Add(15*time.Minute).Format("20060102150405"))

	// Sort & Hash
	query := params.Encode()
	hash, _ := v.generateHash(query)
	fullQuery := query + "&vnp_SecureHash=" + hash

	// Build URL
	return v.Config.BaseURL + "?" + fullQuery, nil
}

// Verify Return URL (Callback)
func (v *VNPay) VerifyReturnUrl(query url.Values) model.PaymentResponse {
	secureHash := query.Get("vnp_SecureHash")
	query.Del("vnp_SecureHash")

	// Re-hash
	expectedHash, _ := v.generateHash(query.Encode())

	if secureHash != expectedHash {
		return model.PaymentResponse{IsSuccess: false, Message: "Invalid hash"}
	}

	if query.Get("vnp_ResponseCode") == "00" {
		txnRef := query.Get("vnp_TxnRef")
		amount, _ := strconv.ParseInt(query.Get("vnp_Amount"), 10, 64)
		return model.PaymentResponse{
			IsSuccess: true,
			TxnRef:    txnRef,
			Amount:    amount / 100,
			Status:    "PAID",
		}
	}

	return model.PaymentResponse{IsSuccess: false, Message: "Payment failed"}
}

// Verify IPN (Server-to-Server)
func (v *VNPay) VerifyIPN(query url.Values) model.PaymentResponse {
	secureHash := query.Get("vnp_SecureHash")
	query.Del("vnp_SecureHash")

	expectedHash, _ := v.generateHash(query.Encode())

	if secureHash != expectedHash {
		return model.PaymentResponse{IsSuccess: false, Message: "Invalid IPN hash"}
	}

	if query.Get("vnp_ResponseCode") == "00" {
		return model.PaymentResponse{
			IsSuccess: true,
			TxnRef:    query.Get("vnp_TxnRef"),
			Amount:    0, // Không cần amount ở IPN
			Status:    "PAID",
		}
	}

	return model.PaymentResponse{IsSuccess: false, Message: "IPN failed"}
}

// Helpers
func (v *VNPay) generateHash(data string) (string, error) {
	h := hmac.New(sha512.New, []byte(v.Config.HashSecret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (v *VNPay) encodeOrderInfo(info string) string {
	// UTF-8 to TCVN3 (VNPay yêu cầu)
	return url.QueryEscape(info) // Encode để đảm bảo không lỗi ký tự đặc biệt
}
