package handler

import (
	"bytes"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gopkg.in/gomail.v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	seatConnections = make(map[uint]map[*websocket.Conn]bool)
	seatMutex       sync.Mutex
)

type SeatUI struct {
	Id        uint       `json:"id"`
	Label     string     `json:"label"`
	Type      string     `json:"type"`
	Status    string     `json:"status"`
	HeldBy    string     `json:"heldBy,omitempty"`
	ExpiredAt *time.Time `json:"expiredAt,omitempty"`
	CoupleId  *uint      `json:"coupleId,omitempty"`
}

// WebSocket handler cho ghế suất chiếu
func SeatWebsocket(c *websocket.Conn) {
	showtimeIdStr := c.Params("showtimeId")
	showtimeId, err := strconv.ParseUint(showtimeIdStr, 10, 64)
	if err != nil {
		log.Printf("Invalid showtimeId: %s", showtimeIdStr)
		c.Close()
		return
	}
	id := uint(showtimeId)

	// Thêm connection vào map
	seatMutex.Lock()
	if seatConnections[id] == nil {
		seatConnections[id] = make(map[*websocket.Conn]bool)
	}
	seatConnections[id][c] = true
	seatMutex.Unlock()

	log.Printf("New WS connection for showtime %d. Total connections: %d", id, len(seatConnections[id]))

	defer func() {
		seatMutex.Lock()
		delete(seatConnections[id], c)
		if len(seatConnections[id]) == 0 {
			delete(seatConnections, id)
		}
		seatMutex.Unlock()
		c.Close()
		log.Printf("WS connection closed for showtime %d. Total remaining: %d", id, len(seatConnections[id]))
	}()

	// Gửi ngay trạng thái ghế hiện tại cho client mới connect
	BroadcastShowtime(id)

	// Loop để giữ connection (không cần đọc message nếu không có logic client gửi)
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			break
		}
	}
}

// Broadcast full state ghế cho showtimeId (khi client mới connect)
func BroadcastShowtime(showtimeId uint) {
	db := database.DB

	var seats []model.ShowtimeSeat
	if err := db.
		Preload("Seat").
		Preload("Seat.SeatType").
		Where("showtime_id = ?", showtimeId).
		Find(&seats).Error; err != nil {
		log.Printf("Error loading seats for broadcast: %v", err)
		return
	}

	result := make(map[string][]SeatUI)
	for _, s := range seats {
		// if s.Seat == nil {
		// 	log.Printf("Seat not preloaded for ShowtimeSeat ID %d", s.ID)
		// 	continue
		// }
		row := s.Seat.Row
		if _, ok := result[row]; !ok {
			result[row] = []SeatUI{}
		}
		result[row] = append(result[row], SeatUI{
			Id:        s.SeatId,
			Label:     fmt.Sprintf("%s%d", s.Seat.Row, s.Seat.Column),
			Type:      s.Seat.SeatType.Type,
			Status:    s.Status,
			HeldBy:    s.HeldBy,
			ExpiredAt: s.ExpiredAt,
			CoupleId:  s.Seat.CoupleId,
		})
	}

	seatMutex.Lock()
	conns, ok := seatConnections[showtimeId]
	if !ok {
		seatMutex.Unlock()
		return
	}
	seatMutex.Unlock()

	for conn := range conns {
		if err := conn.WriteJSON(result); err != nil {
			log.Printf("Error broadcasting full state: %v", err)
		}
	}
}

// Broadcast chỉ ghế thay đổi (delta)
func BroadcastSeatChange(showtimeId uint, updatedSeats []model.ShowtimeSeat) {
	seatMutex.Lock()
	conns, ok := seatConnections[showtimeId]
	if !ok {
		seatMutex.Unlock()
		return
	}
	seatMutex.Unlock()

	delta := make(map[string][]SeatUI)
	for _, s := range updatedSeats {
		// if s.Seat == nil {
		// 	log.Printf("Seat not preloaded in updatedSeats ID %d", s.ID)
		// 	continue
		// }
		row := s.Seat.Row
		delta[row] = append(delta[row], SeatUI{
			Id:        s.SeatId,
			Label:     fmt.Sprintf("%s%d", s.Seat.Row, s.Seat.Column),
			Type:      s.Seat.SeatType.Type,
			Status:    s.Status,
			HeldBy:    s.HeldBy,
			ExpiredAt: s.ExpiredAt,
			CoupleId:  s.Seat.CoupleId,
		})
	}

	for conn := range conns {
		if err := conn.WriteJSON(delta); err != nil {
			log.Printf("Error broadcasting delta: %v", err)
		}
	}
}

const HoldTimeout = 10 * time.Minute
const (
	SeatAvailable  = "AVAILABLE"
	SeatHeld       = "HELD"
	SeatSeatBooked = "BOOKED"
)

func HoldSeat(c *fiber.Ctx) error {
	db := database.DB

	code := c.Params("code")

	var input struct {
		SeatIds        []uint `json:"seatIds" validate:"required"`
		GuestSessionId string `json:"guestSessionId"` // Nếu guest
	}

	if err := c.BodyParser(&input); err != nil {
		return utils.ErrorResponse(c, 400, "Invalid input", err)
	}

	if len(input.SeatIds) == 0 {
		return utils.ErrorResponse(c, 400, "Seat IDs are required", nil)
	}

	// Lấy HeldBy
	customer, _ := c.Locals("customer").(*model.Customer)
	heldBy := ""
	if customer != nil {
		heldBy = fmt.Sprintf("USER_%d", customer.ID)
	} else {
		if input.GuestSessionId != "" {
			heldBy = input.GuestSessionId
		} else {
			heldBy = "GUEST_" + uuid.New().String() // Tạo session ID cho guest
		}
	}

	tx := db.Begin()

	// Tìm suất chiếu
	var showtime model.Showtime
	if err := tx.Where("public_code = ?", code).First(&showtime).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, 404, "Showtime not found", err)
	}
	if showtime.StartTime.Before(time.Now()) {
		tx.Rollback()
		return utils.ErrorResponse(c, 400, "Showtime already started", nil)
	}

	var updatedSeats []model.ShowtimeSeat
	for _, seatId := range input.SeatIds {
		var stSeat model.ShowtimeSeat
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Preload("Seat").
			Preload("Seat.SeatType").
			Where("seat_id = ? AND showtime_id = ? AND status = ?", seatId, showtime.ID, SeatAvailable).
			First(&stSeat).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 400, fmt.Sprintf("Seat %d not available", seatId), err)
		}

		// Hold ghế
		expTime := time.Now().Add(HoldTimeout)
		if err := tx.Model(&stSeat).Updates(map[string]any{
			"status":     SeatHeld,
			"held_by":    heldBy,
			"expired_at": expTime,
		}).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Cannot hold seat", err)
		}
		updatedSeats = append(updatedSeats, stSeat)

		// Nếu ghế đôi → hold ghế còn lại
		if stSeat.Seat.CoupleId != nil {
			coupleId := *stSeat.Seat.CoupleId
			var coupleStSeat model.ShowtimeSeat
			if err := tx.
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("seat_id = ? AND showtime_id = ? AND status = ?", coupleId, showtime.ID, SeatAvailable).
				First(&coupleStSeat).Error; err == nil {
				if err := tx.Model(&coupleStSeat).Updates(map[string]any{
					"status":     SeatHeld,
					"held_by":    heldBy,
					"expired_at": expTime,
				}).Error; err != nil {
					tx.Rollback()
					return utils.ErrorResponse(c, 500, "Cannot hold couple seat", err)
				}
				updatedSeats = append(updatedSeats, coupleStSeat)
			}
		}
	}

	tx.Commit()
	BroadcastSeatChange(showtime.ID, updatedSeats)

	return utils.SuccessResponse(c, 200, fiber.Map{
		"heldSeatIds": input.SeatIds,
		"expiresAt":   time.Now().Add(HoldTimeout),
		"heldBy":      heldBy, // Trả về cho guest
	})
}

func ReleaseSeat(c *fiber.Ctx) error {
	db := database.DB
	code := c.Params("code")

	var input struct {
		SeatIds []uint `json:"seatIds" validate:"required"`
		HeldBy  string `json:"heldBy" validate:"required"`
	}
	if err := c.BodyParser(&input); err != nil {
		return utils.ErrorResponse(c, 400, "Invalid input", err)
	}

	log.Printf("ReleaseSeat - input: %+v", input) // Debug input

	var showtime model.Showtime
	if err := db.Where("public_code = ?", code).First(&showtime).Error; err != nil {
		return utils.ErrorResponse(c, 404, "Showtime not found", err)
	}

	tx := db.Begin()

	var updatedSeats []model.ShowtimeSeat
	for _, seatId := range input.SeatIds {
		var stSeat model.ShowtimeSeat
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Preload("Seat").
			Where("seat_id = ? AND showtime_id = ? AND status = ? AND held_by = ?",
				seatId, showtime.ID, SeatHeld, input.HeldBy).
			First(&stSeat).Error; err != nil {
			tx.Rollback()
			log.Printf("Seat %d not held by %s: %v", seatId, input.HeldBy, err)
			return utils.ErrorResponse(c, 400, fmt.Sprintf("Ghế %d không được giữ bởi bạn (heldBy: %s)", seatId, input.HeldBy), err)
		}

		// Release ghế
		if err := tx.Model(&stSeat).Updates(map[string]any{
			"status":     SeatAvailable,
			"held_by":    "",
			"expired_at": nil,
		}).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Cannot release seat", err)
		}
		updatedSeats = append(updatedSeats, stSeat)

		// Release ghế đôi nếu có
		if stSeat.Seat.CoupleId != nil {
			coupleId := *stSeat.Seat.CoupleId
			var coupleStSeat model.ShowtimeSeat
			if err := tx.
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("seat_id = ? AND showtime_id = ? AND status = ? AND held_by = ?",
					coupleId, showtime.ID, SeatHeld, input.HeldBy).
				First(&coupleStSeat).Error; err == nil {
				if err := tx.Model(&coupleStSeat).Updates(map[string]any{
					"status":     SeatAvailable,
					"held_by":    "",
					"expired_at": nil,
				}).Error; err != nil {
					tx.Rollback()
					return utils.ErrorResponse(c, 500, "Cannot release couple seat", err)
				}
				updatedSeats = append(updatedSeats, coupleStSeat)
			}
		}
	}

	tx.Commit()
	BroadcastSeatChange(showtime.ID, updatedSeats)
	BroadcastShowtime(showtime.ID)

	return utils.SuccessResponse(c, 200, "Released")
}
func GetHeldSeatsBySession(c *fiber.Ctx) error {
	code := c.Params("code")
	sessionId := c.Query("sessionId")

	if sessionId == "" {
		return utils.ErrorResponse(c, 400, "Session ID required", nil)
	}

	var showtime model.Showtime
	if err := database.DB.Where("public_code = ?", code).First(&showtime).Error; err != nil {
		return utils.ErrorResponse(c, 404, "Showtime not found", err)
	}

	var heldSeats []model.ShowtimeSeat
	if err := database.DB.
		Preload("Seat").
		Where("showtime_id = ? AND status = ? AND held_by = ? AND expired_at > ?", showtime.ID, SeatHeld, sessionId, time.Now()).
		Find(&heldSeats).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Error fetching held seats", err)
	}

	return utils.SuccessResponse(c, 200, heldSeats)
}
func PurchaseSeats(c *fiber.Ctx) error {
	db := database.DB
	code := c.Params("code")

	var input struct {
		SeatIds       []uint `json:"seatIds" validate:"required"`
		HeldBy        string `json:"heldBy" validate:"required"`
		PaymentMethod string `json:"paymentMethod" validate:"required"`
		CustomerName  string `json:"customer_name"`
		Phone         string `json:"phone"`
		Email         string `json:"email,omitempty"`
	}

	if err := c.BodyParser(&input); err != nil {
		return utils.ErrorResponse(c, 400, "Invalid input", err)
	}

	//log.Printf("PurchaseSeats - input: %+v", input)

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var showtime model.Showtime
	if err := tx.Preload("Movie").Where("public_code = ?", code).First(&showtime).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, 404, "Suất chiếu không tồn tại", err)
	}

	var heldSeats []model.ShowtimeSeat
	if err := tx.Where("showtime_id = ? AND seat_id IN ? AND status = ? AND held_by = ?",
		showtime.ID, input.SeatIds, "HELD", input.HeldBy).
		Preload("Seat.SeatType").
		Preload("Showtime").
		Preload("Seat").Find(&heldSeats).Error; err != nil || len(heldSeats) != len(input.SeatIds) {
		tx.Rollback()
		return utils.ErrorResponse(c, 400, "Một số ghế không hợp lệ hoặc đã hết hạn giữ chỗ", nil)
	}

	totalAmount := CalculateTotalAmount(heldSeats)

	now := time.Now()
	order := model.Order{
		PublicCode:    "ORD-" + uuid.New().String()[:8],
		CustomerID:    nil,
		ShowtimeID:    showtime.ID,
		TotalAmount:   totalAmount,
		ActualRevenue: totalAmount,
		Status:        "PAID",
		PaymentMethod: input.PaymentMethod,
		PaidAt:        &now,
		CustomerName:  input.CustomerName,
		Phone:         input.Phone,
		Email:         input.Email,
	}

	// Lấy customer từ Locals
	customer, isLoggedIn := c.Locals("customer").(*model.Customer)
	//log.Printf("PurchaseSeats - isLoggedIn: %v, customer: %+v", isLoggedIn, customer)

	if isLoggedIn && customer != nil {
		order.CustomerID = &customer.ID
		//log.Printf("PurchaseSeats - Customer ID: %d", customer.ID)
	} else {
		//log.Println("PurchaseSeats - Guest checkout")
	}

	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, 500, "Không thể tạo đơn hàng", err)
	}

	// Tạo Tickets và cập nhật ghế
	var tickets []model.Ticket
	var seatLabels []string
	for _, seat := range heldSeats {
		log.Printf("Ghế %s%d - Loại: %s - Modifier: %.2f - Giá vé: %.0f",
			seat.Seat.Row, seat.Seat.Column,
			seat.Seat.SeatType.Type,
			seat.Seat.SeatType.PriceModifier,
			showtime.Price*seat.Seat.SeatType.PriceModifier)
		ticketPrice := showtime.Price * seat.Seat.SeatType.PriceModifier
		ticket := model.Ticket{
			OrderId:        order.ID,
			ShowtimeSeatId: seat.ID,
			TicketCode:     "TKT-" + uuid.New().String()[:10],
			Status:         "ISSUED",
			IssuedAt:       time.Now(),
			Price:          ticketPrice,
			SeatId:         seat.SeatId, // ← Sửa: Lấy SeatId từ ShowtimeSeat (id ghế thật)
			ShowtimeId:     showtime.ID, // ← Thêm nếu cần
		}
		tickets = append(tickets, ticket)

		if err := tx.Model(&seat).Updates(map[string]any{
			"status":     "SOLD",
			"held_by":    "",
			"expired_at": nil,
		}).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Không thể cập nhật trạng thái ghế", err)
		}

		seatLabels = append(seatLabels, fmt.Sprintf("%s%d", seat.Seat.Row, seat.Seat.Column))
	}

	if err := tx.Create(&tickets).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, 500, "Không thể tạo vé", err)
	}

	tx.Commit()

	// Broadcast realtime
	BroadcastShowtime(showtime.ID)

	// Gửi email (async)
	email := ""
	if isLoggedIn && customer != nil {
		email = customer.Email // Giả sử Customer có trường Email
	} else if input.Email != "" {
		email = input.Email // Guest nhập email
	}

	if email != "" {
		detailLink := fmt.Sprintf("http://localhost:5173/don-hang/%s", order.PublicCode)
		cancelLink := fmt.Sprintf("https://yourdomain.com/api/v1/orders/cancel-by-code?orderCode=%s", order.PublicCode)

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
			m.SetHeader("From", "CinemaPro <cinema_hub@gmail.com>")
			m.SetHeader("To", email)
			m.SetHeader("Subject", "Vé xem phim - Mã đơn: "+order.PublicCode)
			m.SetBody("text/html", htmlBody.String())

			// === TẠO QR VÀ NHÚNG INLINE VỚI CID ===
			qrContent := order.PublicCode
			qrBytes, err := utils.GenerateQRCode(qrContent, 400)
			if err != nil {
				log.Printf("Lỗi tạo QR: %v", err)
			} else {
				// Nhúng inline từ memory bằng SetCopyFunc
				m.Embed("qr_checkin.png", // tên file giả (không quan trọng)
					gomail.SetCopyFunc(func(w io.Writer) error {
						_, err := w.Write(qrBytes)
						return err
					}),
					gomail.SetHeader(map[string][]string{
						"Content-Type":        {"image/png"},
						"Content-ID":          {"<qr_checkin_code>"}, // trùng với cid: trong HTML
						"Content-Disposition": {"inline"},
					}),
				)
			}

			d := gomail.NewDialer(os.Getenv("SMTP_HOST"), 587, os.Getenv("SMTP_USERNAME"), os.Getenv("SMTP_PASSWORD"))
			if err := d.DialAndSend(m); err != nil {
				log.Printf("Lỗi gửi email: %v", err)
			} else {
				log.Printf("Email vé + QR đã gửi đến %s", email)
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

// Hàm tính tổng tiền (bạn cần implement theo logic giá ghế)
func CalculateTotalAmount(seats []model.ShowtimeSeat) float64 {
	var total float64
	processedCouples := make(map[uint]bool) // Đánh dấu cặp đã tính

	if len(seats) == 0 {
		return 0
	}
	basePrice := seats[0].Showtime.Price

	for _, s := range seats {
		modifier := s.Seat.SeatType.PriceModifier

		if s.Seat.CoupleId != nil && processedCouples[*s.Seat.CoupleId] {
			continue // Bỏ qua ghế thứ 2 của cặp
		}

		total += basePrice * modifier

		if s.Seat.CoupleId != nil {
			processedCouples[*s.Seat.CoupleId] = true
		}
	}

	return total
}

func ExpireSeats() {
	db := database.DB
	now := time.Now()

	var expiredSeats []model.ShowtimeSeat
	if err := db.
		Where("status = ? AND expired_at < ?", SeatHeld, now).
		Find(&expiredSeats).Error; err != nil {
		return
	}

	if len(expiredSeats) == 0 {
		return
	}

	tx := db.Begin()
	affectedShowtimes := make(map[uint]bool)

	for _, seat := range expiredSeats {
		err := tx.Model(&model.ShowtimeSeat{}).
			Where("id = ? AND status = ?", seat.ID, SeatHeld).
			Updates(map[string]any{
				"status":     SeatAvailable,
				"held_by":    "",
				"expired_at": nil,
			}).Error

		if err != nil {
			tx.Rollback()
			return
		}

		affectedShowtimes[seat.ShowtimeId] = true
	}

	tx.Commit()

	// Broadcast delta cho từng showtime
	for showtimeId := range affectedShowtimes {
		// Lấy ghế vừa expire
		var updatedSeats []model.ShowtimeSeat
		db.Where("showtime_id = ? AND status = ?", showtimeId, SeatAvailable).
			Where("expired_at IS NULL").
			Find(&updatedSeats) // Hoặc lọc chính xác hơn nếu cần

		BroadcastSeatChange(showtimeId, updatedSeats)
	}
}
func StartExpireSeatWorker() {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for range ticker.C {
			ExpireSeats()
		}
	}()
}

// handler/showtime_staff.go

const StaffHoldTimeout = 20 * time.Minute // 20 phút mặc định, có thể tùy chỉnh 15-30 phút

// API: Lấy ghế đang giữ bởi nhân viên hiện tại
func GetHeldSeatsForStaff(c *fiber.Ctx) error {
	db := database.DB
	code := c.Params("code")

	accountInfo, _, _, _, isBanve := helper.GetInfoAccountFromToken(c)
	if !isBanve {
		return utils.ErrorResponse(c, 403, "FORBIDDEN", nil)
	}

	heldBy := fmt.Sprintf("STAFF_%d", accountInfo.AccountId)

	var showtime model.Showtime
	if err := db.Where("public_code = ?", code).First(&showtime).Error; err != nil {
		return utils.ErrorResponse(c, 404, "Suất chiếu không tồn tại", err)
	}

	var heldSeats []model.ShowtimeSeat
	if err := db.
		Preload("Seat").
		Preload("Seat.SeatType").
		Where("showtime_id = ? AND status = ? AND held_by = ?", showtime.ID, SeatHeld, heldBy).
		Find(&heldSeats).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Lỗi lấy ghế đang giữ", err)
	}

	// Định nghĩa struct giống SeatUI trong GetSeatsByShowtime
	type SeatUI struct {
		Id            uint       `json:"id"`
		Label         string     `json:"label"`
		Type          string     `json:"type"`
		Status        string     `json:"status"`
		HeldBy        string     `json:"heldBy,omitempty"`
		ExpiredAt     *time.Time `json:"expiredAt,omitempty"`
		CoupleId      *uint      `json:"coupleId,omitempty"`
		PriceModifier float64    `json:"priceModifier"`
	}

	result := make([]SeatUI, 0, len(heldSeats))
	for _, s := range heldSeats {
		result = append(result, SeatUI{
			Id:            s.Seat.ID, // hoặc s.SeatId tùy model
			Label:         fmt.Sprintf("%s%d", s.Seat.Row, s.Seat.Column),
			Type:          s.Seat.SeatType.Type,
			Status:        s.Status,
			HeldBy:        s.HeldBy,
			ExpiredAt:     s.ExpiredAt,
			CoupleId:      s.Seat.CoupleId,
			PriceModifier: s.Seat.SeatType.PriceModifier,
		})
	}

	return utils.SuccessResponse(c, 200, fiber.Map{
		"heldSeats": result,
		// Tùy chọn: thêm thông tin heldBy chung nếu muốn
		"heldBy": heldBy,
	})
}

// Hold ghế cho STAFF (không expire tự động, nhưng có timeout để an toàn)
func HoldSeatForStaff(c *fiber.Ctx) error {
	db := database.DB

	code := c.Params("code")

	var input struct {
		SeatIds        []uint `json:"seatIds" validate:"required"`
		TimeoutMinutes int    `json:"timeoutMinutes"` // Tùy chọn: 15-30
	}

	if err := c.BodyParser(&input); err != nil {
		return utils.ErrorResponse(c, 400, "Invalid input", err)
	}

	if len(input.SeatIds) == 0 {
		return utils.ErrorResponse(c, 400, "Seat IDs are required", nil)
	}

	// Giới hạn timeout 15-30 phút
	timeout := StaffHoldTimeout
	if input.TimeoutMinutes >= 15 && input.TimeoutMinutes <= 30 {
		timeout = time.Duration(input.TimeoutMinutes) * time.Minute
	}

	// Kiểm tra quyền STAFF
	accountInfo, _, _, _, isBanve := helper.GetInfoAccountFromToken(c)
	if !isBanve {
		return utils.ErrorResponse(c, 403, "FORBIDDEN", nil)
	}
	if accountInfo.CinemaId == nil {
		return utils.ErrorResponse(c, 403, "Nhân viên chưa được gán rạp", nil)
	}

	tx := db.Begin()

	var showtime model.Showtime
	if err := tx.Where("public_code = ?", code).First(&showtime).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, 404, "Showtime not found", err)
	}

	var updatedSeats []model.ShowtimeSeat
	heldBy := fmt.Sprintf("STAFF_%d", accountInfo.AccountId)

	for _, seatId := range input.SeatIds {
		var stSeat model.ShowtimeSeat
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Preload("Seat").
			Where("seat_id = ? AND showtime_id = ? AND status = ?", seatId, showtime.ID, SeatAvailable).
			First(&stSeat).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 400, fmt.Sprintf("Ghế %d không khả dụng", seatId), err)
		}

		expTime := time.Now().Add(timeout)
		if err := tx.Model(&stSeat).Updates(map[string]any{
			"status":     SeatHeld,
			"held_by":    heldBy,
			"expired_at": expTime,
		}).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Không thể giữ ghế", err)
		}
		updatedSeats = append(updatedSeats, stSeat)

		// Xử lý ghế đôi
		if stSeat.Seat.CoupleId != nil {
			coupleId := *stSeat.Seat.CoupleId
			var coupleStSeat model.ShowtimeSeat
			if err := tx.
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("seat_id = ? AND showtime_id = ? AND status = ?", coupleId, showtime.ID, SeatAvailable).
				First(&coupleStSeat).Error; err == nil {
				if err := tx.Model(&coupleStSeat).Updates(map[string]any{
					"status":     SeatHeld,
					"held_by":    heldBy,
					"expired_at": expTime,
				}).Error; err != nil {
					tx.Rollback()
					return utils.ErrorResponse(c, 500, "Không thể giữ ghế đôi", err)
				}
				updatedSeats = append(updatedSeats, coupleStSeat)
			}
		}
	}

	tx.Commit()
	BroadcastSeatChange(showtime.ID, updatedSeats)

	return utils.SuccessResponse(c, 200, fiber.Map{
		"heldBy":    heldBy,
		"expiresAt": time.Now().Add(timeout),
	})
}

// Release ghế cho STAFF (tương tự ReleaseSeat nhưng chỉ cho STAFF)
func ReleaseSeatForStaff(c *fiber.Ctx) error {
	db := database.DB
	code := c.Params("code")

	var input struct {
		SeatIds []uint `json:"seatIds" validate:"required"`
	}
	if err := c.BodyParser(&input); err != nil {
		return utils.ErrorResponse(c, 400, "Invalid input", err)
	}

	accountInfo, _, _, _, isBanve := helper.GetInfoAccountFromToken(c)
	if !isBanve {
		return utils.ErrorResponse(c, 403, "FORBIDDEN", nil)
	}

	heldBy := fmt.Sprintf("STAFF_%d", accountInfo.AccountId)

	var showtime model.Showtime
	if err := db.Where("public_code = ?", code).First(&showtime).Error; err != nil {
		return utils.ErrorResponse(c, 404, "Suất chiếu không tồn tại", err)
	}

	tx := db.Begin()
	var updatedSeats []model.ShowtimeSeat

	for _, seatId := range input.SeatIds {
		var stSeat model.ShowtimeSeat
		// Chỉ kiểm tra HELD và held_by (không thêm kiểm tra thời gian)
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("seat_id = ? AND showtime_id = ? AND status = ? AND held_by = ?",
				seatId, showtime.ID, SeatHeld, heldBy).
			First(&stSeat).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 400, fmt.Sprintf("Ghế %d không được giữ bởi bạn", seatId), err)
		}

		// Release ngay lập tức
		if err := tx.Model(&stSeat).Updates(map[string]any{
			"status":     SeatAvailable,
			"held_by":    "",
			"expired_at": nil,
		}).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Không thể trả ghế", err)
		}
		updatedSeats = append(updatedSeats, stSeat)

		// Release ghế đôi nếu có
		if stSeat.Seat.CoupleId != nil {
			coupleId := *stSeat.Seat.CoupleId
			var coupleStSeat model.ShowtimeSeat
			if err := tx.
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("seat_id = ? AND showtime_id = ? AND status = ? AND held_by = ?",
					coupleId, showtime.ID, SeatHeld, heldBy).
				First(&coupleStSeat).Error; err == nil {
				if err := tx.Model(&coupleStSeat).Updates(map[string]any{
					"status":     SeatAvailable,
					"held_by":    "",
					"expired_at": nil,
				}).Error; err != nil {
					tx.Rollback()
					return utils.ErrorResponse(c, 500, "Không thể trả ghế đôi", err)
				}
				updatedSeats = append(updatedSeats, coupleStSeat)
			}
		}
	}

	tx.Commit()
	BroadcastSeatChange(showtime.ID, updatedSeats)

	return utils.SuccessResponse(c, 200, "Trả ghế thành công")
}

func CreateTicketForStaff(c *fiber.Ctx) error {
	db := database.DB
	code := c.Params("code")

	var input struct {
		SeatIds       []uint `json:"seatIds" validate:"required"`
		CustomerName  string `json:"customerName"`
		Phone         string `json:"phone"`
		Email         string `json:"email"`
		PaymentMethod string `json:"paymentMethod" validate:"required"`
	}

	if err := c.BodyParser(&input); err != nil {
		return utils.ErrorResponse(c, 400, "Invalid input", err)
	}

	// Kiểm tra quyền STAFF
	accountInfo, _, _, _, isBanve := helper.GetInfoAccountFromToken(c)
	if !isBanve {
		return utils.ErrorResponse(c, 403, "FORBIDDEN", nil)
	}

	heldBy := fmt.Sprintf("STAFF_%d", accountInfo.AccountId)

	tx := db.Begin()

	// Tìm suất chiếu
	var showtime model.Showtime
	if err := tx.Preload("Movie").Where("public_code = ?", code).First(&showtime).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, 404, "Suất chiếu không tồn tại", err)
	}
	var heldSeats []model.ShowtimeSeat
	if err := tx.Where("showtime_id = ? AND seat_id IN ? AND status = ? AND held_by = ?",
		showtime.ID, input.SeatIds, "HELD", heldBy).
		Preload("Seat.SeatType").
		Preload("Showtime").
		Preload("Seat").Find(&heldSeats).Error; err != nil || len(heldSeats) != len(input.SeatIds) {
		tx.Rollback()
		return utils.ErrorResponse(c, 400, "Một số ghế không hợp lệ hoặc đã hết hạn giữ chỗ", nil)
	}
	totalAmount := CalculateTotalAmount(heldSeats)
	now := time.Now()
	order := model.Order{
		PublicCode:    "ORD-" + uuid.New().String()[:8],
		CustomerName:  input.CustomerName,
		Phone:         input.Phone,
		Email:         input.Email,
		PaymentMethod: input.PaymentMethod,
		Status:        "PAID",
		PaidAt:        &now,
		CreatedBy:     accountInfo.AccountId,
		ShowtimeID:    showtime.ID,
		TotalAmount:   totalAmount, // sẽ tính sau
	}
	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, 500, "Không thể tạo đơn hàng", err)
	}
	// Kiểm tra và tạo vé cho từng ghế
	var tickets []model.Ticket
	for _, seatId := range input.SeatIds {
		var stSeat model.ShowtimeSeat
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("seat_id = ? AND showtime_id = ? AND status = ? AND held_by = ?",
				seatId, showtime.ID, SeatHeld, heldBy).
			First(&stSeat).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 400, fmt.Sprintf("Ghế %d không được giữ bởi bạn", seatId), err)
		}
		ticketCode := "TKT-" + uuid.New().String()[:10]
		// Tạo vé
		ticket := model.Ticket{
			OrderId:        order.ID, // ← QUAN TRỌNG: liên kết với Order
			ShowtimeId:     showtime.ID,
			ShowtimeSeatId: stSeat.ID, // ← liên kết với ghế trong suất
			SeatId:         stSeat.SeatId,
			TicketCode:     ticketCode,
			Price:          float64(showtime.Price) * stSeat.SeatType.PriceModifier,
			Status:         "PAID",
			IssuedAt:       now,
			CreatedBy:      accountInfo.AccountId,
		}
		if err := tx.Create(&ticket).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Không thể tạo vé", err)
		}
		tickets = append(tickets, ticket)

		// Release ghế sau khi tạo vé
		if err := tx.Model(&stSeat).Updates(map[string]any{
			"status":     "SOLD",
			"held_by":    "",
			"expired_at": nil,
		}).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Không thể release ghế sau khi tạo vé", err)
		}
	}

	tx.Commit()

	return utils.SuccessResponse(c, 200, fiber.Map{
		"tickets": tickets,
		"message": "Tạo vé thành công",
	})
}

// API check-in
func CheckinByBookingCode(c *fiber.Ctx) error {
	code := c.Query("code") // hoặc từ body
	db := database.DB
	var tickets []model.Ticket
	if err := db.Where("booking_code = ? AND status = 'PAID'", code).Find(&tickets).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Đơn hàng không tồn tại", err)
	}

	if len(tickets) == 0 {
		return utils.ErrorResponse(c, fiber.StatusNotFound, "Không tìm thấy vé", nil)
	}

	// Kiểm tra đã check-in chưa
	var checkedInCount int64
	db.Model(&model.Ticket{}).Where("booking_code = ? AND status = 'CHECKED_IN'", code).Count(&checkedInCount)
	if checkedInCount > 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Đơn này đã được check-in một phần hoặc toàn bộ", nil)
	}

	// Check-in tất cả
	if err := db.Model(&model.Ticket{}).Where("booking_code = ?", code).
		Updates(map[string]interface{}{
			"status":        "CHECKED_IN",
			"checked_in_at": time.Now(),
		}).Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Check-in thất bại", nil)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"message":       fmt.Sprintf("Check-in thành công %d vé", len(tickets)),
		"customer_name": tickets[0].Order.CustomerName,
		"movie":         tickets[0].Showtime.Movie.Title,
		"ticketCode":    tickets[0].TicketCode,
		"showtime":      tickets[0].Showtime.StartTime,
	})
}
func CheckinByOrderCode(c *fiber.Ctx) error {
	type CheckinInput struct {
		Code string `json:"code" validate:"required"`
	}

	var input CheckinInput
	if err := c.BodyParser(&input); err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Dữ liệu không hợp lệ", err)
	}

	db := database.DB
	accountInfo, _, _, _, isBanve := helper.GetInfoAccountFromToken(c)
	if !isBanve {
		return utils.ErrorResponse(c, 403, "FORBIDDEN", nil)
	}
	// 1️⃣ Lấy order + tickets + seat + showtime + movie
	var order model.Order
	err := db.
		Preload("Tickets.ShowtimeSeat.Seat").
		Preload("Tickets.Showtime.Movie").
		Preload("Tickets.Showtime.Room").
		Where("public_code = ?", input.Code).
		First(&order).Error

	if err != nil {
		return utils.ErrorResponse(
			c,
			fiber.StatusNotFound,
			"Đơn hàng không tồn tại hoặc mã không hợp lệ",
			err,
		)
	}

	if len(order.Tickets) == 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Đơn hàng không có vé", nil)
	}

	showtime := order.Tickets[0].Showtime

	// 2️⃣ KIỂM TRA SUẤT CHIẾU ĐÃ KẾT THÚC CHƯA
	// Thời gian kết thúc ước tính = start_time + duration (phút) + 15 phút dọn phòng
	endTime := showtime.StartTime.Add(time.Duration(showtime.Movie.Duration+15) * time.Minute)

	if time.Now().After(endTime) {
		return utils.ErrorResponse(
			c,
			fiber.StatusForbidden,
			fmt.Sprintf(
				"Suất chiếu đã kết thúc từ %s. Không thể check-in sau khi phim hết.",
				endTime.Format("15:04"),
			),
			nil,
		)
	}

	// 3️⃣ Kiểm tra vé đã check-in chưa
	for _, ticket := range order.Tickets {
		if ticket.Status == "CHECKED_IN" {
			return utils.ErrorResponse(
				c,
				fiber.StatusBadRequest,
				"Đơn hàng đã được check-in trước đó",
				nil,
			)
		}
	}

	// 4️⃣ Check-in tất cả vé (transaction)
	now := time.Now()

	err = db.Transaction(func(tx *gorm.DB) error {
		return tx.Model(&model.Ticket{}).
			Where("order_id = ?", order.ID).
			Updates(map[string]interface{}{
				"status":        "CHECKED_IN",
				"used_at":       &now,
				"checked_in_by": accountInfo.AccountId,
			}).Error
	})

	if err != nil {
		return utils.ErrorResponse(
			c,
			fiber.StatusInternalServerError,
			"Check-in thất bại",
			err,
		)
	}

	// 5️⃣ Build danh sách ghế
	seats := make([]string, 0, len(order.Tickets))
	for _, ticket := range order.Tickets {
		seat := ticket.ShowtimeSeat.Seat
		if seat.ID != 0 {
			seats = append(seats, fmt.Sprintf("%s%d", seat.Row, seat.Column))
		}
	}

	// 6️⃣ Trả response
	return utils.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"message":   fmt.Sprintf("Check-in thành công %d vé!", len(order.Tickets)),
		"orderCode": order.PublicCode,

		"movie":         showtime.Movie.Title,
		"showtime":      showtime.StartTime.Format("15:04 - 02/01/2006"),
		"seats":         strings.Join(seats, ", "),
		"checked_in_at": now.Format("15:04:05"),
		"room":          showtime.Room.Name,
		"ticketCount":   len(order.Tickets),
	})
}
