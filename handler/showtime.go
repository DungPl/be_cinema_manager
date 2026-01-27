package handler

import (
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

func GetShowtime(c *fiber.Ctx) error {
	filterInput := new(model.FilterShowtimeInput)
	if err := c.QueryParser(filterInput); err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_INPUT, err)
	}
	accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)

	if !isAdmin && !isManager {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền "))
	}
	db := database.DB

	condition := db.Model(&model.Showtime{}).
		Joins("JOIN movies ON movies.id = showtimes.movie_id").
		Joins("JOIN rooms ON rooms.id = showtimes.room_id ").
		Joins("JOIN cinemas     ON cinemas.id = rooms.cinema_id").
		Joins("JOIN addresses   ON addresses.cinema_id = cinemas.id")
	// Thời gian hiện tại (múi giờ +07:00, 2025-10-12 17:03:00)
	currentTime := time.Now().In(time.FixedZone("ICT", 7*3600))

	// Áp dụng bộ lọc thời gian thực dựa trên ShowingStatus
	if filterInput.ShowingStatus != "" {
		switch filterInput.ShowingStatus {
		case "UPCOMING":
			condition = condition.Where("showtimes.start_time > ?", currentTime)
		case "ONGOING":
			condition = condition.Where("showtimes.start_time <= ? AND showtimes.end_time >= ?", currentTime, currentTime)
		case "ENDED":
			condition = condition.Where("showtimes.end_time < ?", currentTime)
		}
	}
	// 🔥 Filter theo tỉnh
	if filterInput.Province != "" {
		condition = condition.Where("addresses.province = ?", filterInput.Province)
	}

	// 🔥 Filter theo huyện
	if filterInput.District != "" {
		condition = condition.Where("addresses.district = ?", filterInput.District)
	}

	// 🔥 Filter theo phường
	if filterInput.Ward != "" {
		condition = condition.Where("addresses.ward = ?", filterInput.Ward)
	}

	// Áp dụng các bộ lọc khác
	if filterInput.MovieId > 0 {
		condition = condition.Where("showtimes.movie_id = ?", filterInput.MovieId)
	}
	if filterInput.RoomId > 0 {
		condition = condition.Where("showtimes.room_id = ?", filterInput.RoomId)
	}
	if filterInput.CinemaId > 0 {
		condition = condition.
			Where("cinemas.id = ?", filterInput.CinemaId)
	}
	startDateStr := filterInput.StartDate
	if startDateStr != "" {
		startDate, err := time.Parse("2006-01-02", startDateStr)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid startDate format (use YYYY-MM-DD)", err)
		}

		startOfDay := startDate.Truncate(24 * time.Hour)
		endOfDay := startOfDay.Add(24*time.Hour - time.Second)

		condition = condition.Where("showtimes.start_time BETWEEN ? AND ?", startOfDay, endOfDay)
	}
	endDateStr := filterInput.EndDate
	if endDateStr != "" {
		endDate, err := time.Parse("2006-01-02", endDateStr)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Invalid endDate format (use YYYY-MM-DD)", err)
		}
		startOfDay := endDate.Truncate(24 * time.Hour)
		endOfDay := startOfDay.Add(24*time.Hour - time.Second)
		condition = condition.Where("showtimes.end_time BETWEEN ? AND ?", startOfDay, endOfDay)
	}
	if isManager {
		if accountInfo.CinemaId == nil {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Manager không được gán rạp", nil)
		}
		condition = condition.Where("room_id IN (SELECT id FROM rooms WHERE cinema_id = ?)", *accountInfo.CinemaId)
	}

	// Tính tổng số bản ghi
	var total int64
	if err := condition.Count(&total).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Không thể đếm dữ liệu", err)
	}

	// Pagination
	condition = utils.ApplyPagination(condition, filterInput.Limit, filterInput.Page)

	// Final fetch with preload
	var showtimes []model.Showtime
	if err := condition.
		Preload("Movie").
		Preload("Movie.Formats").
		Preload("Movie.Directors").
		Preload("Movie.Actors").
		Preload("Room").
		Preload("Room.Formats").
		Preload("Room.Cinema").
		Preload("Room.Cinema.Addresses").
		Order("showtimes.start_time ASC").
		Find(&showtimes).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Không thể lấy dữ liệu", err)
	}

	// Response
	var responses []model.ShowtimeResponse

	for _, s := range showtimes {
		var totalSeats int64
		db.Model(&model.ShowtimeSeat{}).
			Where("showtime_id = ?", s.ID).
			Count(&totalSeats)

		var bookedSeats int64
		db.Model(&model.ShowtimeSeat{}).
			Where("showtime_id = ? AND status IN ?",
				s.ID, []string{"SOLD", "BOOKED"}).
			Count(&bookedSeats)

		var actualRevenue float64
		db.Model(&model.Order{}).
			Select("COALESCE(SUM(orders.actual_revenue), 0)").
			Where("orders.showtime_id = ?", s.ID).
			Scan(&actualRevenue)
		var bookedRevenue float64
		db.Model(&model.Ticket{}).
			Joins("JOIN orders ON orders.id = tickets.order_id").
			Where("tickets.showtime_id = ?", s.ID).
			Select("COALESCE(SUM(tickets.price), 0)").
			Scan(&bookedRevenue)
		fillRate := 0.0
		if totalSeats > 0 {
			fillRate = float64(bookedSeats) / float64(totalSeats) * 100
		}

		responses = append(responses, model.ShowtimeResponse{
			DTO:            s.DTO,
			Price:          s.Price,
			Movie:          s.Movie,
			Room:           s.Room,
			StartTime:      s.StartTime,
			EndTime:        s.EndTime,
			FillRate:       fillRate,
			Format:         s.Format,
			BookedSeats:    bookedSeats,
			TotalSeats:     totalSeats,
			LanguageType:   string(s.LanguageType),
			BookedRevenue:  bookedRevenue, // hoặc tính chính xác từ ticket.price
			ActualRevenue:  actualRevenue,
			RefundedAmount: float64(bookedSeats)*s.Price - actualRevenue,
		})
	}
	if responses == nil {
		responses = []model.ShowtimeResponse{}
	}
	return utils.SuccessResponse(c, fiber.StatusOK, responses)
}

func GetShowtimeForStaff(c *fiber.Ctx) error {
	accountInfo, _, _, _, isBanve := helper.GetInfoAccountFromToken(c)
	if !isBanve {
		return utils.ErrorResponse(c, 403, "FORBIDDEN", nil)
	}

	if accountInfo.CinemaId == nil {
		return utils.ErrorResponse(c, 403, "Nhân viên chưa được gán rạp", nil)
	}

	// 👉 Lấy query filter tên phim
	movieTitle := c.Query("movieTitle")

	db := database.DB

	type StaffShowtimeResponse struct {
		Id         uint      `json:"id"`
		PublicCode string    `json:"publicCode"`
		MovieTitle string    `json:"movieTitle"`
		StartTime  time.Time `json:"startTime"`
		PosterUrl  string    `json:"posterUrl"`
		Price      float64   `json:"price"`
	}

	var result []StaffShowtimeResponse

	query := db.Table("showtimes").
		Select(`
			showtimes.id,
			showtimes.public_code,
			movie_posters.url as poster_url,
			movies.title as movie_title,
			showtimes.price,
			showtimes.start_time
		`).
		Joins("JOIN movie_posters ON movie_posters.movie_id=showtimes.movie_id").
		Joins("JOIN movies ON movies.id = showtimes.movie_id").
		Joins("JOIN rooms  ON rooms.id  = showtimes.room_id").
		Where("rooms.cinema_id = ?", *accountInfo.CinemaId).
		Where("showtimes.start_time >= ?", time.Now())

	// 👉 Nếu có filter tên phim
	if movieTitle != "" {
		query = query.Where("movies.title LIKE ?", "%"+movieTitle+"%")
	}

	err := query.
		Order("showtimes.start_time ASC").
		Scan(&result).Error

	if err != nil {
		return utils.ErrorResponse(c, 500, "Không thể lấy suất chiếu", err)
	}

	return utils.SuccessResponse(c, 200, result)
}

func GetShowtimeById(c *fiber.Ctx) error {
	db := database.DB
	showtimeId := c.Locals("showtimeId").(uint)
	tx := db.Begin()
	var showtime model.Showtime
	tx.First(&showtime, showtimeId)
	if err := db.Preload("Movie").Preload("Movie.Director").Preload("Movie.Actors").
		Preload("Room").Preload("Room.Cinema").
		Where("id = ? ", showtimeId).First(&showtime).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Lịch chiếu không tồn tại",
		})
	}

	// Tính trạng thái thời gian thực
	currentTime := time.Now().In(time.FixedZone("ICT", 7*3600))
	var status string
	switch {
	case showtime.StartTime.After(currentTime):
		status = "UPCOMING"
	case showtime.EndTime.Before(currentTime):
		status = "ENDED"
	default:
		status = "ONGOING"
	}
	response := map[string]interface{}{
		"id":        showtime.ID,
		"movieId":   showtime.MovieId,
		"movie":     showtime.Movie,
		"roomId":    showtime.RoomId,
		"room":      showtime.Room,
		"startTime": showtime.StartTime,
		"endTime":   showtime.EndTime,
		"price":     showtime.Price,
		"status":    status,
		"tickets":   showtime.Tickets,
	}

	return utils.SuccessResponse(c, fiber.StatusOK, response)
}
func GetShowtimeTicket(c *fiber.Ctx) error {
	showtimeId, ok := c.Locals("showtimeId").(int)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy ID lịch chiếu",
		})
	}

	db := database.DB
	var tickets []model.Ticket
	if err := db.Where("showtime_id = ? AND deleted_at IS NULL", showtimeId).
		Preload("Customer").Preload("Seat").Preload("Seat.SeatType").
		Find(&tickets).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể lấy danh sách vé: %s", err.Error()),
		})
	}
	response := &model.ResponseCustom{
		Rows:       tickets,
		TotalCount: int64(len(tickets)),
	}
	return utils.SuccessResponse(c, fiber.StatusOK, response)

}
func GetShowtimeSeat(c *fiber.Ctx) error {
	showtimeId, ok := c.Locals("showtimeId").(int)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy ID lịch chiếu",
		})
	}

	db := database.DB
	var showtime model.Showtime
	if err := db.Preload("Room").Where("id = ? ", showtimeId).First(&showtime).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Lịch chiếu không tồn tại",
		})
	}

	// Lấy tất cả ghế
	var seats []model.Seat
	db.Where("room_id = ?", showtime.RoomId).Preload("SeatType").Find(&seats)

	// Lấy vé BOOKED
	var bookedTickets []model.Ticket
	db.Where("showtime_id = ?", showtimeId).Find(&bookedTickets)

	// Lấy reservation ACTIVE
	var showtimeSeats []model.ShowtimeSeat
	if err := database.DB.
		Where("showtime_id = ?", showtimeId).
		Find(&showtimeSeats).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Không lấy được trạng thái ghế", err)
	}

	seatStatusMap := map[uint]model.ShowtimeSeat{}
	for _, s := range showtimeSeats {
		seatStatusMap[s.SeatId] = s
	}

	// Build response
	response := []map[string]interface{}{}

	for _, seat := range seats {
		status := "AVAILABLE"
		expiredAt := ""
		heldBy := ""

		if ss, exists := seatStatusMap[seat.ID]; exists {
			status = ss.Status
			if ss.ExpiredAt.After(time.Now()) {
				expiredAt = ss.ExpiredAt.Format(time.RFC3339)
			}
			heldBy = ss.HeldBy
		}

		response = append(response, map[string]interface{}{
			"id":        seat.ID,
			"row":       seat.Row,
			"number":    seat.Column,
			"type":      seat.SeatType.Type,
			"price":     showtime.Price * seat.SeatType.PriceModifier,
			"status":    status,
			"expiredAt": expiredAt,
			"heldBy":    heldBy,
		})
	}

	return utils.SuccessResponse(c, 200, response)
}
func CreateShowtimeBatch(c *fiber.Ctx) error {

	input := c.Locals("batchInput").(model.CreateShowtimeBatchInput)
	duration := c.Locals("movieDuration").(int)
	startDate := c.Locals("startDate").(time.Time)
	endDate := c.Locals("endDate").(time.Time)

	db := database.DB
	location := time.FixedZone("ICT", 7*3600)
	currentTime := time.Now().In(location)

	var showtimes []model.Showtime
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		for _, roomID := range input.RoomIDs {
			for _, format := range input.Formats {
				for _, slot := range input.TimeSlots {
					startStr := fmt.Sprintf("%s %s", d.Format("2006-01-02"), slot)
					startTime, err := time.ParseInLocation("2006-01-02 15:04", startStr, location)
					if err != nil {
						continue
					}

					// Không tạo lịch quá khứ
					if startTime.Before(currentTime) {
						continue
					}

					endTime := startTime.Add(time.Duration(duration) * time.Minute)
					price := helper.CalculatePrice(startTime, format, d)
					showtimes = append(showtimes, model.Showtime{
						MovieId:   input.MovieID,
						RoomId:    roomID,
						StartTime: startTime,
						EndTime:   endTime,
						Format:    format,
						Price:     price,
						Status:    "scheduled",
					})
				}
			}
		}
	}

	if len(showtimes) == 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Không có lịch chiếu hợp lệ để tạo", nil)
	}

	tx := db.Begin()
	for i, st := range showtimes {
		// Kiểm tra phòng chiếu hoạt động
		var room model.Room
		if err := tx.Where("id = ? AND status = ?", st.RoomId, "active").First(&room).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest,
				fmt.Sprintf("Phòng chiếu không hoạt động (index %d)", i), err, "roomId")
		}

		// Kiểm tra xung đột
		var conflict int64
		if err := tx.Model(&model.Showtime{}).
			Where(`room_id = ? 
					AND ((start_time <= ? AND end_time >= ?) 
					OR (start_time <= ? AND end_time >= ?))`,
				st.RoomId, st.StartTime, st.StartTime, st.EndTime, st.EndTime).
			Count(&conflict).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Lỗi kiểm tra xung đột lịch chiếu", err)
		}
		if conflict > 0 {
			tx.Rollback()
			return utils.ErrorResponseHaveKey(c, 400,
				fmt.Sprintf("Xung đột lịch chiếu (Phòng %d, %s)", st.RoomId, st.StartTime.Format("02/01 15:04")),
				nil, "startTime")
		}

		if err := tx.Create(&st).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Không thể tạo lịch chiếu", err)
		}
	}

	tx.Commit()

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":   fmt.Sprintf("Tạo thành công %d lịch chiếu", len(showtimes)),
		"showtimes": showtimes,
	})

}

type LanguageType string

const (
	LangViSub LanguageType = "VI_SUB"
	LangViDub LanguageType = "VI_DUB"
	LangEnSub LanguageType = "EN_SUB"
	LangEnDub LanguageType = "EN_DUB"
	LangVi    LanguageType = "VI"
)

func AutoGenerateShowtimeSchedule(c *fiber.Ctx) error {
	input := c.Locals("input").(model.AutoGenerateScheduleInput)
	movie := c.Locals("movie").(model.Movie)
	startDate := c.Locals("startDate").(time.Time)
	endDate := c.Locals("endDate").(time.Time)
	useTemplate := c.Locals("useTemplate").(bool)
	var template *model.ScheduleTemplate
	if useTemplate {
		template = c.Locals("template").(*model.ScheduleTemplate)
	}
	if template != nil {
		if len(input.TimeSlots) == 0 {
			input.TimeSlots = template.TimeSlots
		}

		if len(input.Formats) == 0 {
			input.Formats = template.Formats
		}
		if len(input.RoomIDs) > template.MaxRooms {
			input.RoomIDs = input.RoomIDs[:template.MaxRooms]
		}
	}
	db := database.DB
	if len(input.LanguageType) == 0 {
		input.LanguageType = []string{string(LangViSub)}
	}

	// Validate language
	validLang := map[string]bool{
		string(LangViSub): true,
		string(LangViDub): true,
		string(LangEnSub): true,
		string(LangEnDub): true,
	}
	tx := db.Begin()
	if tx.Error != nil {
		return utils.ErrorResponse(c, 500, "Lỗi DB", tx.Error)
	}
	movieFormatsMap := make(map[string]bool)
	for _, f := range movie.Formats {
		movieFormatsMap[f.Name] = true // giả sử Format có field Name (string) như "IMAX", "2D", "3D",...
	}

	unsupported := []string{}
	for _, requestedFormat := range input.Formats {
		if !movieFormatsMap[requestedFormat] {
			unsupported = append(unsupported, requestedFormat)
		}
	}

	if len(unsupported) > 0 {
		tx.Rollback()
		msg := fmt.Sprintf("Phim không hỗ trợ định dạng: %s", strings.Join(unsupported, ", "))
		return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest,
			msg,
			fmt.Errorf("unsupported formats: %v", unsupported),
			"formats")
	}
	const (
		MinGapBetweenRooms = 10 * time.Minute // cách nhau tối thiểu giữa hai phòng chiếu cùng phim
		BreakStartHour     = 11
		BreakEndHour       = 14
		ExtraBreakTime     = 10 * time.Minute
		MaxShowtimesPerDay = 8
	)
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	createdCount := 0
	skippedCount := 0

	// Duyệt từng ngày
	// Thêm map để đếm suất IMAX muộn per day (nếu cần "không quá 1h" nghĩa là không quá 1 suất sau 22:00)
	imaxLateCountPerDay := make(map[string]int) // key: "2006-01-02"
	for currentDate := startDate; !currentDate.After(endDate); currentDate = currentDate.AddDate(0, 0, 1) {
		dateKey := currentDate.Format("2006-01-02")
		imaxLateCountPerDay[dateKey] = 0 // reset nếu cần, nhưng map sẽ tự tạo

		for _, roomID := range input.RoomIDs {

			var room model.Room
			if err := tx.Preload("Formats").First(&room, roomID).Error; err != nil {
				tx.Rollback()
				return utils.ErrorResponse(c, 404, "Phòng không tồn tại", err)
			}
			if room.Status != "available" {
				tx.Rollback()
				return utils.ErrorResponse(c, 400, "Phòng đang bị khóa", nil)
			}
			if len(input.RoomIDs) > 6 {
				return utils.ErrorResponse(c, 400, "Tối đa 6 phòng cho 1 phim", nil)
			}
			// Lọc định dạng phòng hỗ trợ
			dailyCreatedByFormat := make(map[string]int) // format to count

			for _, format := range input.Formats {

				if !helper.RoomSupportsFormat(room.Formats, format) {
					skippedCount++
					continue
				}

				maxPerDay := helper.GetMaxPerDay(format, template)
				config := helper.GetFormatConfig(format, template)

				// Kiểm tra số phòng (chỉ 1-2 phòng cho phim đặc biệt)
				if config.MaxPerDay <= 4 && len(input.RoomIDs) > 2 {
					return utils.ErrorResponse(c, 400, fmt.Sprintf("Định dạng %s chỉ được chiếu tối đa ở 2 phòng", format), nil)
				}
				if dailyCreatedByFormat[format] >= maxPerDay {
					continue // đã đủ suất cho format này
				}

				//specialFormats := map[string]bool{"4DX": true, "IMAX": true}
				offset := time.Duration(config.OffsetMinutes) * time.Minute

				validSlots := helper.FilterSlotsByFormatAndTime(input.TimeSlots, format, movie.Duration, currentDate, loc)

				for _, slot := range validSlots {
					if dailyCreatedByFormat[format] >= maxPerDay {
						break
					}

					// Tạo startTime
					startStr := fmt.Sprintf("%s %s", currentDate.Format("2006-01-02"), slot)
					startTime, err := time.ParseInLocation("2006-01-02 15:04", startStr, loc)
					if err != nil {
						continue
					}

					// Tránh giờ nghỉ trưa
					// if startTime.Hour() >= BreakStartHour && startTime.Hour() < BreakEndHour {
					// 	startTime = time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), BreakEndHour, 0, 0, 0, loc).Add(ExtraBreakTime)
					// }

					// Offset theo thứ tự phòng
					idx := helper.IndexInArray(input.RoomIDs, roomID)
					if idx < 0 {
						skippedCount++
						continue
					}
					startTime = startTime.Add(time.Duration(idx) * offset)

					if startTime.Day() != currentDate.Day() {
						skippedCount++
						continue
					}

					endTime := startTime.Add(time.Minute * time.Duration(movie.Duration))

					// IMAX: chỉ 1 suất sau 22:00/ngày
					if format == "IMAX" && startTime.Hour() >= 22 {
						if imaxLateCountPerDay[dateKey] >= 1 {
							skippedCount++
							continue
						}
						imaxLateCountPerDay[dateKey]++
					}

					// Kiểm tra trùng phòng
					if helper.HasConflict(tx, roomID, startTime, endTime) {
						skippedCount++
						continue
					}

					// Kiểm tra khoảng cách giữa các phòng
					if helper.HasNearbyConflict(tx, room.CinemaId, movie.ID, roomID, startTime, MinGapBetweenRooms) {
						skippedCount++
						continue
					}

					// Tính giá

					if len(input.LanguageType) == 0 {
						input.LanguageType = []string{"VI_SUB"}
					}
					for _, lang := range input.LanguageType {

						if !validLang[lang] {
							continue
						}
						var price float64
						if input.Price > 0 {
							price = float64(input.Price) // frontend override
						} else {
							price = helper.CalculateDynamicPrice(startTime, format, currentDate, input.IsVietnamese)
						}
						showtime := model.Showtime{
							PublicCode:   "ST-" + utils.RandomString(6),
							MovieId:      input.MovieID,
							RoomId:       roomID,
							StartTime:    startTime,
							EndTime:      endTime,
							LanguageType: model.LanguageType(lang),
							Format:       format,
							Price:        price,
							Status:       "AVAILABLE",
						}

						if err := tx.Create(&showtime).Error; err != nil {
							tx.Rollback()
							return utils.ErrorResponse(c, 500, "Lỗi tạo suất chiếu", err)
						}
						if err := helper.CreateShowtimeSeats(tx, showtime.ID, roomID); err != nil {
							return utils.ErrorResponse(c, 500, "Không tạo được danh sách ghế", err)
						}

						createdCount++
					}
				}
				dailyCreatedByFormat[format]++
			}

		}

	}

	if err := tx.Commit().Error; err != nil {
		return utils.ErrorResponse(c, 500, "Lỗi commit", err)
	}

	return utils.SuccessResponse(c, 201, fiber.Map{
		"message":    "Tạo lịch khung thành công",
		"created":    createdCount,
		"skipped":    skippedCount,
		"totalDays":  int(endDate.Sub(startDate).Hours()/24) + 1,
		"totalRooms": len(input.RoomIDs),
	})

}
func AutoSchedule(c *fiber.Ctx) error {
	template := c.Locals("template").(struct {
		ID      uint
		MovieID uint
		Items   []struct {
			RoomID uint   `gorm:"column:room_id"`
			Time   string `gorm:"column:time"`
			Format string `gorm:"column:format"`
		}
	})
	duration := c.Locals("movieDuration").(int)
	applyDates := c.Locals("applyDates").([]string)

	location := time.FixedZone("ICT", 7*3600)
	var showtimes []model.Showtime
	conflicts := []string{}

	// === TẠO KẾ HOẠCH ===
	for _, dateStr := range applyDates {
		date, _ := time.Parse("2006-01-02", dateStr)
		for _, item := range template.Items {
			timeStr := fmt.Sprintf("%s %s", date.Format("2006-01-02"), item.Time)
			startTime, err := time.ParseInLocation("2006-01-02 15:04", timeStr, location)
			if err != nil {
				continue
			}

			endTime := startTime.Add(time.Duration(duration) * time.Minute)
			price := helper.CalculatePrice(startTime, item.Format, date)

			showtimes = append(showtimes, model.Showtime{
				MovieId:   template.MovieID,
				RoomId:    item.RoomID,
				StartTime: startTime,
				EndTime:   endTime,
				Format:    item.Format,
				Price:     price,
				Status:    "scheduled",
			})
		}
	}

	if len(showtimes) == 0 {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Không tạo được suất nào", nil)
	}

	// === KIỂM TRA XUNG ĐỘT ===
	tx := database.DB.Begin()
	for _, st := range showtimes {
		var count int64
		tx.Model(&model.Showtime{}).
			Where("room_id = ? AND status != 'cancelled'", st.RoomId).
			Where("(start_time < ? AND end_time > ?) OR (start_time < ? AND end_time > ?)",
				st.EndTime, st.StartTime, st.EndTime, st.StartTime).
			Count(&count)
		if count > 0 {
			conflicts = append(conflicts, fmt.Sprintf("Phòng %d: %s", st.RoomId, st.StartTime.Format("02/01 15:04")))
		}
	}

	if len(conflicts) > 0 {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusConflict,
			fmt.Sprintf("Xung đột %d suất: %s", len(conflicts), strings.Join(conflicts[:3], ", ")), nil)
	}

	// === TẠO HÀNG LOẠT ===
	if err := tx.Create(&showtimes).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi tạo suất chiếu", err)
	}

	tx.Commit()

	return utils.SuccessResponse(c, fiber.StatusCreated, fiber.Map{
		"message": fmt.Sprintf("Tạo thành công %d suất chiếu", len(showtimes)),
		"days":    len(applyDates),
		"perDay":  len(template.Items),
		"total":   len(showtimes),
	})

}
func EditShowtime(c *fiber.Ctx) error {
	db := database.DB
	showtimeId := c.Locals("showtimeId").(uint)
	showtimeInput, ok := c.Locals("inputEditShowtime").(model.UpdateShowtimeInput)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}

	tx := db.Begin()
	var showtime model.Showtime
	if err := tx.First(&showtime, showtimeId).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Showtime not found", err)
	}
	//log.Printf("EditShowtime input: %+v", showtimeInput)
	//log.Printf("EditShowtime handler - input: %+v", showtimeInput)
	if showtimeInput.MovieId != nil {
		//log.Printf("Updating movieId từ %d thành %d", showtime.MovieId, *showtimeInput.MovieId)
		showtime.MovieId = *showtimeInput.MovieId
		var movie model.Movie
		if err := tx.First(&movie, *showtimeInput.MovieId).Error; err == nil {
			showtime.Movie = movie
		}
	}
	if showtimeInput.RoomId != nil {
		showtime.RoomId = *showtimeInput.RoomId
	}
	if showtimeInput.StartTime != nil {
		showtime.StartTime = *showtimeInput.StartTime
		if showtime.Movie.ID != 0 {
			showtime.EndTime = showtime.StartTime.Add(
				time.Duration(showtime.Movie.Duration) * time.Minute,
			)
		}
	}

	if showtimeInput.Price != nil {
		showtime.Price = *showtimeInput.Price
	}
	showtime.PublicCode = "ST-" + utils.RandomString(6)
	if err := tx.Save(&showtime).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_EDIT, err)
	}
	tx.Commit()
	return utils.SuccessResponse(c, fiber.StatusOK, showtime)
}
func DeleteShowtime(c *fiber.Ctx) error {
	showtimeId, ok := c.Locals("showtimeId").(int)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy ID lịch chiếu",
		})
	}

	db := database.DB
	tx := db.Begin()
	// Xóa mềm lịch chiếu
	if err := db.Where("showtime_id = ?", showtimeId).Delete(&model.ShowtimeSeat{}).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Không xóa được ghế của suất chiếu", err)
	}
	if err := tx.Where("id = ?", showtimeId).Delete(&model.Showtime{}).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể xóa lịch chiếu: %s", err.Error()),
		})
	}

	// Commit giao dịch
	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể commit giao dịch: %s", err.Error()),
		})
	}
	return utils.SuccessResponse(c, fiber.StatusOK, showtimeId)
}
func GetShowtimeByCinemaIdAndDate(c *fiber.Ctx) error {
	cinemaIdStr := c.Params("cinemaId")
	dateStr := c.Query("date")

	if cinemaIdStr == "" || dateStr == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "cinemaId and date required",
		})
	}

	cinemaId, err := strconv.ParseUint(cinemaIdStr, 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "cinemaId must be a number",
		})
	}

	start, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid date format, use YYYY-MM-DD",
		})
	}
	end := start.Add(24 * time.Hour)

	var showtimes []model.Showtime
	db := database.DB
	err = db.Preload("Room").Preload("Movie").
		Joins("JOIN rooms ON rooms.id = showtimes.room_id").
		Where("rooms.cinema_id = ?", cinemaId).
		Where("showtimes.start_time >= ? AND showtimes.start_time < ?", start, end).
		Find(&showtimes).Error

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(showtimes)
}
func GetShowtimeSeats(c *fiber.Ctx) error {
	showtimeId := c.Params("showtimeId")

	var seats []model.ShowtimeSeat

	err := database.DB.
		Where("showtime_id = ?", showtimeId).
		Find(&seats).Error

	if err != nil {
		return utils.ErrorResponse(c, 500, "Không lấy được ghế", err)
	}

	return utils.SuccessResponse(c, 200, seats)
}
func GetSeatsByShowtime(c *fiber.Ctx) error {
	code := c.Params("code")
	db := database.DB

	// 1️⃣ Lấy showtime (chỉ cần ID và Price để tính tiền nếu cần)
	var showtime struct {
		ID    uint
		Price float64 // nếu bạn dùng để tính tổng tiền ở frontend
	}
	if err := db.Model(&model.Showtime{}).
		Select("id, price").
		Where("public_code = ?", code).
		First(&showtime).Error; err != nil {
		return utils.ErrorResponse(c, 404, "Suất chiếu không tồn tại", err)
	}

	now := time.Now()

	// 2️⃣ AUTO RELEASE ghế hết hạn
	released := db.Model(&model.ShowtimeSeat{}).
		Where("showtime_id = ? AND expired_at < ? AND status = ?", showtime.ID, now, SeatHeld).
		Updates(map[string]any{
			"status":     SeatAvailable,
			"held_by":    "",
			"expired_at": nil,
		})

	if released.RowsAffected > 0 {
		BroadcastShowtime(showtime.ID)
	}

	// 3️⃣ Query tối ưu: chỉ lấy các field cần thiết, dùng JOIN thông minh
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

	var rawSeats []struct {
		ShowtimeSeatID uint
		SeatID         uint
		Row            string
		Column         int
		Status         string
		HeldBy         string
		ExpiredAt      *time.Time
		SeatType       string
		PriceModifier  float64
		CoupleId       *uint
	}

	err := db.Table("showtime_seats ss").
		Select(`
			ss.id as showtime_seat_id,
			ss.seat_id,
			s.row,
			s.column,
			ss.status,
			ss.held_by,
			ss.expired_at,
			st.type as seat_type,
			st.price_modifier,
			s.couple_id
		`).
		Joins("JOIN seats s ON s.id = ss.seat_id").
		Joins("JOIN seat_types st ON st.id = s.seat_type_id").
		Where("ss.showtime_id = ?", showtime.ID).
		Order("ss.seat_id ASC").
		Scan(&rawSeats).Error

	if err != nil {
		return utils.ErrorResponse(c, 500, "Không thể load ghế", err)
	}

	// 4️⃣ Group theo hàng ở Go (nhanh, gọn)
	result := make(map[string][]SeatUI)

	for _, rs := range rawSeats {
		row := rs.Row
		result[row] = append(result[row], SeatUI{
			Id:            rs.SeatID, // frontend dùng seat_id thật
			Label:         fmt.Sprintf("%s%d", rs.Row, rs.Column),
			Type:          rs.SeatType,
			Status:        rs.Status,
			HeldBy:        rs.HeldBy,
			ExpiredAt:     rs.ExpiredAt,
			CoupleId:      rs.CoupleId,
			PriceModifier: rs.PriceModifier,
		})
	}

	return utils.SuccessResponse(c, 200, result)
}

func FetchShowtimeSeats(showtimeId uint) ([]model.ShowtimeSeat, error) {
	var seats []model.ShowtimeSeat

	err := database.DB.
		Where("showtime_id = ?", showtimeId).
		Find(&seats).Error

	return seats, err
}

type LocationResponse struct {
	Province    string       `json:"province"`
	CinemaCount int          `json:"cinemaCount"`
	Chains      []ChainCount `json:"chains"`
}

type ChainCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func GetCinemaLocations(c *fiber.Ctx) error {
	db := database.DB

	// Lấy danh sách tỉnh unique từ addresses, với cinemas active
	var provinces []string
	if err := db.Model(&model.Address{}).
		Joins("JOIN cinemas ON cinemas.id = addresses.cinema_id").
		Where("cinemas.active = ?", true).
		Distinct("province").
		Pluck("province", &provinces).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Cannot fetch provinces", err)
	}

	// Chuẩn bị response
	locations := make([]LocationResponse, 0, len(provinces))

	for _, prov := range provinces {
		// Đếm số rạp trong tỉnh
		var cinemaCount int64
		if err := db.Model(&model.Cinema{}).
			Joins("JOIN addresses ON addresses.cinema_id = cinemas.id").
			Where("cinemas.active = ? AND addresses.province = ?", true, prov).
			Count(&cinemaCount).Error; err != nil {
			return utils.ErrorResponse(c, 500, "Cannot count cinemas", err)
		}

		// Lấy chains và count trong tỉnh
		var chains []ChainCount
		if err := db.Model(&model.Cinema{}).
			Select("cinema_chains.name, COUNT(cinemas.id) as count").
			Joins("JOIN cinema_chains ON cinema_chains.id = cinemas.chain_id").
			Joins("JOIN addresses ON addresses.cinema_id = cinemas.id").
			Where("cinemas.active = ? AND addresses.province = ?", true, prov).
			Group("cinema_chains.name").
			Scan(&chains).Error; err != nil {
			return utils.ErrorResponse(c, 500, "Cannot fetch chains", err)
		}

		locations = append(locations, LocationResponse{
			Province:    prov,
			CinemaCount: int(cinemaCount),
			Chains:      chains,
		})
	}

	// Sắp xếp theo số rạp giảm dần hoặc theo tên
	// Ở đây giả sử sắp theo số rạp
	sort.Slice(locations, func(i, j int) bool {
		return locations[i].CinemaCount > locations[j].CinemaCount
	})

	return utils.SuccessResponse(c, 200, locations)
}
func GetShowtimeByPublicCode(c *fiber.Ctx) error {
	code := c.Params("code")

	var showtime model.Showtime
	err := database.DB.
		Preload("Movie").
		Preload("Movie.Posters").
		Preload("Room").
		Preload("Room.Cinema").
		Where("public_code = ?", code).
		First(&showtime).Error

	if err != nil {
		return utils.ErrorResponse(c, 404, "Không tìm thấy suất chiếu", nil)
	}

	return utils.SuccessResponse(c, 200, showtime)
}

type ShowtimeGroupResponse struct {
	Movie   model.Movie              `json:"movie"`
	Cinemas []CinemaShowtimeResponse `json:"cinemas"`
}

type CinemaShowtimeResponse struct {
	Cinema  model.Cinema                `json:"cinema"`
	Formats map[string][]model.Showtime `json:"formats"`
}

func GetShowtimesByMovieAndProvince(c *fiber.Ctx) error {
	// ===== Params =====
	movieIDStr := c.Query("movie_id")
	province := c.Query("province")
	format := c.Query("format")
	chainIDStr := c.Query("chain_id")
	dateStr := c.Query("date")
	if movieIDStr == "" || province == "" {
		return utils.ErrorResponse(c, 400, "Thiếu movie_id hoặc province", nil)
	}

	movieID64, err := strconv.ParseUint(movieIDStr, 10, 32)
	if err != nil {
		return utils.ErrorResponse(c, 400, "movie_id không hợp lệ", err)
	}
	movieID := uint(movieID64)

	var chainID uint
	if chainIDStr != "" {
		id64, err := strconv.ParseUint(chainIDStr, 10, 32)
		if err != nil {
			return utils.ErrorResponse(c, 400, "chain_id không hợp lệ", err)
		}
		chainID = uint(id64)
	}

	// ===== Time range =====
	var startDate, endDate time.Time

	if dateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return utils.ErrorResponse(c, 400, "date không hợp lệ (YYYY-MM-DD)", err)
		}
		startDate = parsedDate
		endDate = parsedDate.Add(24 * time.Hour)
	} else {
		startDate = time.Now().Truncate(24 * time.Hour)
		endDate = startDate.AddDate(0, 0, 7)
	}

	// ===== Query =====
	query := database.DB.
		Model(&model.Showtime{}).
		Preload("Movie").
		Preload("Movie.Posters").
		Preload("Room").
		Preload("Room.Cinema").
		Preload("Room.Cinema.Chain").
		Preload("Room.Cinema.Addresses").
		Joins("JOIN rooms ON rooms.id = showtimes.room_id").
		Joins("JOIN cinemas ON cinemas.id = rooms.cinema_id").
		Joins("JOIN addresses ON addresses.cinema_id = cinemas.id").
		Where("showtimes.movie_id = ?", movieID).
		Where("showtimes.start_time >= ? AND showtimes.start_time < ?", startDate, endDate).
		Where("addresses.province LIKE ?", province+"%").
		Where("showtimes.status = ?", "AVAILABLE")

	if format != "" {
		query = query.Where("showtimes.format = ?", format)
	}

	if chainID != 0 {
		query = query.Where("cinemas.chain_id = ?", chainID)
	}

	var showtimes []model.Showtime
	if err := query.
		Order("cinemas.chain_id, cinemas.id, showtimes.start_time").
		Find(&showtimes).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Lỗi truy vấn lịch chiếu", err)
	}

	if len(showtimes) == 0 {
		return utils.SuccessResponse(c, 200, fiber.Map{
			"movie":  nil,
			"chains": []any{},
		})
	}

	// ===== GROUP: chain → cinema → showtimes =====
	type CinemaGroup struct {
		Cinema    model.Cinema     `json:"cinema"`
		Showtimes []model.Showtime `json:"showtimes"`
	}

	type ChainGroup struct {
		Chain   model.CinemaChain `json:"chain"`
		Cinemas []CinemaGroup     `json:"cinemas"`
	}

	chainMap := make(map[uint]*ChainGroup)
	cinemaMap := make(map[uint]*CinemaGroup)

	for _, st := range showtimes {
		chain := st.Room.Cinema.Chain
		cinema := st.Room.Cinema

		if _, ok := chainMap[chain.ID]; !ok {
			chainMap[chain.ID] = &ChainGroup{
				Chain:   chain,
				Cinemas: []CinemaGroup{},
			}
		}

		if _, ok := cinemaMap[cinema.ID]; !ok {
			cg := CinemaGroup{
				Cinema:    cinema,
				Showtimes: []model.Showtime{},
			}
			chainMap[chain.ID].Cinemas = append(chainMap[chain.ID].Cinemas, cg)
			cinemaMap[cinema.ID] = &chainMap[chain.ID].Cinemas[len(chainMap[chain.ID].Cinemas)-1]
		}

		cinemaMap[cinema.ID].Showtimes = append(cinemaMap[cinema.ID].Showtimes, st)
	}

	var chains []ChainGroup
	for _, cg := range chainMap {
		chains = append(chains, *cg)
	}

	return utils.SuccessResponse(c, 200, fiber.Map{
		"movie":   showtimes[0].Movie,
		"format":  format,
		"chainId": chainID,
		"chains":  chains,
	})
}
