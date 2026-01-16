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
	TicketChange    float64 `json:"ticketChangePct"`
	CustomerChange  float64 `json:"customerChangePct"`
	OccupancyChange float64 `json:"occupancyChangePct"`
}

type TopMovieItem struct {
	Title          string  `json:"title"`
	Revenue        float64 `json:"revenue"`
	Tickets        int64   `json:"tickets"`
	ShowtimesCount int64   `json:"showtimesCount"`
	OccupancyAvg   float64 `json:"occupancyAvg"`
	AvgRating      float64 `json:"avgRating"` // Nếu Movie có rating
}
type TicketByHourItem struct {
	TimeRange string  `json:"timeRange"` // "09-12"
	Tickets   int64   `json:"tickets"`
	Percent   float64 `json:"percent"`
}
type RevenueByCinemaItem struct {
	CinemaName     string  `json:"cinemaName"`
	Revenue        float64 `json:"revenue"`
	Tickets        int64   `json:"tickets"`
	OccupancyAvg   float64 `json:"occupancyAvg"`
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

	PrevTotalRevenue   float64 `json:"prevTotalRevenue"`
	PrevTotalTickets   int64   `json:"prevTotalTickets"`
	PrevAvgOccupancy   float64 `json:"prevAvgOccupancy"`
	PrevTotalCustomers int64   `json:"prevTotalCustomers"`

	RevenueChangePct   float64 `json:"revenueChangePct"`
	TicketChangePct    float64 `json:"ticketChangePct"`
	CustomerChangePct  float64 `json:"customerChangePct"`
	OccupancyChangePct float64 `json:"occupancyChangePct"`
}

type DashboardReport struct {
	Items          []interface{}         `json:"items"` // Linh hoạt cho top movies/revenue
	Summary        *DashboardSummary     `json:"summary"`
	Pagination     *PaginationInfo       `json:"pagination,omitempty"`
	Trends         []OccupancyTrendItem  `json:"trends,omitempty"`
	TopMovies      []TopMovieItem        `json:"top_movies"`
	RevenueCinemas []RevenueByCinemaItem `json:"revenue_cinemas"`
	DailyMetrics   []DailyMetric         `json:"daily_metrics"`
	TicketByHours  []TicketByHourItem    `json:"ticket_by_hours"`
}
type PrevKPI struct {
	Revenue   float64
	Tickets   int64
	Customers int64
	Occupancy float64
}
type DailyMetric struct {
	Date    string  `json:"date"` // "02/01"
	Revenue float64 `json:"revenue"`
	Tickets int64   `json:"tickets"`
}
type PaginationInfo struct {
	CurrentPage int  `json:"currentPage"`
	TotalPages  int  `json:"totalPages"`
	TotalItems  int  `json:"totalItems"`
	Limit       int  `json:"limit"`
	HasNext     bool `json:"hasNext"`
	HasPrev     bool `json:"hasPrev"`
}

func pctChange(current, previous float64) float64 {
	if previous == 0 {
		return 0
	}
	return (current - previous) / previous * 100
}
func GetDashboardReport(
	db *gorm.DB,
	from, to time.Time,
	cinemaID, movieID *uint,
	province *string,
	search string,
	limit, offset int,
) (*DashboardReport, error) {

	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	to = time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, 999999999, to.Location())

	// 1. KPI hiện tại
	var kpi DashboardKPI
	kpiQuery := `
WITH paid_orders AS (
    SELECT DISTINCT
        o.id,
        o.actual_revenue,
        o.customer_name,
        o.created_at
    FROM orders o
    JOIN tickets t ON t.order_id = o.id
    JOIN showtimes st ON t.showtime_id = st.id
    JOIN rooms r ON st.room_id = r.id
    JOIN cinemas cin ON r.cinema_id = cin.id
    LEFT JOIN addresses a ON cin.id = a.cinema_id
    WHERE o.status = 'PAID'
      AND o.created_at >= $1
      AND o.created_at <= $2
      AND ($3::bigint IS NULL OR cin.id = $3)
      AND ($4::bigint IS NULL OR st.movie_id = $4)
      AND ($5::text IS NULL OR $5 = '' OR LOWER(a.province) = LOWER($5))
      AND ($6::text IS NULL OR $6 = '' OR o.public_code ILIKE '%' || $6 || '%' OR o.customer_name ILIKE '%' || $6 || '%')
),
paid_tickets AS (
    SELECT 
        t.id,
        t.showtime_id
    FROM tickets t
    JOIN paid_orders po ON t.order_id = po.id
),
showtime_occupancy AS (
    SELECT 
        AVG(
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
        ) AS occupancy_rate
    FROM showtimes st
    JOIN rooms r ON st.room_id = r.id
    JOIN cinemas cin ON r.cinema_id = cin.id
    LEFT JOIN addresses a ON cin.id = a.cinema_id
    WHERE st.start_time >= $1 AND st.start_time <= $2
      AND ($3::bigint IS NULL OR cin.id = $3)
      AND ($4::bigint IS NULL OR st.movie_id = $4)
      AND ($5::text IS NULL OR LOWER(a.province) = LOWER($5))
)
SELECT
    COALESCE(SUM(po.actual_revenue), 0) AS total_revenue,
    (SELECT COUNT(DISTINCT pt.id) FROM paid_tickets pt) AS tickets_sold,
    COUNT(DISTINCT po.customer_name) AS unique_customers,
    COALESCE((SELECT occupancy_rate FROM showtime_occupancy), 0) AS occupancy_rate
FROM paid_orders po;
`
	err := db.Raw(kpiQuery, from, to, cinemaID, movieID, province, search).Scan(&kpi).Error
	if err != nil {
		return nil, err
	}

	// 2. KPI kỳ trước (7 ngày trước) - truyền đúng tham số thời gian
	prevFrom := from.AddDate(0, 0, -7)
	prevTo := to.AddDate(0, 0, -7)
	var prevKPI struct {
		TotalRevenue    float64
		TicketsSold     int64
		UniqueCustomers int64
		OccupancyRate   float64
	}
	err = db.Raw(kpiQuery, prevFrom, prevTo, cinemaID, movieID, province, search).Scan(&prevKPI).Error
	if err != nil {
		return nil, err
	}

	// Tính % thay đổi
	kpi.RevenueChange = pctChange(kpi.TotalRevenue, prevKPI.TotalRevenue)
	kpi.TicketChange = pctChange(float64(kpi.TicketsSold), float64(prevKPI.TicketsSold))
	kpi.CustomerChange = pctChange(float64(kpi.UniqueCustomers), float64(prevKPI.UniqueCustomers))
	kpi.OccupancyChange = kpi.OccupancyRate - prevKPI.OccupancyRate

	// 3. Top 5 Movies
	var topMovies []TopMovieItem
	topQuery := `
WITH filtered_showtimes AS (
    SELECT
        st.id AS showtime_id,
        st.movie_id
    FROM showtimes st
    JOIN rooms r ON r.id = st.room_id
    JOIN cinemas cin ON cin.id = r.cinema_id
    LEFT JOIN addresses a ON a.cinema_id = cin.id
    WHERE st.start_time BETWEEN $1 AND $2
         AND ($3::bigint IS NULL OR cin.id = $3)
      AND ($4::bigint IS NULL OR st.movie_id = $4)
      AND ($5::text IS NULL OR LOWER(a.province) = LOWER($5))
),

order_movie AS (
    -- 1 ORDER = 1 MOVIE
    SELECT DISTINCT
        o.id,
        o.actual_revenue,
        st.movie_id
    FROM orders o
    JOIN tickets t ON t.order_id = o.id
    JOIN showtimes st ON st.id = t.showtime_id
    JOIN filtered_showtimes fs ON fs.showtime_id = st.id
    WHERE o.status = 'PAID'
),

movie_revenue AS (
    SELECT
        movie_id,
        SUM(actual_revenue) AS revenue
    FROM order_movie
    GROUP BY movie_id
),

movie_tickets AS (
    SELECT
        st.movie_id,
        COUNT(t.id) AS tickets
    FROM tickets t
    JOIN orders o ON o.id = t.order_id AND o.status = 'PAID'
    JOIN showtimes st ON st.id = t.showtime_id
    JOIN filtered_showtimes fs ON fs.showtime_id = st.id
    GROUP BY st.movie_id
),
movie_showtimes_count AS (
    SELECT
        movie_id,
        COUNT(DISTINCT showtime_id) AS showtimes_count
    FROM filtered_showtimes
    GROUP BY movie_id
),
-- ✅ 1️⃣ Occupancy THEO SHOWTIME
showtime_occupancy AS (
    SELECT
        st.movie_id,
        st.id AS showtime_id,
        COUNT(*) FILTER (
            WHERE s.status IN ('SOLD','CHECKED_IN','EXPIRED')
        )::float
        / NULLIF(COUNT(s.id),0) * 100 AS occupancy
    FROM showtimes st
    JOIN showtime_seats s ON s.showtime_id = st.id
    JOIN filtered_showtimes fs ON fs.showtime_id = st.id
    GROUP BY st.movie_id, st.id
),

-- ✅ 2️⃣ AVG occupancy theo MOVIE
movie_occupancy AS (
    SELECT
        movie_id,
        AVG(occupancy) AS occupancy_avg
    FROM showtime_occupancy
    GROUP BY movie_id
)

SELECT
    m.title,
    COALESCE(r.revenue,0) AS revenue,
    COALESCE(t.tickets,0) AS tickets,
    COALESCE(sc.showtimes_count,0) AS showtimes_count,
    COALESCE(o.occupancy_avg,0) AS occupancy_avg,
    0 AS avg_rating
FROM movies m
LEFT JOIN movie_revenue r ON r.movie_id = m.id
LEFT JOIN movie_tickets t ON t.movie_id = m.id
LEFT JOIN movie_showtimes_count sc ON sc.movie_id = m.id
LEFT JOIN movie_occupancy o ON o.movie_id = m.id
WHERE r.revenue IS NOT NULL
ORDER BY r.revenue DESC
LIMIT 5;

`
	err = db.Raw(topQuery, from, to, cinemaID, movieID, province).Scan(&topMovies).Error
	if err != nil {
		return nil, err
	}

	// 4. Doanh thu theo rạp + avg_ticket_price từ tickets.price
	var revenueCinemas []RevenueByCinemaItem
	revenueQuery := `
WITH paid_orders AS (
    SELECT DISTINCT
        o.id AS order_id,
        o.actual_revenue,
        r.cinema_id
    FROM orders o
    JOIN tickets t      ON t.order_id = o.id
    JOIN showtimes st   ON st.id = t.showtime_id
    JOIN rooms r        ON r.id = st.room_id
    JOIN cinemas cin    ON cin.id = r.cinema_id
    LEFT JOIN addresses a ON a.cinema_id = cin.id
    WHERE o.status = 'PAID'
      AND st.start_time BETWEEN $1 AND $2
      AND ($3::bigint IS NULL OR cin.id = $3)
      AND ($4::bigint IS NULL OR st.movie_id = $4)
      AND ($5::text IS NULL OR LOWER(a.province) = LOWER($5))
      AND ($6::text IS NULL OR $6 = '' OR cin.name ILIKE '%' || $6 || '%')
),

order_ticket_count AS (
    SELECT
        o.id AS order_id,
        COUNT(t.id) AS tickets
    FROM orders o
    JOIN tickets t      ON t.order_id = o.id
    JOIN showtimes st   ON st.id = t.showtime_id
    JOIN rooms r        ON r.id = st.room_id
    JOIN cinemas cin    ON cin.id = r.cinema_id
    LEFT JOIN addresses a ON a.cinema_id = cin.id
    WHERE o.status = 'PAID'
      AND st.start_time BETWEEN $1 AND $2
      AND ($3::bigint IS NULL OR cin.id = $3)
      AND ($4::bigint IS NULL OR st.movie_id = $4)
      AND ($5::text IS NULL OR LOWER(a.province) = LOWER($5))
      AND ($6::text IS NULL OR $6 = '' OR cin.name ILIKE '%' || $6 || '%')
    GROUP BY o.id
),

showtime_occupancy AS (
    SELECT
        r.cinema_id,
        st.id AS showtime_id,
        COUNT(*) FILTER (
            WHERE ss.status IN ('SOLD','CHECKED_IN','EXPIRED')
        )::float
        / NULLIF(COUNT(ss.id), 0) * 100 AS occupancy
    FROM showtimes st
    JOIN rooms r ON r.id = st.room_id
    JOIN cinemas cin ON cin.id = r.cinema_id
    JOIN showtime_seats ss ON ss.showtime_id = st.id
    LEFT JOIN addresses a ON a.cinema_id = cin.id
    WHERE st.start_time BETWEEN $1 AND $2
      AND ($3::bigint IS NULL OR cin.id = $3)
      AND ($4::bigint IS NULL OR st.movie_id = $4)
      AND ($5::text IS NULL OR LOWER(a.province) = LOWER($5))
      AND ($6::text IS NULL OR $6 = '' OR cin.name ILIKE '%' || $6 || '%')
    GROUP BY r.cinema_id, st.id
),

cinema_occupancy AS (
    SELECT
        cinema_id,
        AVG(occupancy) AS occupancy_avg
    FROM showtime_occupancy
    GROUP BY cinema_id
)

SELECT
    cin.id   AS cinema_id,
    cin.name AS cinema_name,

    SUM(po.actual_revenue)      AS revenue,
    SUM(ot.tickets)             AS tickets,
    COUNT(DISTINCT po.order_id) AS total_orders,

    SUM(po.actual_revenue)
      / NULLIF(SUM(ot.tickets),0) AS avg_ticket_price,

    COALESCE(co.occupancy_avg, 0) AS occupancy_avg

FROM paid_orders po
JOIN order_ticket_count ot ON ot.order_id = po.order_id
JOIN cinemas cin ON cin.id = po.cinema_id
LEFT JOIN cinema_occupancy co ON co.cinema_id = cin.id

GROUP BY cin.id, cin.name, co.occupancy_avg
ORDER BY revenue DESC;



`
	err = db.Raw(revenueQuery, from, to, cinemaID, movieID, province, search).Scan(&revenueCinemas).Error
	if err != nil {
		return nil, err
	}

	// 5. Ticket by hour
	var ticketByHours []TicketByHourItem
	ticketByHourQuery := `
WITH total_tickets AS (
  SELECT COUNT(t.id) AS total
  FROM tickets t
  JOIN showtimes st ON t.showtime_id = st.id
  JOIN orders o ON t.order_id = o.id AND o.status = 'PAID'
  WHERE st.start_time BETWEEN $1 AND $2
),
hourly_tickets AS (
  SELECT
    CASE
      WHEN EXTRACT(HOUR FROM st.start_time) BETWEEN 9 AND 11 THEN '09-12'
      WHEN EXTRACT(HOUR FROM st.start_time) BETWEEN 12 AND 14 THEN '12-15'
      WHEN EXTRACT(HOUR FROM st.start_time) BETWEEN 15 AND 17 THEN '15-18'
      WHEN EXTRACT(HOUR FROM st.start_time) BETWEEN 18 AND 20 THEN '18-21'
      WHEN EXTRACT(HOUR FROM st.start_time) BETWEEN 21 AND 23 THEN '21-24'
      ELSE 'Khác'
    END AS time_range,
    COUNT(t.id) AS tickets
  FROM tickets t
  JOIN showtimes st ON t.showtime_id = st.id
  JOIN orders o ON t.order_id = o.id AND o.status = 'PAID'
  WHERE st.start_time BETWEEN $1 AND $2
  GROUP BY time_range
)
SELECT
  h.time_range,
  h.tickets,
  ROUND(h.tickets::numeric / NULLIF(tt.total, 0) * 100, 2) AS percent
FROM hourly_tickets h
CROSS JOIN total_tickets tt
ORDER BY h.time_range
`
	err = db.Raw(ticketByHourQuery, from, to).Scan(&ticketByHours).Error
	if err != nil {
		return nil, err
	}

	// 6. Daily Metrics
	var dailyMetrics []DailyMetric
	dailyQuery := `
WITH daily_orders AS (
    SELECT DISTINCT
        DATE(o.created_at) AS order_date,
        o.id AS order_id,
        o.actual_revenue
    FROM orders o
    JOIN tickets t ON t.order_id = o.id  -- đảm bảo order có vé
    JOIN showtimes st ON t.showtime_id = st.id
    JOIN rooms r ON st.room_id = r.id
    JOIN cinemas cin ON r.cinema_id = cin.id
    LEFT JOIN addresses a ON cin.id = a.cinema_id
    WHERE o.created_at >= $1
      AND o.created_at <= $2
      AND o.status = 'PAID'
      -- thêm lọc cinema/movie/province/search nếu cần
       AND ($3::bigint IS NULL OR cin.id = $3)
    AND ($4::bigint IS NULL OR st.movie_id = $4)
      AND ($5::text IS NULL OR LOWER(a.province) = LOWER($5))
       AND ($6::text IS NULL OR $6 = '' OR o.public_code ILIKE '%' || $6 || '%' OR o.customer_name ILIKE '%' || $6 || '%')
),
daily_tickets AS (
    SELECT 
        DATE(o.created_at) AS ticket_date,
        COUNT(DISTINCT t.id) AS tickets
    FROM orders o
    JOIN tickets t ON t.order_id = o.id
    WHERE o.status = 'PAID'
      AND o.created_at >= $1
      AND o.created_at <= $2
    GROUP BY DATE(o.created_at)
)
SELECT 
    TO_CHAR(d.order_date, 'DD/MM') AS date,
    COALESCE(SUM(d.actual_revenue), 0) AS revenue,
    COALESCE(dt.tickets, 0) AS tickets
FROM daily_orders d
LEFT JOIN daily_tickets dt ON dt.ticket_date = d.order_date
GROUP BY d.order_date, dt.tickets
ORDER BY d.order_date
`
	err = db.Raw(dailyQuery, from, to, cinemaID, movieID, province, search).Scan(&dailyMetrics).Error
	if err != nil {
		return nil, err
	}

	// 7. Occupancy Trends
	var trends []OccupancyTrendItem
	trendQuery := `
SELECT 
    TO_CHAR(DATE(st.start_time), 'DD/MM') AS date,
    COALESCE(AVG(
        (SELECT COUNT(tt.id)::float FROM tickets tt WHERE tt.showtime_id = st.id AND tt.status IN ('PAID','ISSUED','CHECKED_IN','EXPIRED')) /
        (SELECT COUNT(sts.id) FROM showtime_seats sts WHERE sts.showtime_id = st.id)
    ), 0) * 100 AS rate
FROM showtimes st
JOIN rooms r ON st.room_id = r.id
JOIN cinemas cin ON r.cinema_id = cin.id
LEFT JOIN addresses a ON cin.id = a.cinema_id
WHERE st.start_time >= $1 AND st.start_time <= $2
  AND ($3::bigint IS NULL OR cin.id = $3)
  AND ($4::bigint IS NULL OR st.movie_id = $4)
  AND ($5::text IS NULL OR LOWER(a.province) = LOWER($5))
GROUP BY DATE(st.start_time)
ORDER BY DATE(st.start_time)
`
	type TrendRow struct {
		Date string  `gorm:"column:date"`
		Rate float64 `gorm:"column:rate"`
	}
	var trendRows []TrendRow
	err = db.Raw(trendQuery, from, to, cinemaID, movieID, province).Scan(&trendRows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range trendRows {
		trends = append(trends, OccupancyTrendItem{Date: row.Date, Rate: row.Rate})
	}

	// Summary
	summary := &DashboardSummary{
		TotalRevenue:       kpi.TotalRevenue,
		TotalTickets:       kpi.TicketsSold,
		AvgOccupancy:       kpi.OccupancyRate,
		TotalCustomers:     kpi.UniqueCustomers,
		PrevTotalRevenue:   prevKPI.TotalRevenue,
		PrevTotalTickets:   prevKPI.TicketsSold,
		PrevAvgOccupancy:   prevKPI.OccupancyRate,
		PrevTotalCustomers: prevKPI.UniqueCustomers,
		RevenueChangePct:   kpi.RevenueChange,
		TicketChangePct:    kpi.TicketChange,
		CustomerChangePct:  kpi.CustomerChange,
		OccupancyChangePct: kpi.OccupancyChange,
	}
	countQuery := "SELECT COUNT(*) FROM movies m JOIN showtimes st ON m.id = st.movie_id JOIN tickets t ON st.id = t.showtime_id JOIN orders o ON t.order_id = o.id WHERE o.status = 'PAID' AND o.created_by = 0 AND st.start_time >= $1 AND st.start_time <= $2" // Ví dụ cho top movies
	var total int
	db.Raw(countQuery, from, to).Scan(&total)
	// Report
	report := &DashboardReport{
		Summary:        summary,
		Trends:         trends,
		TopMovies:      topMovies,
		RevenueCinemas: revenueCinemas,
		DailyMetrics:   dailyMetrics,
		TicketByHours:  ticketByHours,
		Pagination: &PaginationInfo{
			CurrentPage: (offset / limit) + 1,
			TotalPages:  (total + limit - 1) / limit,
			TotalItems:  total,
			Limit:       limit,
			HasNext:     offset+limit < total,
			HasPrev:     offset > 0,
		},
	}

	return report, nil
}
