package handler

import (
	"bytes"
	"cinema_manager/database"
	"cinema_manager/model"
	"cinema_manager/utils"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gopkg.in/gomail.v2"
	"gorm.io/gorm"
)

func GetMyOrders(c *fiber.Ctx) error {
	customer, ok := c.Locals("customer").(*model.Customer)
	if !ok || customer == nil {
		return utils.ErrorResponse(c, fiber.StatusUnauthorized, "Vui lòng đăng nhập", nil)
	}

	var orders []model.Order
	if err := database.DB.
		Preload("Tickets").
		Preload("Tickets.ShowtimeSeat").
		Preload("Tickets.ShowtimeSeat.Seat").
		Preload("Showtime").
		Preload("Showtime.Movie").
		Preload("Showtime.Movie.Posters").
		Where("customer_id = ? AND status = ?", customer.ID, "PAID").
		Order("created_at desc").
		Find(&orders).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi tải đơn hàng", err)
	}

	response := []map[string]interface{}{}

	for _, order := range orders {
		seats := []string{}
		tickets := []map[string]interface{}{}

		for _, ticket := range order.Tickets {
			seatLabel := ""
			seat := ticket.ShowtimeSeat.Seat
			if seat.ID != 0 {
				seatLabel = fmt.Sprintf("%s%d", seat.Row, seat.Column)
				seats = append(seats, seatLabel)
			}

			tickets = append(tickets, map[string]interface{}{
				"ticketCode": ticket.TicketCode,
				"seatLabel":  seatLabel,
			})
		}
		qrContent := order.PublicCode
		qrBytes, err := utils.GenerateQRCode(qrContent, 400) // size lớn hơn cho dễ quét
		qrBase64 := ""
		if err != nil {
			log.Printf("Lỗi tạo QR cho đơn hàng %s: %v", order.PublicCode, err)
		} else {
			qrBase64 = "data:image/png;base64," + base64.StdEncoding.EncodeToString(qrBytes)
		}
		// Poster
		posterUrl := ""
		if order.Showtime.Movie.Posters != nil {
			for _, poster := range *order.Showtime.Movie.Posters {
				if poster.Url != nil {
					posterUrl = *poster.Url
					break
				}
			}
		}

		response = append(response, map[string]interface{}{
			"orderCode":   order.PublicCode,
			"movieTitle":  order.Showtime.Movie.Title,
			"showtime":    order.Showtime.StartTime.Format("02/01/2006 15:04"),
			"seats":       seats,
			"totalAmount": order.TotalAmount,
			"At":          order.PaidAt.Format("02/01/2006 15:04"),
			"poster":      posterUrl,
			"ticketCount": len(order.Tickets),
			"tickets":     tickets,
			"qrCode":      qrBase64,
		})
	}

	return utils.SuccessResponse(c, fiber.StatusOK, response)
}
func GetOrderDetail(c *fiber.Ctx) error {
	orderCode := c.Params("orderCode")

	var order model.Order
	if err := database.DB.
		Preload("Tickets").
		Preload("Tickets.ShowtimeSeat").
		Preload("Tickets.ShowtimeSeat.Seat").
		Preload("Showtime").
		Preload("Showtime.Movie").
		Where("public_code = ?", orderCode).
		First(&order).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Không tìm thấy đơn hàng", err)
	}

	// Tạo danh sách ghế (label)
	seats := []string{}
	for _, ticket := range order.Tickets {
		seat := ticket.ShowtimeSeat.Seat
		if seat.ID != 0 {
			seats = append(seats, fmt.Sprintf("%s%d", seat.Row, seat.Column))
		}
	}

	// === TẠO 1 QR DUY NHẤT CHO CẢ ĐƠN HÀNG ===
	qrContent := order.PublicCode                        // hoặc link: https://yourdomain.com/staff/checkin/order?code=ORD-ABC123
	qrBytes, err := utils.GenerateQRCode(qrContent, 400) // size lớn hơn cho dễ quét
	qrBase64 := ""
	if err != nil {
		log.Printf("Lỗi tạo QR cho đơn hàng %s: %v", order.PublicCode, err)
	} else {
		qrBase64 = "data:image/png;base64," + base64.StdEncoding.EncodeToString(qrBytes)
	}
	languageLabel := utils.GetLanguageLabel(utils.LanguageType(order.Showtime.LanguageType))
	// Response – chỉ 1 qrCode cho cả đơn
	response := map[string]interface{}{
		"orderCode":     order.PublicCode,
		"movieTitle":    order.Showtime.Movie.Title,
		"showtime":      order.Showtime.StartTime.Format("15:04 - 02/01/2006"),
		"format":        order.Showtime.Format, // nếu có field format trong Showtime
		"language":      languageLabel,
		"seats":         seats,
		"totalAmount":   order.TotalAmount,
		"paymentMethod": order.PaymentMethod,
		"paidAt":        order.PaidAt.Format("15:04 - 02/01/2006"),
		"customerName":  order.CustomerName,
		"phone":         order.Phone,
		"email":         order.Email,
		"qrCode":        qrBase64, // ← 1 QR DUY NHẤT
		"status":        order.Status,
	}

	return utils.SuccessResponse(c, fiber.StatusOK, response)
}
func GetOrderSuccessDetail(c *fiber.Ctx) error {
	orderCode := c.Params("orderCode")

	var order model.Order
	if err := database.DB.
		Preload("Tickets").
		Preload("Tickets.ShowtimeSeat").
		Preload("Tickets.ShowtimeSeat.Seat").
		Preload("Showtime").
		Preload("Showtime.Movie").
		Where("public_code = ?", orderCode).
		First(&order).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Không tìm thấy đơn hàng", err)
	}

	// Tạo danh sách ghế
	seats := []string{}
	for _, ticket := range order.Tickets {
		seat := ticket.ShowtimeSeat.Seat
		if seat.ID != 0 {
			seats = append(seats, fmt.Sprintf("%s%d", seat.Row, seat.Column))
		}
	}

	// === TẠO 1 QR DUY NHẤT CHO CẢ ĐƠN HÀNG ===
	qrContent := order.PublicCode // hoặc link check-in
	qrBytes, err := utils.GenerateQRCode(qrContent, 400)
	qrBase64 := ""
	if err == nil {
		qrBase64 = "data:image/png;base64," + base64.StdEncoding.EncodeToString(qrBytes)
	} else {
		log.Printf("Lỗi tạo QR đơn hàng: %v", err)
	}

	// Response – chỉ 1 qrCode
	response := map[string]interface{}{
		"orderCode":   order.PublicCode,
		"movieTitle":  order.Showtime.Movie.Title,
		"showtime":    order.Showtime.StartTime.Format("15:04 - 02/01/2006"),
		"seats":       seats,
		"totalAmount": order.TotalAmount,
		"qrCode":      qrBase64, // ← 1 QR duy nhất
	}

	return utils.SuccessResponse(c, fiber.StatusOK, response)
}
func SendCancelConfirmationEmail(order model.Order, refundAmount float64) {
	// Lấy danh sách ghế
	seatLabels := make([]string, 0, len(order.Tickets))
	for _, ticket := range order.Tickets {
		seat := ticket.ShowtimeSeat.Seat
		if seat.ID != 0 {
			seatLabels = append(seatLabels, fmt.Sprintf("%s%d", seat.Row, seat.Column))
		}
	}
	// Data cho template
	data := utils.OrderConfirmationData{
		OrderCode:     order.PublicCode,
		MovieName:     order.Showtime.Movie.Title,
		Showtime:      order.Showtime.StartTime.Format("15:04 - 02/01/2006"),
		Seats:         strings.Join(seatLabels, ", "),
		TotalAmount:   order.TotalAmount,
		PaymentMethod: order.PaymentMethod,
		// Thêm thông tin hủy
		RefundAmount: refundAmount,
		CancelledAt:  time.Now().Format("15:04 - 02/01/2006"),
	}

	// Render template hủy vé (tạo file templates/order_cancelled.html)
	tmplPath := "templates/order_cancelled.html"
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Printf("Lỗi load template hủy vé: %v", err)
		return
	}

	var htmlBody bytes.Buffer
	if err := tmpl.Execute(&htmlBody, data); err != nil {
		log.Printf("Lỗi render template hủy vé: %v", err)
		return
	}

	// Tạo email
	m := gomail.NewMessage()
	m.SetHeader("From", "CinemaPro <cinema_hub@gmail.com>")
	m.SetHeader("To", order.Email)
	m.SetHeader("Subject", fmt.Sprintf("Hủy vé thành công - Mã đơn: %s", order.PublicCode))
	m.SetBody("text/html", htmlBody.String())

	// Nhúng QR code (giống email đặt vé)
	qrContent := order.PublicCode
	qrBytes, err := utils.GenerateQRCode(qrContent, 400)
	if err == nil {
		m.Embed("qr_cancel.png", gomail.SetCopyFunc(func(w io.Writer) error {
			_, err := w.Write(qrBytes)
			return err
		}), gomail.SetHeader(map[string][]string{
			"Content-Type":        {"image/png"},
			"Content-ID":          {"<qr_cancel_code>"},
			"Content-Disposition": {"inline"},
		}))
	}

	// Gửi email async
	d := gomail.NewDialer(os.Getenv("SMTP_HOST"), 587, os.Getenv("SMTP_USERNAME"), os.Getenv("SMTP_PASSWORD"))
	if err := d.DialAndSend(m); err != nil {
		log.Printf("Lỗi gửi email hủy vé cho %s: %v", order.Email, err)
	} else {
		log.Printf("Đã gửi email xác nhận hủy vé đến %s (hoàn %fđ)", order.Email, refundAmount)
	}
}
func CancelOrder(c *fiber.Ctx) error {
	orderCode := c.Params("orderCode")
	db := database.DB
	var order model.Order
	if err := db.Preload("Tickets").
		Preload("Tickets.ShowtimeSeat").
		Preload("Tickets.ShowtimeSeat.Seat").
		Preload("Showtime").
		Preload("Showtime.Movie").
		Where("public_code = ?", orderCode).First(&order).Error; err != nil {
		return utils.ErrorResponse(c, 404, "Không tìm thấy đơn hàng", err)
	}

	// Không cho hủy nếu đã check-in
	for _, ticket := range order.Tickets {
		if ticket.Status == "CHECKED_IN" {
			return utils.ErrorResponse(c, 400, "Có vé đã check-in, không thể hủy đơn hàng", nil)
		}
	}

	// Không cho hủy nếu đã hủy rồi
	if order.Status == "CANCELLED" {
		return utils.ErrorResponse(c, 400, "Đơn hàng đã được hủy trước đó", nil)
	}

	// Tính thời gian còn lại đến suất chiếu
	showtimeStart := order.Showtime.StartTime
	hoursBefore := time.Until(showtimeStart).Hours()

	var refundPercent float64
	if hoursBefore >= 2 {
		refundPercent = 1.0 // 100%
	} else if hoursBefore >= 1.0 { // 30 phút
		refundPercent = 0.5 // 50%
	} else {
		return utils.ErrorResponse(c, 400, "Quá muộn để hủy vé. Không thể hoàn tiền.", nil)
	}

	refundAmount := float64(order.TotalAmount) * refundPercent
	actualRevenue := order.TotalAmount - refundAmount
	// Transaction: cập nhật trạng thái + hoàn tiền
	err := db.Transaction(func(tx *gorm.DB) error {
		// Cập nhật order
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status":         "CANCELLED",
			"cancelled_at":   time.Now(),
			"refund_amount":  refundAmount,
			"actual_revenue": actualRevenue,
		}).Error; err != nil {
			return err
		}

		// Cập nhật tất cả vé
		if err := tx.Model(&model.Ticket{}).
			Where("order_id = ?", order.ID).
			Update("status", "CANCELLED").Error; err != nil {
			return err
		}

		// Giải phóng ghế
		if err := tx.Model(&model.ShowtimeSeat{}).
			Where("showtime_id = ? AND seat_id IN (SELECT seat_id FROM tickets WHERE order_id = ?)", order.ShowtimeID, order.ID).
			Update("status", "AVAILABLE").Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return utils.ErrorResponse(c, 500, "Hủy vé thất bại", err)
	}

	// Gửi email thông báo hủy + hoàn tiền (async)
	go SendCancelConfirmationEmail(order, refundAmount)

	return utils.SuccessResponse(c, 200, fiber.Map{
		"message":        "Hủy vé thành công",
		"refund_amount":  refundAmount,
		"refund_percent": refundPercent * 100,
	})
}

// Query params: ?orderCode=ORD-ABC123&ticketCodes=TKT-123,TKT-456
func CancelOrderByCode(c *fiber.Ctx) error {
	orderCode := c.Query("orderCode")
	rawTicketCodes := c.Query("ticketCodes") // "TKT-123,TKT-456" hoặc rỗng (hủy cả đơn)

	if orderCode == "" {
		return utils.ErrorResponse(c, 400, "Mã đơn hàng không hợp lệ", nil)
	}

	var order model.Order
	if err := database.DB.Preload("Tickets").Preload("Showtime").First(&order, "public_code = ?", orderCode).Error; err != nil {
		return utils.ErrorResponse(c, 404, "Đơn hàng không tồn tại hoặc đã bị hủy", err)
	}

	// Kiểm tra trạng thái đơn hàng
	if order.Status != "PAID" {
		return utils.ErrorResponse(c, 400, "Đơn hàng đã bị hủy hoặc không hợp lệ", nil)
	}

	// Kiểm tra thời gian hủy: trước giờ chiếu ít nhất 60 phút (có thể tùy chỉnh)
	if time.Now().Add(60 * time.Minute).After(order.Showtime.StartTime) {
		return utils.ErrorResponse(c, 400, "Chỉ được hủy trước giờ chiếu ít nhất 60 phút", nil)
	}

	// Danh sách vé cần hủy
	var ticketsToCancel []model.Ticket
	if rawTicketCodes == "" {
		// Hủy toàn bộ vé trong đơn
		ticketsToCancel = order.Tickets
	} else {
		codes := strings.Split(rawTicketCodes, ",")
		codeMap := make(map[string]bool)
		for _, code := range codes {
			codeMap[strings.TrimSpace(code)] = true
		}

		for _, t := range order.Tickets {
			if codeMap[t.TicketCode] {
				ticketsToCancel = append(ticketsToCancel, t)
			}
		}

		if len(ticketsToCancel) == 0 {
			return utils.ErrorResponse(c, 400, "Không tìm thấy vé hợp lệ để hủy", nil)
		}
	}

	// Transaction
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	now := time.Now()

	for _, ticket := range ticketsToCancel {
		// Giải phóng ghế
		if err := tx.Model(&model.ShowtimeSeat{}).
			Where("id = ?", ticket.ShowtimeSeatId).
			Updates(map[string]any{
				"status":     SeatAvailable,
				"held_by":    "",
				"expired_at": nil,
			}).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Lỗi giải phóng ghế", err)
		}

		// Cập nhật vé
		if err := tx.Model(&ticket).
			Updates(map[string]any{
				"status":       "CANCELLED",
				"cancelled_at": now,
			}).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Lỗi cập nhật vé", err)
		}
	}

	// Nếu hủy hết vé → hủy đơn
	if len(ticketsToCancel) == len(order.Tickets) {
		order.Status = "CANCELLED"
		order.CancelledAt = &now
		if err := tx.Save(&order).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Lỗi cập nhật đơn hàng", err)
		}
	}

	tx.Commit()

	// Broadcast realtime
	BroadcastShowtime(order.ShowtimeID)

	return utils.SuccessResponse(c, 200, fiber.Map{
		"message":           "Hủy vé thành công! Tiền sẽ được hoàn lại trong 3-7 ngày làm việc.",
		"cancelled_tickets": len(ticketsToCancel),
	})
}

// POST /api/v1/orders/:publicCode/cancel
func CancelOrderByUser(c *fiber.Ctx) error {
	publicCode := c.Params("publicCode")
	if publicCode == "" {
		return utils.ErrorResponse(c, 400, "Mã đơn hàng không hợp lệ", nil)
	}

	customer, ok := c.Locals("customer").(*model.Customer)
	if !ok || customer == nil {
		return utils.ErrorResponse(c, 401, "Vui lòng đăng nhập", nil)
	}

	var order model.Order
	if err := database.DB.
		Preload("Tickets").
		Preload("Showtime").
		Where("public_code = ? AND (customer_id = ? OR customer_id IS NULL)", publicCode, customer.ID).
		First(&order).Error; err != nil {
		return utils.ErrorResponse(c, 404, "Đơn hàng không tồn tại hoặc không thuộc về bạn", err)
	}

	// Kiểm tra trạng thái + thời gian
	if order.Status != "PAID" {
		return utils.ErrorResponse(c, 400, "Đơn hàng không thể hủy", nil)
	}

	if time.Now().Add(60 * time.Minute).After(order.Showtime.StartTime) {
		return utils.ErrorResponse(c, 400, "Chỉ được hủy trước giờ chiếu ít nhất 60 phút", nil)
	}

	tx := database.DB.Begin()
	now := time.Now()

	for _, ticket := range order.Tickets {
		// Giải phóng ghế
		tx.Model(&model.ShowtimeSeat{}).
			Where("id = ?", ticket.ShowtimeSeatId).
			Updates(map[string]any{
				"status":     SeatAvailable,
				"held_by":    "",
				"expired_at": nil,
			})

		// Hủy vé
		tx.Model(&ticket).Updates(map[string]any{
			"status":       "CANCELLED",
			"cancelled_at": now,
		})
	}

	// Hủy đơn
	order.Status = "CANCELLED"
	order.CancelledAt = &now
	tx.Save(&order)

	tx.Commit()

	BroadcastShowtime(order.ShowtimeID)

	return utils.SuccessResponse(c, 200, "Hủy đơn hàng thành công!")
}
