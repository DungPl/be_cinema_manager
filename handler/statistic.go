package handler

import (
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// handlers/admin_stats.go
func GetAdminStats(c *fiber.Ctx) error {
	_, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isManager {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
	}

	db := database.DB

	type Stats struct {
		Chains   int64 `json:"chains"`
		Cinemas  int64 `json:"cinemas"`
		Rooms    int64 `json:"rooms"`
		Movies   int64 `json:"movies"`
		Customer int64 `json:"customers"`

		TodayRevenue  float64 `json:"todayRevenue"`
		TodayTickets  int64   `json:"todayTickets"`
		UpcomingShows int64   `json:"upcomingShows"`
		RevenueGrowth float64 `json:"revenueGrowth"` // %
		TicketsGrowth float64 `json:"ticketsGrowth"` // %
		ShowsGrowth   float64 `json:"showsGrowth"`   // %
	}

	var stats Stats
	// ĐÚNG: Tạo time object có giờ phút giây cụ thể
	today := time.Now().In(time.Local) // Đảm bảo timezone đúng
	todayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	todayEnd := time.Date(today.Year(), today.Month(), today.Day(), 23, 59, 59, 0, today.Location())

	// === Hôm nay ===
	db.Model(&model.CinemaChain{}).Count(&stats.Chains)
	db.Model(&model.Cinema{}).Count(&stats.Cinemas)
	db.Model(&model.Room{}).Count(&stats.Rooms)
	db.Model(&model.Movie{}).Count(&stats.Movies)
	// Doanh thu hôm nay
	db.Raw(`
        SELECT COALESCE(SUM(price), 0)
        FROM tickets
        WHERE status = 'BOOKED'
          AND created_at BETWEEN ? AND ?
    `, todayStart, todayEnd).Scan(&stats.TodayRevenue)

	// Số vé hôm nay
	db.Raw(`
        SELECT COUNT(*) 
        FROM tickets t
        WHERE t.status = 'BOOKED'
          AND t.created_at BETWEEN ? AND ?
    `, todayStart, todayEnd).Scan(&stats.TodayTickets)

	db.Model(&model.Showtime{}).
		Where("start_time > ? AND start_time < ?", time.Now(), time.Now().Add(24*time.Hour)).
		Count(&stats.UpcomingShows)

	// === Hôm qua ===
	yesterdayStart := todayStart.AddDate(0, 0, -1)
	yesterdayEnd := todayEnd.AddDate(0, 0, -1)

	var yesterdayRevenue float64
	var yesterdayTickets int64
	var yesterdayShows int64

	db.Raw(`
        SELECT COALESCE(SUM(price), 0)
        FROM tickets
        WHERE status = 'BOOKED'
          AND created_at BETWEEN ? AND ?
    `, yesterdayStart, yesterdayEnd).Scan(&yesterdayRevenue)

	// Số vé hôm qua
	db.Raw(`
        SELECT COUNT(*) 
        FROM tickets t
        WHERE t.status = 'BOOKED'
          AND t.created_at BETWEEN ? AND ?
    `, yesterdayStart, yesterdayEnd).Scan(&yesterdayTickets)
	db.Model(&model.Showtime{}).
		Where("start_time > ? AND start_time < ?", yesterdayStart, yesterdayEnd).
		Count(&yesterdayShows)

	// === Tính % tăng trưởng ===
	stats.RevenueGrowth = utils.CalculateGrowth(stats.TodayRevenue, yesterdayRevenue)
	stats.TicketsGrowth = utils.CalculateGrowth(float64(stats.TodayTickets), float64(yesterdayTickets))
	stats.ShowsGrowth = utils.CalculateGrowth(float64(stats.UpcomingShows), float64(yesterdayShows))
	return utils.SuccessResponse(c, fiber.StatusOK, stats)
}
func GetAdminStatsV2(c *fiber.Ctx) error {
	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "NOT_ADMIN", errors.New("not admin"))
	}

	db := database.DB

	// ---------------------------------------------
	// 1. ADMIN OVERVIEW (Chains, Cinemas, Rooms...)
	// ---------------------------------------------
	var adminCounts struct {
		Chains    int64
		Cinemas   int64
		Rooms     int64
		Movies    int64
		Customers int64
	}

	db.Model(&model.CinemaChain{}).Where("active = ?", true).Count(&adminCounts.Chains)
	db.Model(&model.Cinema{}).Where("active = ?", true).Count(&adminCounts.Cinemas)
	db.Model(&model.Room{}).Where("active = ?", true).Count(&adminCounts.Rooms)
	db.Model(&model.Movie{}).Where("active = ?", true).Count(&adminCounts.Movies)
	db.Model(&model.Customer{}).Count(&adminCounts.Customers)

	// ---------------------------------------------
	// 2. DOANH THU (Today – Week – Month – Total)
	// ---------------------------------------------
	var revenue struct {
		Today float64
		Week  float64
		Month float64
		Total float64
	}

	db.Raw(`
    SELECT
        COALESCE(SUM(CASE WHEN DATE(booking_time) = CURDATE() THEN price END), 0) AS today,
        COALESCE(SUM(CASE WHEN YEARWEEK(booking_time) = YEARWEEK(CURDATE()) THEN price END), 0) AS week,
        COALESCE(SUM(CASE WHEN MONTH(booking_time) = MONTH(CURDATE()) THEN price END), 0) AS month,
        COALESCE(SUM(price), 0) AS total
    FROM tickets
    WHERE status = 'COMPLETED'
	`).Scan(&revenue)

	// ---------------------------------------------
	// 3. TICKET COUNTS (Today – Week – Month)
	// ---------------------------------------------
	var ticketStats struct {
		Today int64
		Week  int64
		Month int64
	}

	db.Raw(`
    SELECT
        COALESCE(SUM(CASE WHEN DATE(booking_time) = CURDATE() THEN 1 END), 0) AS today,
        COALESCE(SUM(CASE WHEN YEARWEEK(booking_time) = YEARWEEK(CURDATE()) THEN 1 END), 0) AS week,
        COALESCE(SUM(CASE WHEN MONTH(booking_time) = MONTH(CURDATE()) THEN 1 END), 0) AS month
    FROM tickets
    WHERE status = 'COMPLETED'
	`).Scan(&ticketStats)

	// ---------------------------------------------
	// 4. PRODUCER STATS (Top Movies By Revenue)
	// ---------------------------------------------
	type TopMovie struct {
		Title   string  `json:"title"`
		Tickets int64   `json:"tickets"`
		Revenue float64 `json:"revenue"`
	}

	var topMovies []TopMovie

	db.Raw(`
    SELECT 
        m.title,
        COUNT(t.id) AS tickets,
        SUM(t.price) AS revenue
    FROM tickets t
    JOIN showtimes s ON s.id = t.showtime_id
    JOIN movies m ON m.id = s.movie_id
    WHERE MONTH(t.booking_time) = MONTH(CURDATE())
    GROUP BY m.id
    ORDER BY revenue DESC
    LIMIT 5
 	`).Scan(&topMovies)

	// ---------------------------------------------
	// 5. CUSTOMER STATS (Hot Movies, Upcoming Showtimes)
	// ---------------------------------------------

	// Hot movies this week
	var hotMovies []struct {
		Title   string `json:"title"`
		Tickets int64  `json:"tickets"`
	}

	db.Raw(`
    SELECT m.title, COUNT(*) AS tickets
    FROM tickets t
    JOIN showtimes s ON s.id = t.showtime_id
    JOIN movies m ON m.id = s.movie_id
    WHERE YEARWEEK(t.booking_time) = YEARWEEK(CURDATE())
    GROUP BY m.id
    ORDER BY tickets DESC
    LIMIT 5
	`).Scan(&hotMovies)

	// Showtimes within next 24h
	var upcomingShowtimes []model.Showtime
	db.Where("start_time BETWEEN NOW() AND DATE_ADD(NOW(), INTERVAL 24 HOUR)").
		Order("start_time ASC").
		Find(&upcomingShowtimes)

	// ---------------------------------------------
	// 6. RESPONSE JSON FORMAT
	// ---------------------------------------------
	response := fiber.Map{
		"admin": fiber.Map{
			"chains":    adminCounts.Chains,
			"cinemas":   adminCounts.Cinemas,
			"rooms":     adminCounts.Rooms,
			"movies":    adminCounts.Movies,
			"customers": adminCounts.Customers,
			"revenue": fiber.Map{
				"today": revenue.Today,
				"week":  revenue.Week,
				"month": revenue.Month,
				"total": revenue.Total,
			},
			"tickets": fiber.Map{
				"today": ticketStats.Today,
				"week":  ticketStats.Week,
				"month": ticketStats.Month,
			},
		},
		"producer": fiber.Map{
			"topMovies": topMovies,
		},
		"customer": fiber.Map{
			"hotMovies":         hotMovies,
			"upcomingShowtimes": upcomingShowtimes,
		},
	}

	return utils.SuccessResponse(c, fiber.StatusOK, response)
}
func GetAdminStatsV3(c *fiber.Ctx) error {
	_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
	}

	db := database.DB

	type MovieRevenue struct {
		MovieID uint    `json:"movieId"`
		Title   string  `json:"title"`
		Revenue float64 `json:"revenue"`
	}

	var stats struct {
		TodayRevenue     float64 `json:"todayRevenue"`
		YesterdayRevenue float64 `json:"yesterdayRevenue"`
		ThisWeekRevenue  float64 `json:"thisWeekRevenue"`
		ThisMonthRevenue float64 `json:"thisMonthRevenue"`

		TodayTickets     int64 `json:"todayTickets"`
		YesterdayTickets int64 `json:"yesterdayTickets"`

		UpcomingShows int64 `json:"upcomingShows"`

		TopMovies []MovieRevenue `json:"topMovies"`

		RoomFullRate      float64 `json:"roomFullRate"`
		LowAttendanceRate float64 `json:"lowAttendanceRate"`
	}

	// Time ranges
	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.Add(-24 * time.Hour).Format("2006-01-02")

	thisWeekStart := now.AddDate(0, 0, -int(now.Weekday())).Format("2006-01-02")
	thisMonthStart := fmt.Sprintf("%d-%02d-01", now.Year(), now.Month())

	// ===============================
	// 1) Doanh thu
	// ===============================
	db.Raw(`
        SELECT COALESCE(SUM(total_price), 0)
        FROM tickets
        WHERE created_at BETWEEN ? AND ?
    `, today+" 00:00:00", today+" 23:59:59").Scan(&stats.TodayRevenue)

	db.Raw(`
        SELECT COALESCE(SUM(total_price), 0)
        FROM tickets
        WHERE created_at BETWEEN ? AND ?
    `, yesterday+" 00:00:00", yesterday+" 23:59:59").Scan(&stats.YesterdayRevenue)

	db.Raw(`
        SELECT COALESCE(SUM(total_price), 0)
        FROM tickets
        WHERE created_at >= ?
    `, thisWeekStart).Scan(&stats.ThisWeekRevenue)

	db.Raw(`
        SELECT COALESCE(SUM(total_price), 0)
        FROM tickets
        WHERE created_at >= ?
    `, thisMonthStart).Scan(&stats.ThisMonthRevenue)

	// ===============================
	// 2) Vé bán
	// ===============================
	db.Raw(`
        SELECT COALESCE(SUM(quantity), 0)
        FROM tickets
        WHERE created_at BETWEEN ? AND ?
    `, today+" 00:00:00", today+" 23:59:59").Scan(&stats.TodayTickets)

	db.Raw(`
        SELECT COALESCE(SUM(quantity), 0)
        FROM tickets
        WHERE created_at BETWEEN ? AND ?
    `, yesterday+" 00:00:00", yesterday+" 23:59:59").Scan(&stats.YesterdayTickets)

	// ===============================
	// 3) Suất chiếu sắp tới
	// ===============================
	db.Model(&model.Showtime{}).
		Where("start_time > ?", now).
		Where("start_time < ?", now.Add(3*time.Hour)).
		Count(&stats.UpcomingShows)

	// ===============================
	// 4) Top 5 phim doanh thu cao nhất
	// ===============================
	db.Raw(`
        SELECT 
            m.id AS movie_id,
            m.title,
            COALESCE(SUM(t.total_price), 0) AS revenue
        FROM tickets t
        JOIN showtimes s ON t.showtime_id = s.id
        JOIN movies m ON s.movie_id = m.id
        GROUP BY m.id, m.title
        ORDER BY revenue DESC
        LIMIT 5
    `).Scan(&stats.TopMovies)

	// ===============================
	// 5) Tỷ lệ lấp đầy phòng
	// ===============================
	var totalSeats float64
	var seatsSold float64

	db.Raw(`
        SELECT COALESCE(SUM(r.seats), 0)
        FROM rooms r
    `).Scan(&totalSeats)

	db.Raw(`
        SELECT COALESCE(SUM(t.quantity), 0)
        FROM tickets t
    `).Scan(&seatsSold)

	if totalSeats > 0 {
		stats.RoomFullRate = (seatsSold / totalSeats) * 100
	}

	// ===============================
	// 6) Suất chiếu ít khách (< 30% seats)
	// ===============================
	var lowTotal int64
	var lowCount int64

	db.Raw(`
        SELECT COUNT(*)
        FROM showtimes
    `).Scan(&lowTotal)

	db.Raw(`
        SELECT COUNT(*)
        FROM showtimes s
        JOIN rooms r ON s.room_id = r.id
        LEFT JOIN (
            SELECT showtime_id, SUM(quantity) AS sold
            FROM tickets
            GROUP BY showtime_id
        ) t ON t.showtime_id = s.id
        WHERE COALESCE(t.sold, 0) < r.seats * 0.3
    `).Scan(&lowCount)

	if lowTotal > 0 {
		stats.LowAttendanceRate = float64(lowCount) * 100 / float64(lowTotal)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, stats)
}
