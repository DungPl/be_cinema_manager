package handler

import (
	"bytes"
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gopkg.in/gomail.v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	TicketIssued    = "ISSUED"    // Đã phát hành
	TicketUsed      = "USED"      // Đã check-in
	TicketExpired   = "EXPIRED"   // Hết hạn
	TicketCancelled = "CANCELLED" // Hủy
)

// Auto cleanup expired ShowtimeSeat(chạy mỗi 10 phút)
func CleanupExpiredReservations() {
	database.DB.Where("status = ? AND expires_at < ?", "ACTIVE", time.Now()).Update("status", "EXPIRED")
}
func ExpireTickets() {
	db := database.DB

	now := time.Now()

	// Tìm vé chưa check-in, suất chiếu đã bắt đầu
	var expiredTickets []model.Ticket
	err := db.
		Joins("JOIN showtimes ON showtimes.id = tickets.showtime_id").
		Where("tickets.status = ? AND showtimes.start_time < ?", "ISSUED", now.Add(-30*time.Minute)).
		Find(&expiredTickets).Error

	if err != nil {
		log.Printf("Lỗi tìm vé hết hạn: %v", err)
		return
	}

	if len(expiredTickets) == 0 {
		return
	}

	// Cập nhật trạng thái vé sang EXPIRED
	for _, ticket := range expiredTickets {
		ticket.Status = "EXPIRED"
		ticket.UpdatedAt = now
		if err := db.Save(&ticket).Error; err != nil {
			log.Printf("Lỗi cập nhật vé %s: %v", ticket.TicketCode, err)
		}
	}

	log.Printf("Đã expire %d vé hết hạn", len(expiredTickets))
}
func CreateTicket(c *fiber.Ctx) error {
	input := c.Locals("input").(model.CreateTicketInput)

	db := database.DB
	tx := db.Begin()
	// Tạo mã đặt vé unique
	var showtime model.Showtime
	if err := tx.Preload("Movie").First(&showtime, input.ShowtimeID).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, 404, "Suất chiếu không tồn tại", err)
	}
	var heldSeats []model.ShowtimeSeat
	for _, seatId := range input.SeatIds {
		var stSeat model.ShowtimeSeat
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("seat_id = ? AND showtime_id = ? AND status = ?", seatId, showtime.ID, SeatAvailable).
			First(&stSeat).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 400, "Ghế không khả dụng", err)
		}
		heldSeats = append(heldSeats, stSeat)
	}
	totalAmount := CalculateTotalAmount(heldSeats) // Hàm tính tổng tiền
	now := time.Now()
	order := model.Order{
		PublicCode:    "ORD-COUNTER-" + uuid.New().String()[:8],
		CustomerID:    nil, // Guest hoặc tạo customer mới nếu cần
		ShowtimeID:    showtime.ID,
		TotalAmount:   totalAmount,
		Status:        "PAID",
		PaymentMethod: "CASH_COUNTER",
		PaidAt:        &now,
	}

	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, 500, "Không thể tạo đơn hàng", err)
	}

	// Tạo tickets & update ghế sang SOLD
	var tickets []model.Ticket
	var seatLabels []string
	for _, seat := range heldSeats {
		ticket := model.Ticket{
			OrderId:        order.ID,
			ShowtimeSeatId: seat.ID,
			TicketCode:     "TKT-COUNTER-" + uuid.New().String()[:10],
			Status:         "ISSUED",
			IssuedAt:       now,
			SeatId:         seat.SeatId,
			ShowtimeId:     showtime.ID,
		}
		tickets = append(tickets, ticket)

		tx.Model(&seat).Update("status", "SOLD")
		seatLabels = append(seatLabels, fmt.Sprintf("%s%d", seat.Seat.Row, seat.Seat.Column))
	}

	tx.Create(&tickets)

	tx.Commit()
	// Broadcast realtime
	BroadcastShowtime(showtime.ID)

	// Gửi email (async)
	email := ""
	if input.Email != "" {
		email = input.Email // Giả sử Customer có trường Email
	}

	if email != "" {
		detailLink := fmt.Sprintf("http://localhost:5173/don-hang/%s", order.PublicCode)
		cancelLink := fmt.Sprintf("http://localhost:5173/huy-don/%s", order.PublicCode)

		data := utils.OrderConfirmationData{
			OrderCode:     order.PublicCode,
			MovieName:     showtime.Movie.Title,
			Showtime:      showtime.StartTime.Format("02/01/2006 15:04"),
			Seats:         strings.Join(seatLabels, ", "),
			TotalAmount:   totalAmount,
			PaymentMethod: order.PaymentMethod,
			DetailLink:    detailLink,
			CancelledLink: cancelLink,
		}

		// Async gửi email
		go func() {
			// Render template HTML
			tmplPath := "templates/order_confirmation.html"
			tmpl, err := template.ParseFiles(tmplPath)
			if err != nil {
				log.Printf("Lỗi load template: %v", err)
				return
			}

			var htmlBody bytes.Buffer
			if err := tmpl.Execute(&htmlBody, data); err != nil {
				log.Printf("Lỗi render template: %v", err)
				return
			}

			// Tạo message gomail
			m := gomail.NewMessage()
			m.SetHeader("From", os.Getenv("SMTP_FROM"))
			m.SetHeader("To", email)
			m.SetHeader("Subject", "Xác nhận đơn hàng #"+order.PublicCode)
			m.SetBody("text/html", htmlBody.String())

			// Đính kèm QR code cho từng vé
			for _, ticket := range tickets {
				qrContent := fmt.Sprintf("https://yourdomain.com/checkin/%s", ticket.TicketCode) // Link check-in hoặc mã vé
				qrBytes, err := utils.GenerateQRCode(qrContent, 256)                             // Kích thước 256x256
				if err != nil {
					log.Printf("Lỗi tạo QR cho vé %s: %v", ticket.TicketCode, err)
					continue
				}

				filename := fmt.Sprintf("Ve_%s.png", ticket.TicketCode)

				// Attach bytes as file
				m.Attach(filename, gomail.Rename(filename), gomail.SetCopyFunc(func(w io.Writer) error {
					_, err := io.Copy(w, bytes.NewReader(qrBytes))
					return err
				}))
			}

			// Gửi email
			d := gomail.NewDialer(os.Getenv("SMTP_HOST"), 587, os.Getenv("SMTP_USERNAME"), os.Getenv("SMTP_PASSWORD"))
			if err := d.DialAndSend(m); err != nil {
				log.Printf("Lỗi gửi email: %v", err)
			} else {
				log.Printf("Email xác nhận + QR đã gửi đến %s", email)
			}
		}()
	}

	// Trả response
	return utils.SuccessResponse(c, 200, fiber.Map{
		"order":           order,
		"tickets":         tickets,
		"message":         "Thanh toán và tạo vé thành công",
		"emailSent":       email != "",
		"orderPublicCode": order.PublicCode,
	})
}

func CheckInTicket(c *fiber.Ctx) error {
	ticketCode := c.Params("ticketCode")

	var ticket model.Ticket
	if err := database.DB.First(&ticket, "ticket_code = ?", ticketCode).Error; err != nil {
		return utils.ErrorResponse(c, 404, "Vé không tồn tại", err)
	}

	// Kiểm tra trạng thái
	if ticket.Status == "USED" {
		return utils.ErrorResponse(c, 400, "Vé đã được sử dụng", nil)
	}
	if ticket.Status != "ISSUED" {
		return utils.ErrorResponse(c, 400, "Vé không hợp lệ", nil)
	}

	// Kiểm tra thời gian (ví dụ: không check-in sau khi phim bắt đầu 30 phút)
	var showtime model.Showtime
	database.DB.First(&showtime, ticket.ShowtimeId)
	if time.Now().After(showtime.StartTime.Add(30 * time.Minute)) {
		return utils.ErrorResponse(c, 400, "Suất chiếu đã bắt đầu quá lâu", nil)
	}

	// Check-in thành công
	now := time.Now()
	ticket.Status = "USED"
	ticket.UsedAt = &now
	if err := database.DB.Save(&ticket).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Lỗi cập nhật vé", err)
	}

	return utils.SuccessResponse(c, 200, fiber.Map{
		"message":    "Check-in thành công",
		"ticketCode": ticket.TicketCode,
		"seatLabel":  ticket.ShowtimeSeat.Seat.Row + strconv.Itoa(ticket.ShowtimeSeat.Seat.Column),
		"usedAt":     now.Format("02/01/2006 15:04"),
	})
}
func GetTicket(c *fiber.Ctx) error {
	customerInfo, _ := helper.GetInfoCustomerFromToken(c)
	customerId := customerInfo.AccountId

	db := database.DB

	var customer model.Customer

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
	var tickets []model.Ticket
	currentTime := time.Now().In(time.FixedZone("ICT", 7*3600))

	if err := database.DB.Preload("Showtime").Preload("Showtime.Movie").Preload("Showtime.Room").
		Preload("Seat").Preload("Seat.SeatType").
		Where("user_id = ? AND deleted_at IS NULL AND showtime.start_time > ?", customerId, currentTime).
		Order("created_at desc").Find(&tickets).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "Lấy vé thành công",
		"data":    tickets,
		"total":   len(tickets),
	})
}
func GetTicketAdmin(c *fiber.Ctx) error {

	_, isAdmin, isQuanLy, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isQuanLy {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not permission"))
	}
	filterInput := new(model.FilterTicketInput)
	if err := c.QueryParser(filterInput); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INPUT, err)
	}

	db := database.DB
	var tickets []model.Ticket
	condition := db.Preload("Showtime").Preload("Showtime.Movie").Preload("Showtime.Room").
		Preload("User").Preload("Seat").Preload("Seat.SeatType").
		Where("tickets.deleted_at IS NULL")

	if filterInput.ShowtimeId > 0 {
		condition = condition.Where("tickets.showtime_id = ?", filterInput.ShowtimeId)
	}
	if filterInput.Status != "" {
		condition = condition.Where("tickets.status = ?", filterInput.Status)
	}
	if filterInput.StartDate != nil {
		condition = condition.Where("showtimes.start_time >= ?", filterInput.StartDate)
	}
	if filterInput.EndDate != nil {
		condition = condition.Where("showtimes.start_time <= ?", filterInput.EndDate)
	}

	var totalCount int64
	condition.Count(&totalCount)

	condition = utils.ApplyPagination(condition, filterInput.Limit, filterInput.Page)
	condition.Order("tickets.created_at desc").Find("tickets")
	response := &model.ResponseCustom{
		Rows:       tickets,
		Limit:      filterInput.Limit,
		Page:       filterInput.Page,
		TotalCount: totalCount,
	}
	return utils.SuccessResponse(c, fiber.StatusOK, response)
}
func GetTicketById(c *fiber.Ctx) error {
	ticketId := c.Locals("ticketId").(int)

	var ticket model.Ticket
	if err := database.DB.Preload("Showtime").Preload("Showtime.Movie").Preload("Showtime.Room").
		Preload("Customer").Preload("Seat").Preload("Seat.SeatType").
		Where("id = ?", ticketId).First(&ticket).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Vé không tồn tại"})
	}

	return utils.SuccessResponse(c, fiber.StatusOK, ticket)
}

// ============================================
// 7. GET /api/v1/ticket/:id/qrcode - QR CODE VÉ
// ============================================
func GetTicketQRCode(c *fiber.Ctx) error {
	ticketId, _ := c.ParamsInt("id")

	var ticket model.Ticket
	if err := database.DB.Where("id = ?", ticketId).First(&ticket).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Vé không tồn tại"})
	}

	// Generate QR Code URL (giả lập)
	qrUrl := fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=TKT-%s", ticket.TicketCode)

	return c.JSON(fiber.Map{
		"message": "Lấy QR Code thành công",
		"data": fiber.Map{
			"ticketId":    ticket.ID,
			"bookingCode": ticket.TicketCode,
			"qrUrl":       qrUrl,
		},
	})
}

// ============================================
// 9. GET /api/v1/ticket/stats - THỐNG KÊ VÉ (Admin)
// ============================================
// func GetTicketStats(c *fiber.Ctx) error {
// 	// Kiểm tra admin
// 	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
// 	if !isAdmin {
// 		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not permission"))
// 	}

// 	currentTime := time.Now().In(time.FixedZone("ICT", 7*3600))

// 	var stats struct {
// 		TotalTickets     int     `json:"totalTickets"`
// 		TotalRevenue     float64 `json:"totalRevenue"`
// 		TodayTickets     int     `json:"todayTickets"`
// 		TodayRevenue     float64 `json:"todayRevenue"`
// 		UpcomingTickets  int     `json:"upcomingTickets"`
// 		CancelledTickets int     `json:"cancelledTickets"`
// 		OccupancyRate    float64 `json:"occupancyRate"`
// 	}

// 	// Tổng vé & doanh thu
// 	database.DB.Model(&model.Ticket{}).Where("status = ? AND deleted_at IS NULL", "BOOKED").Count(&stats.TotalTickets)
// 	database.DB.Model(&model.Ticket{}).Where("status = ? AND deleted_at IS NULL", "BOOKED").Select("SUM(price)").Scan(&stats.TotalRevenue)

// 	// Vé hôm nay
// 	today := currentTime.Truncate(24 * time.Hour)
// 	database.DB.Joins("JOIN showtimes ON showtimes.id = tickets.showtime_id").
// 		Where("tickets.status = ? AND tickets.deleted_at IS NULL AND DATE(showtimes.start_time) = ?", "BOOKED", today.Format("2006-01-02")).
// 		Count(&stats.TodayTickets)
// 	database.DB.Joins("JOIN showtimes ON showtimes.id = tickets.showtime_id").
// 		Where("tickets.status = ? AND tickets.deleted_at IS NULL AND DATE(showtimes.start_time) = ?", "BOOKED", today.Format("2006-01-02")).
// 		Select("SUM(tickets.price)").Scan(&stats.TodayRevenue)

// 	// Upcoming
// 	database.DB.Joins("JOIN showtimes ON showtimes.id = tickets.showtime_id").
// 		Where("tickets.status = ? AND tickets.deleted_at IS NULL AND showtimes.start_time > ?", "BOOKED", currentTime).
// 		Count(&stats.UpcomingTickets)

// 	// Cancelled
// 	database.DB.Model(&model.Ticket{}).Where("status = ? AND deleted_at IS NULL", "CANCELLED").Count(&stats.CancelledTickets)

// 	// Occupancy Rate (giả lập 75%)
// 	stats.OccupancyRate = 75.5

// 	return c.JSON(fiber.Map{
// 		"message": "Lấy thống kê thành công",
// 		"data":    stats,
// 	})
// }
