package handler

import (
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func CreatePayment(c *fiber.Ctx) error {

	var input model.CreatePaymentInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể phân tích yêu cầu: %s", err.Error()),
		})
	}

	// Validate input
	validate := validator.New()
	if err := validate.Struct(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	customerInfo, _ := helper.GetInfoCustomerFromToken(c)
	customerId := customerInfo.AccountId

	db := database.DB

	var customer model.Customer
	tx := db.Begin()
	// Kiểm tra khách hàng có tồn tại không
	if err := db.First(&customer, "id = ? AND deleted_at IS NULL AND IsActive IS true", customerId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Khách hàng không tồn tại hoặc đã bị xóa",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Lỗi truy vấn khách hàng: %s", err.Error()),
		})
	}
	// CHECK ORDER
	var order model.Order
	if err := tx.Where("id = ? AND customer_id = ? AND status = ?",
		input.OrderId, customerId, "PENDING").First(&order).Error; err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Đơn hàng không hợp lệ"})
	}

	// if time.Now().After(order.ExpiresAt) {
	// 	database.DB.Model(&order).Update("status", "CANCELLED")
	// 	return c.Status(400).JSON(fiber.Map{"error": "Đơn hàng đã hết hạn"})
	// }
	paymentCode := fmt.Sprintf("PAY_%s_%d", time.Now().Format("20060102"), rand.Intn(1000))

	// VNPAY URL
	vnpay := NewVNPay()
	req := model.PaymentRequest{
		Amount:    int64(order.TotalAmount),
		OrderInfo: fmt.Sprintf("Thanh toán đơn hàng %d - Vé xem phim", order.ID),
		TxnRef:    paymentCode,
		IPAddr:    c.IP(),
	}

	paymentUrl, err := vnpay.BuildPaymentUrl(req)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Lỗi tạo payment URL"})
	}
	// CREATE PAYMENT RECORD
	payment := model.Payment{
		OrderId:     input.OrderId,
		Amount:      order.TotalAmount,
		PaymentCode: paymentCode,
		Status:      "PENDING",
		Method:      input.Method,
	}
	database.DB.Create(&payment)

	return c.JSON(fiber.Map{
		"message":     "Tạo thanh toán thành công",
		"paymentUrl":  paymentUrl,
		"paymentCode": paymentCode,
		"nextStep":    "Hoàn tất thanh toán",
	})

}
func VNPayCallback(c *fiber.Ctx) error {
	vnpay := NewVNPay()
	var payment model.Payment
	queryString := c.OriginalURL() // Hoặc c.Request().URI().QueryString()
	query, _ := url.ParseQuery(queryString)

	result := vnpay.VerifyReturnUrl(query)
	if result.IsSuccess {
		database.DB.Where("payment_code = ?", result.TxnRef).First(&payment)
		database.DB.Model(&payment).Update("status", "PAID")

		var order model.Order
		database.DB.Where("id = ?", payment.OrderId).First(&order)
		database.DB.Model(&order).Update("status", "PAID")

		// Redirect success
		return c.Redirect(fmt.Sprintf("%s/success?orderId=%d", os.Getenv("APP_URL"), payment.OrderId))
	}

	// Failed
	return c.Redirect(fmt.Sprintf("%s/payment-failed?reason=%s", os.Getenv("APP_URL"), result.Message))
}
func VNPayIPN(c *fiber.Ctx) error {
	vnpay := NewVNPay()

	// Parse POST body as query
	body := c.Body()
	query, _ := url.ParseQuery(string(body))
	result := vnpay.VerifyIPN(query)

	if result.IsSuccess {
		// UPDATE DB (idempotent - chỉ update nếu chưa PAID)
		var payment model.Payment
		database.DB.Where("payment_code = ? AND status != ?", result.TxnRef, "PAID").First(&payment)
		if payment.ID > 0 {
			database.DB.Model(&payment).Update("status", "PAID")
			database.DB.Model(&model.Order{}).Where("id = ?", payment.OrderId).Update("status", "PAID")
		}

		// Response cho VNPay
		return c.JSON(fiber.Map{
			"RspCode": "00",
			"Message": "Success",
		})
	}

	return c.JSON(fiber.Map{
		"RspCode": "01",
		"Message": "Failed",
	})
}
