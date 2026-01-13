package utils

import (
	"math"
	"time"

	"gorm.io/gorm"
)

type NoShowDailyReport struct {
	Date           string  `json:"date"` // "2026-01-04"
	TotalTickets   int     `json:"totalTickets"`
	CheckInTickets int     `json:"checkInTickets"`
	NoShowTickets  int     `json:"noShowTickets"`
	NoShowRate     float64 `json:"noShowRate"`    // %
	EstimatedLoss  float64 `json:"estimatedLoss"` // VND
}

// calculateAverage tính tỷ lệ No-Show trung bình (có trọng số theo số vé bán mỗi ngày)
func CalculateAverage(report []NoShowDailyReport) float64 {
	if len(report) == 0 {
		return 0.0
	}

	var totalTickets int64 = 0 // Tổng vé bán tất cả các ngày
	var totalNoShow int64 = 0  // Tổng vé no-show tất cả các ngày

	for _, r := range report {
		totalTickets += int64(r.TotalTickets)
		totalNoShow += int64(r.NoShowTickets)
	}

	if totalTickets == 0 {
		return 0.0
	}

	// Tỷ lệ trung bình có trọng số (chính xác hơn trung bình cộng đơn thuần)
	return roundFloat((float64(totalNoShow)/float64(totalTickets))*100, 2)
}

// calculateTotalLoss tính tổng doanh thu mất ước tính từ tất cả các ngày
func CalculateTotalLoss(report []NoShowDailyReport) float64 {
	var totalLoss float64 = 0

	for _, r := range report {
		totalLoss += r.EstimatedLoss
	}

	// Làm tròn về 0 chữ số thập phân (vì là tiền VND)
	return roundFloat(totalLoss, 0)
}

// Hàm phụ trợ để làm tròn số thực
func roundFloat(val float64, precision int) float64 {
	p := math.Pow(10, float64(precision))
	return math.Round(val*p) / p
}
func GetNoShowDailyReport(db *gorm.DB, from, to time.Time, cinemaID *uint) ([]NoShowDailyReport, error) {
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	to = time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, 999999999, to.Location())

	var results []NoShowDailyReport

	const bufferMinutes = 30

	query := `
WITH daily_stats AS (
    SELECT 
        DATE(st.start_time) AS show_date,
        COUNT(*) AS total_tickets,
        SUM(CASE WHEN t.used_at IS NOT NULL THEN 1 ELSE 0 END) AS checkin_tickets,
        SUM(t.price) AS total_revenue
    FROM tickets t
    JOIN showtimes st ON t.showtime_id = st.id
    JOIN rooms r ON st.room_id = r.id
    WHERE t.status IN ('PAID', 'BOOKED')
      AND st.start_time >= $1
      AND st.start_time <= $2
      AND ($3::bigint IS NULL OR r.cinema_id = $3::bigint)
    GROUP BY show_date
),
no_show_stats AS (
    SELECT 
        DATE(st.start_time) AS show_date,
        COUNT(*) AS no_show_tickets,
        SUM(t.price) AS no_show_revenue
    FROM tickets t
    JOIN showtimes st ON t.showtime_id = st.id
    JOIN movies m ON st.movie_id = m.id
    JOIN rooms r ON st.room_id = r.id
    WHERE t.status IN ('PAID', 'BOOKED')
      AND t.used_at IS NULL
      AND st.start_time + (m.duration + $4) * INTERVAL '1 minute' < NOW()
      AND st.start_time >= $5
      AND st.start_time <= $6
      AND ($7::bigint IS NULL OR r.cinema_id = $7::bigint)
    GROUP BY show_date
)
SELECT 
    COALESCE(ds.show_date::text, ns.show_date::text) AS date,
    COALESCE(ds.total_tickets, 0) AS total_tickets,
    COALESCE(ds.checkin_tickets, 0) AS checkin_tickets,
    COALESCE(ns.no_show_tickets, 0) AS no_show_tickets,
    CASE 
        WHEN COALESCE(ds.total_tickets, 0) = 0 THEN 0 
        ELSE ROUND(
            (COALESCE(ns.no_show_tickets, 0)::numeric / ds.total_tickets) * 100,
            2
        )
    END AS no_show_rate,
    COALESCE(ns.no_show_revenue, 0) AS estimated_loss
FROM daily_stats ds
FULL OUTER JOIN no_show_stats ns ON ds.show_date = ns.show_date
WHERE COALESCE(ds.show_date, ns.show_date) IS NOT NULL
ORDER BY date DESC;
`

	var cinemaIDParam interface{} = nil
	if cinemaID != nil {
		cinemaIDParam = *cinemaID
	}

	err := db.Raw(query,
		from,          // $1
		to,            // $2
		cinemaIDParam, // $3
		bufferMinutes, // $4
		from,          // $5
		to,            // $6
		cinemaIDParam, // $7
	).Scan(&results).Error

	if err != nil {
		return nil, err
	}

	return results, nil
}

type StaffCheckInReportItem struct {
	StaffID      uint   `json:"staffId"`
	FullName     string `json:"fullName"` // FirstName + LastName
	Username     string `json:"username"`
	PhoneNumber  string `json:"phoneNumber"`
	CheckInCount int    `json:"checkInCount"`
	CinemaName   string `json:"cinemaName"`
}

type StaffCheckInReportSummary struct {
	TotalCheckIns int `json:"totalCheckIns"`
	TotalStaff    int `json:"totalStaff"`
}

func GetStaffCheckInReport(db *gorm.DB, from, to time.Time, cinemaID *uint) ([]StaffCheckInReportItem, *StaffCheckInReportSummary, error) {
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	to = time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, 999999999, to.Location())

	var results []StaffCheckInReportItem

	query := `
SELECT 
    a.id AS staff_id,
    COALESCE(TRIM(s.first_name || ' ' || s.last_name), a.username) AS full_name,
    a.username AS username,
    COALESCE(s.phone_number, '') AS phone_number,
    COUNT(t.id) AS check_in_count,
    c.name AS cinema_name
FROM tickets t
JOIN accounts a ON t.checked_in_by = a.id
LEFT JOIN staffs s ON a.id = s.account_id
LEFT JOIN cinemas c ON a.cinema_id = c.id
WHERE t.used_at >= $1
  AND t.used_at <= $2
  AND a.role IN ('STAFF', 'CINEMA_MANAGER')
  AND ($3::bigint IS NULL OR a.cinema_id = $3::bigint)
GROUP BY a.id, full_name, a.username, phone_number, c.name
ORDER BY check_in_count DESC;
`

	var cinemaParam interface{} = nil
	if cinemaID != nil {
		cinemaParam = *cinemaID
	}

	err := db.Raw(query, from, to, cinemaParam).Scan(&results).Error
	if err != nil {
		return nil, nil, err
	}

	// Tính tổng
	totalCheckIns := 0
	for _, r := range results {
		totalCheckIns += r.CheckInCount
	}

	summary := &StaffCheckInReportSummary{
		TotalCheckIns: totalCheckIns,
		TotalStaff:    len(results),
	}

	return results, summary, nil
}

type NoShowDetailItem struct {
	ShowtimeID     uint    `json:"showtimeId"`
	PublicCode     string  `json:"publicCode"` // mã suất chiếu
	MovieTitle     string  `json:"movieTitle"`
	CinemaName     string  `json:"cinemaName"`
	RoomName       string  `json:"roomName"`
	StartTime      string  `json:"startTime"` // định dạng dd/MM/yyyy HH:mm
	TotalTickets   int     `json:"totalTickets"`
	CheckInTickets int     `json:"checkInTickets"`
	NoShowTickets  int     `json:"noShowTickets"`
	NoShowRate     float64 `json:"noShowRate"`
	EstimatedLoss  float64 `json:"estimatedLoss"`
}
type NoShowReportSummary struct {
	AverageNoShowRate  float64 `json:"averageNoShowRate"`
	TotalNoShowTickets int     `json:"totalNoShowTickets"`
	TotalLoss          float64 `json:"totalLoss"`
}

func GetNoShowDetailReport(db *gorm.DB, from, to time.Time, cinemaID *uint) ([]NoShowDetailItem, *NoShowReportSummary, error) {
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	to = time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, 999999999, to.Location())

	var results []NoShowDetailItem

	const bufferMinutes = 30

	query := `
SELECT 
    st.id AS showtime_id,
    st.public_code,
    m.title AS movie_title,
    c.name AS cinema_name,
    r.name AS room_name,
    TO_CHAR(st.start_time, 'DD/MM/YYYY HH24:MI') AS start_time,
    COUNT(t.id) AS total_tickets,
    SUM(CASE WHEN t.used_at IS NOT NULL THEN 1 ELSE 0 END) AS checkin_tickets,
    COUNT(t.id) - SUM(CASE WHEN t.used_at IS NOT NULL THEN 1 ELSE 0 END) AS no_show_tickets,
    ROUND(
        (COUNT(t.id) - SUM(CASE WHEN t.used_at IS NOT NULL THEN 1 ELSE 0 END))::numeric / NULLIF(COUNT(t.id), 0) * 100,
        2
    ) AS no_show_rate,
    SUM(st.price * COALESCE(stype.price_modifier, 1.0)) AS estimated_loss
FROM tickets t
JOIN showtimes st ON t.showtime_id = st.id
JOIN movies m ON st.movie_id = m.id
JOIN rooms r ON st.room_id = r.id
JOIN cinemas c ON r.cinema_id = c.id
JOIN showtime_seats sts ON t.showtime_seat_id = sts.id
JOIN seat_types stype ON sts.seat_type_id = stype.id
WHERE t.status IN ('EXPIRED')
  AND st.start_time >= $1
  AND st.start_time <= $2
  AND ($3::bigint IS NULL OR c.id = $3::bigint)
  AND st.start_time + (m.duration + $4) * INTERVAL '1 minute' < NOW()  -- suất đã kết thúc đủ lâu
GROUP BY st.id, st.public_code, m.title, c.name, r.name, st.start_time
HAVING COUNT(t.id) - SUM(CASE WHEN t.used_at IS NOT NULL THEN 1 ELSE 0 END) > 0  -- có ít nhất 1 no-show
ORDER BY no_show_tickets DESC, estimated_loss DESC
LIMIT 100;  -- giới hạn để tránh quá tải
`

	var cinemaParam interface{} = nil
	if cinemaID != nil {
		cinemaParam = *cinemaID
	}

	err := db.Raw(query, from, to, cinemaParam, bufferMinutes).Scan(&results).Error
	if err != nil {
		return nil, nil, err
	}

	// Tính summary
	var summary NoShowReportSummary
	for _, item := range results {
		summary.TotalNoShowTickets += item.NoShowTickets
		summary.TotalLoss += item.EstimatedLoss
	}

	if len(results) > 0 {
		totalTicketsAll := 0
		totalNoShowAll := 0
		for _, item := range results {
			totalTicketsAll += item.TotalTickets
			totalNoShowAll += item.NoShowTickets
		}
		if totalTicketsAll > 0 {
			summary.AverageNoShowRate = roundFloat(float64(totalNoShowAll)/float64(totalTicketsAll)*100, 2)
		}
	}

	return results, &summary, nil
}

type StaffCheckInDetailItem struct {
	ShowtimeID   uint   `json:"showtimeId"`
	PublicCode   string `json:"publicCode"`
	MovieTitle   string `json:"movieTitle"`
	CinemaName   string `json:"cinemaName"`
	RoomName     string `json:"roomName"`
	StartTime    string `json:"startTime"`    // "06/01/2026 20:00"
	CheckInCount int    `json:"checkInCount"` // số vé nhân viên này check-in trong suất
}

func GetStaffCheckInDetailReport(db *gorm.DB, from, to time.Time, cinemaID *uint, staffAccountID uint) ([]StaffCheckInDetailItem, error) {
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	to = time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, 999999999, to.Location())

	var results []StaffCheckInDetailItem

	query := `
SELECT 
    st.id AS showtime_id,
    st.public_code,
    m.title AS movie_title,
    c.name AS cinema_name,
    r.name AS room_name,
    TO_CHAR(st.start_time, 'DD/MM/YYYY HH24:MI') AS start_time,
    COUNT(t.id) AS check_in_count
FROM tickets t
JOIN showtimes st ON t.showtime_id = st.id
JOIN movies m ON st.movie_id = m.id
JOIN rooms r ON st.room_id = r.id
JOIN cinemas c ON r.cinema_id = c.id
WHERE t.checked_in_by = $1
  AND t.used_at >= $2
  AND t.used_at <= $3
  AND ($4::bigint IS NULL OR c.id = $4::bigint)
GROUP BY st.id, st.public_code, m.title, c.name, r.name, st.start_time
ORDER BY st.start_time DESC, check_in_count DESC;
`

	var cinemaParam interface{} = nil
	if cinemaID != nil {
		cinemaParam = *cinemaID
	}

	err := db.Raw(query, staffAccountID, from, to, cinemaParam).Scan(&results).Error
	return results, err
}

type NoShowTicketItem struct {
	OrderCode    string  `json:"orderCode"`
	CustomerName string  `json:"customerName"`
	Phone        string  `json:"phone"`
	Email        string  `json:"email"`
	MovieTitle   string  `json:"movieTitle"`
	CinemaName   string  `json:"cinemaName"`
	RoomName     string  `json:"roomName"`
	Showtime     string  `json:"showtime"` // "2025-12-26 18:00"
	Seats        string  `json:"seats"`    // "C5, C6, D7"
	TotalAmount  float64 `json:"totalAmount"`
	TicketCount  int     `json:"ticketCount"`
	NoShowCount  int     `json:"noShowCount"` // số vé chưa check-in

}

type NoShowTicketSummary struct {
	TotalNoShowTickets int     `json:"totalNoShowTickets"`
	TotalLostRevenue   float64 `json:"totalLostRevenue"`
	TotalEmptySeats    int     `json:"totalEmptySeats"`
	AverageTicketPrice float64 `json:"averageTicketPrice"`
}

func GetNoShowTicketReport(
	db *gorm.DB,
	from, to time.Time,
	cinemaID, movieID *uint,
	search string,
	limit, offset int,
) ([]NoShowTicketItem, *NoShowTicketSummary, int, error) {

	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	to = time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, 999999999, to.Location())

	var results []NoShowTicketItem

	baseQuery := `
SELECT 
    o.public_code AS order_code,
    COALESCE(NULLIF(TRIM(o.customer_name), ''), 'Khách lẻ') AS customer_name,
    COALESCE(o.phone, '') AS phone,
    COALESCE(o.email, '') AS email,
    m.title AS movie_title,
    cin.name AS cinema_name,
    r.name AS room_name,
    TO_CHAR(st.start_time, 'YYYY-MM-DD HH24:MI') AS showtime,
    STRING_AGG(CONCAT(s.row, s.column), ', ' ORDER BY s.row, s.column) AS seats,
    o.total_amount AS total_amount,
    COUNT(t.id) AS ticket_count,
    SUM(CASE WHEN t.status != 'CHECKED_IN' AND t.status != 'CANCELLED' THEN 1 ELSE 0 END) AS no_show_count
FROM orders o
JOIN tickets t ON t.order_id = o.id
JOIN showtimes st ON t.showtime_id = st.id
JOIN movies m ON st.movie_id = m.id
JOIN rooms r ON st.room_id = r.id
JOIN cinemas cin ON r.cinema_id = cin.id
JOIN showtime_seats sts ON t.showtime_seat_id = sts.id
JOIN seats s ON sts.seat_id = s.id
WHERE o.status = 'PAID'
  AND o.created_by = 0 
  AND st.start_time >= $1
  AND st.start_time <= $2
  AND ($3::bigint IS NULL OR r.cinema_id = $3)
  AND ($4::bigint IS NULL OR m.id = $4)
  AND (
    $5::text IS NULL OR $5 = ''
    OR o.public_code ILIKE '%' || $5 || '%'
    OR o.customer_name ILIKE '%' || $5 || '%'
    OR o.phone ILIKE '%' || $5 || '%'
    OR o.email ILIKE '%' || $5 || '%'
  )
  AND st.start_time + (m.duration || ' minutes')::interval + interval '30 minutes' < NOW()
GROUP BY 
    o.id, o.public_code, o.customer_name, o.phone, o.email,
    m.title, cin.name, r.name, st.start_time, o.total_amount
HAVING 
    SUM(CASE WHEN t.status != 'CHECKED_IN' AND t.status != 'CANCELLED' THEN 1 ELSE 0 END) > 0
`

	// Đếm tổng số bản ghi (cho phân trang)
	countQuery := "SELECT COUNT(*) FROM (" + baseQuery + ") AS counted"
	var total int
	err := db.Raw(countQuery, from, to, cinemaID, movieID, search).Scan(&total).Error
	if err != nil {
		return nil, nil, 0, err
	}

	// Query chính có LIMIT và OFFSET
	dataQuery := baseQuery + `
ORDER BY 
    st.start_time DESC, o.total_amount DESC
LIMIT $6 OFFSET $7;
`

	var cinemaParam, movieParam interface{}
	if cinemaID != nil {
		cinemaParam = *cinemaID
	}
	if movieID != nil {
		movieParam = *movieID
	}

	searchParam := ""
	if search != "" {
		searchParam = search
	}

	err = db.Raw(dataQuery, from, to, cinemaParam, movieParam, searchParam, limit, offset).Scan(&results).Error
	if err != nil {
		return nil, nil, 0, err
	}

	// Tính summary
	summary := &NoShowTicketSummary{}
	for _, r := range results {
		summary.TotalNoShowTickets += r.NoShowCount
		summary.TotalLostRevenue += r.TotalAmount
		summary.TotalEmptySeats += r.NoShowCount
	}
	if summary.TotalNoShowTickets > 0 {
		summary.AverageTicketPrice = summary.TotalLostRevenue / float64(summary.TotalNoShowTickets)
	}

	return results, summary, total, nil
}

type DashboardKPI struct {
	TotalRevenue    float64 `json:"totalRevenue"`
	TicketsSold     int64   `json:"ticketsSold"`
	OccupancyRate   float64 `json:"occupancyRate"`
	UniqueCustomers int64   `json:"uniqueCustomers"`
	RevenueChange   float64 `json:"revenueChangePct"` // % thay đổi so với kỳ trước
}

type TopMovieItem struct {
	Title          string  `json:"title"`
	Revenue        float64 `json:"revenue"`
	Tickets        int64   `json:"tickets"`
	ShowtimesCount int64   `json:"showtimesCount"`
	OccupancyAvg   float64 `json:"occupancyAvg"`
	AvgRating      float64 `json:"avgRating"` // Nếu Movie có rating
}

type RevenueByCinemaItem struct {
	CinemaName     string  `json:"cinemaName"`
	Revenue        float64 `json:"revenue"`
	Tickets        int64   `json:"tickets"`
	Occupancy      float64 `json:"occupancy"`
	AvgTicketPrice float64 `json:"avgTicketPrice"`
}

type OccupancyTrendItem struct {
	Date string  `json:"date"` // Format: 02/01
	Rate float64 `json:"rate"`
}

type DashboardSummary struct {
	TotalRevenue   float64 `json:"totalRevenue"`
	TotalTickets   int64   `json:"totalTickets"`
	AvgOccupancy   float64 `json:"avgOccupancy"`
	TotalCustomers int64   `json:"totalCustomers"`
}

type DashboardReport struct {
	Items          []interface{}         `json:"items"` // Linh hoạt cho top movies/revenue
	Summary        *DashboardSummary     `json:"summary"`
	Pagination     *PaginationInfo       `json:"pagination,omitempty"`
	Trends         []OccupancyTrendItem  `json:"trends,omitempty"`
	TopMovies      []TopMovieItem        `json:"top_movies"`
	RevenueCinemas []RevenueByCinemaItem `json:"revenue_cinemas"`
}

type PaginationInfo struct {
	CurrentPage int  `json:"currentPage"`
	TotalPages  int  `json:"totalPages"`
	TotalItems  int  `json:"totalItems"`
	Limit       int  `json:"limit"`
	HasNext     bool `json:"hasNext"`
	HasPrev     bool `json:"hasPrev"`
}

func GetDashboardReport(
	db *gorm.DB,
	from, to time.Time,
	cinemaID, movieID *uint,
	search string,
	limit, offset int,
) (*DashboardReport, error) {

	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	to = time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, 999999999, to.Location())

	// 1. Query KPI (tổng hợp)
	var kpi DashboardKPI
	kpiQuery := `
SELECT 
    COALESCE(SUM(o.actual_revenue), 0) AS total_revenue,
    COUNT(t.id) AS tickets_sold,
    COUNT(DISTINCT o.customer_name) AS unique_customers,
    COALESCE(AVG(
        (SELECT COUNT(tt.id)::float FROM tickets tt 
         WHERE tt.showtime_id = st.id 
           AND tt.status IN ('PAID','ISSUED','CHECK_IN','EXPIRED')
        ) / NULLIF(
            (SELECT COUNT(sts.id) FROM showtime_seats sts WHERE sts.showtime_id = st.id),
            0
        ) * 100
    ), 0) AS occupancy_rate
FROM orders o
LEFT JOIN tickets t ON t.order_id = o.id
LEFT JOIN showtimes st ON t.showtime_id = st.id
WHERE o.created_at >= $1  -- dùng created_at của order thay vì start_time
  AND o.created_at <= $2
  AND ($3::bigint IS NULL OR st.room_id IN (SELECT id FROM rooms WHERE cinema_id = $3))
  AND ($4::bigint IS NULL OR st.movie_id = $4)
  AND ($5::text IS NULL OR $5 = '' OR o.public_code ILIKE '%' || $5 || '%' OR o.customer_name ILIKE '%' || $5 || '%')
`
	err := db.Raw(kpiQuery, from, to, cinemaID, movieID, search).Scan(&kpi).Error
	if err != nil {
		return nil, err
	}

	// Tính revenue change (so với kỳ trước 7 ngày)
	prevFrom := from.AddDate(0, 0, -7)
	prevTo := to.AddDate(0, 0, -7)
	var prevRevenue float64
	db.Raw("SELECT COALESCE(SUM(actual_revenue), 0) FROM orders WHERE created_at BETWEEN $1 AND $2", prevFrom, prevTo).Scan(&prevRevenue)
	if prevRevenue > 0 {
		kpi.RevenueChange = (kpi.TotalRevenue - prevRevenue) / prevRevenue * 100
	}

	// 2. Top 5 Movies (items)
	var topMovies []TopMovieItem

	topQuery := `
SELECT 
    m.title,
    COALESCE(SUM(o.actual_revenue), 0) AS revenue,
    COUNT(t.id) AS tickets,
    COUNT(DISTINCT st.id) AS showtimes_count,
    COALESCE(AVG(
        (SELECT COUNT(tt.id)::float
         FROM tickets tt 
         WHERE tt.showtime_id = st.id 
           AND tt.status IN ('PAID','ISSUED','CHECK_IN','EXPIRED')
        ) / NULLIF(
            (SELECT COUNT(sts.id)
             FROM showtime_seats sts
             WHERE sts.showtime_id = st.id),
            0
        ) * 100
    ), 0) AS occupancy_avg
FROM movies m
JOIN showtimes st ON m.id = st.movie_id
JOIN rooms r ON st.room_id = r.id
LEFT JOIN tickets t ON st.id = t.showtime_id
LEFT JOIN orders o ON t.order_id = o.id
WHERE st.start_time BETWEEN $1 AND $2
  AND ($3::bigint IS NULL OR r.cinema_id = $3)
  AND ($4::bigint IS NULL OR m.id = $4)
  AND ($5::text IS NULL OR $5 = '' OR m.title ILIKE '%' || $5 || '%')
GROUP BY m.id, m.title
HAVING COALESCE(SUM(o.actual_revenue), 0) > 0 OR COUNT(t.id) > 0
ORDER BY revenue DESC
LIMIT 5;

`

	err = db.Raw(topQuery, from, to, cinemaID, movieID, search).Scan(&topMovies).Error
	if err != nil {
		return nil, err
	}

	// 3. Revenue by Cinema (thêm vào items nếu cần, hoặc riêng)
	var revenueCinemas []RevenueByCinemaItem

	revenueQuery := `
WITH showtime_stats AS (
  SELECT
    st.id AS showtime_id,
    COUNT(*) FILTER (
      WHERE s.status IN ('BOOKED','CHECKED_IN','EXPIRED')
    ) AS occupied_seats,
    COUNT(*) AS total_seats
  FROM showtimes st
  JOIN showtime_seats s ON st.id = s.showtime_id
  GROUP BY st.id
)
SELECT
  cin.id,
  cin.name AS cinema_name,
  SUM(o.actual_revenue) AS revenue,
  COUNT(t.id) AS tickets,
  AVG(
    ss.occupied_seats::float / NULLIF(ss.total_seats,0) * 100
  ) AS occupancy,
  SUM(o.actual_revenue) / NULLIF(COUNT(t.id),0) AS avg_ticket_price
FROM cinemas cin
JOIN rooms r ON cin.id = r.cinema_id
JOIN showtimes st ON r.id = st.room_id
JOIN tickets t ON st.id = t.showtime_id
JOIN orders o ON t.order_id = o.id AND o.status = 'PAID'
JOIN showtime_stats ss ON st.id = ss.showtime_id
WHERE st.start_time BETWEEN $1 AND $2
  AND ($3::bigint IS NULL OR cin.id = $3)
  AND ($4::bigint IS NULL OR st.movie_id = $4)
  AND ($5::text IS NULL OR $5 = '' OR cin.name ILIKE '%' || $5 || '%')
GROUP BY cin.id, cin.name
ORDER BY revenue DESC;


`

	err = db.Raw(revenueQuery, from, to, cinemaID, movieID, search).Scan(&revenueCinemas).Error
	if err != nil {
		return nil, err
	}

	// 4. Occupancy Over Time (trends)
	var trends []OccupancyTrendItem
	trendQuery := `
    SELECT 
        TO_CHAR(DATE(st.start_time), 'DD/MM') AS date,
        COALESCE(AVG(
            (SELECT COUNT(tt.id)::float FROM tickets tt WHERE tt.showtime_id = st.id AND tt.status IN ('BOOKED','USED')) /
            (SELECT COUNT(sts.id) FROM showtime_seats sts WHERE sts.showtime_id = st.id)
        ), 0) * 100 AS rate
    FROM showtimes st
    WHERE st.start_time >= $1 AND st.start_time <= $2
      AND ($3::bigint IS NULL OR st.room_id IN (SELECT id FROM rooms WHERE cinema_id = $3))
      AND ($4::bigint IS NULL OR st.movie_id = $4)
    GROUP BY DATE(st.start_time)
    ORDER BY DATE(st.start_time)
    `
	type TrendRow struct {
		Date string  `gorm:"column:date"`
		Rate float64 `gorm:"column:rate"`
	}
	var trendRows []TrendRow
	err = db.Raw(trendQuery, from, to, cinemaID, movieID).Scan(&trendRows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range trendRows {
		trends = append(trends, OccupancyTrendItem{Date: row.Date, Rate: row.Rate})
	}

	// 5. Đếm tổng (cho pagination, nếu áp dụng cho items)
	countQuery := "SELECT COUNT(*) FROM movies m JOIN showtimes st ON m.id = st.movie_id JOIN tickets t ON st.id = t.showtime_id JOIN orders o ON t.order_id = o.id WHERE o.status = 'PAID' AND o.created_by = 0 AND st.start_time >= $1 AND st.start_time <= $2" // Ví dụ cho top movies
	var total int
	db.Raw(countQuery, from, to).Scan(&total)

	// Summary
	summary := &DashboardSummary{
		TotalRevenue:   kpi.TotalRevenue,
		TotalTickets:   kpi.TicketsSold,
		AvgOccupancy:   kpi.OccupancyRate,
		TotalCustomers: kpi.UniqueCustomers,
	}

	// Report tổng hợp
	report := &DashboardReport{
		Items:          make([]interface{}, 0), // Có thể append topMovies và revenueCinemas nếu cần flatten
		Summary:        summary,
		Trends:         trends,
		TopMovies:      topMovies,
		RevenueCinemas: revenueCinemas,
		Pagination: &PaginationInfo{
			CurrentPage: (offset / limit) + 1,
			TotalPages:  (total + limit - 1) / limit,
			TotalItems:  total,
			Limit:       limit,
			HasNext:     offset+limit < total,
			HasPrev:     offset > 0,
		},
	}
	// Append items nếu cần (ví dụ: report.Items = append(report.Items, topMovies...) – nhưng vì khác type, dùng map hoặc riêng)

	return report, nil
}
