package handler

import (
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/utils"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

func NoShowReport(c *fiber.Ctx) error {
	fromStr := c.Query("from", time.Now().AddDate(0, 0, -7).Format("2006-01-02"))
	toStr := c.Query("to", time.Now().Format("2006-01-02"))

	from, _ := time.Parse("2006-01-02", fromStr)
	to, _ := time.Parse("2006-01-02", toStr)
	db := database.DB
	// Nếu là quản lý rạp → chỉ lấy dữ liệu rạp của họ
	var cinemaId *uint
	accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isManager {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Không có quyền", nil)
	}
	if isManager {
		if accountInfo.CinemaId == nil {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Manager chưa được gán rạp", nil)
		}

		cinemaId = accountInfo.CinemaId
	}
	report, err := utils.GetNoShowDailyReport(db, from, to, cinemaId)
	if err != nil {
		return utils.ErrorResponse(c, 500, "Lỗi lấy báo cáo", err)
	}

	return utils.SuccessResponse(c, 200, fiber.Map{
		"report": report,
		"summary": fiber.Map{
			"averageNoShowRate": utils.CalculateAverage(report),
			"totalNoShowTickets": func() int {
				sum := 0
				for _, r := range report {
					sum += r.NoShowTickets
				}
				return sum
			}(),
			"totalLoss": utils.CalculateTotalLoss(report),
		},
	})
}
func StaffCheckInReport(c *fiber.Ctx) error {
	fromStr := c.Query("from", time.Now().AddDate(0, 0, -7).Format("2006-01-02"))
	toStr := c.Query("to", time.Now().Format("2006-01-02"))

	from, _ := time.Parse("2006-01-02", fromStr)
	to, _ := time.Parse("2006-01-02", toStr)

	accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isManager {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Không có quyền truy cập báo cáo", nil)
	}

	var cinemaID *uint
	if isManager {
		if accountInfo.CinemaId == nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Quản lý chưa được gán rạp", nil)
		}
		cinemaID = accountInfo.CinemaId
	}
	// Nếu là Admin → cinemaID = nil → xem tất cả

	report, summary, err := utils.GetStaffCheckInReport(database.DB, from, to, cinemaID)
	if err != nil {
		return utils.ErrorResponse(c, 500, "Lỗi tải báo cáo nhân viên", err)
	}

	return utils.SuccessResponse(c, 200, fiber.Map{
		"report":  report,
		"summary": summary,
		"period": fiber.Map{
			"from": from.Format("02/01/2006"),
			"to":   to.Format("02/01/2006"),
		},
	})
}
func NoShowDetailReport(c *fiber.Ctx) error {
	fromStr := c.Query("from", time.Now().AddDate(0, 0, -7).Format("2006-01-02"))
	toStr := c.Query("to", time.Now().Format("2006-01-02"))

	from, _ := time.Parse("2006-01-02", fromStr)
	to, _ := time.Parse("2006-01-02", toStr)

	db := database.DB

	var cinemaID *uint

	accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isManager {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Không có quyền truy cập báo cáo", nil)
	}

	if isManager {
		if accountInfo.CinemaId == nil {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Quản lý chưa được gán rạp", nil)
		}
		cinemaID = accountInfo.CinemaId
	}
	// Admin: cinemaID = nil → xem tất cả

	report, summary, err := utils.GetNoShowDetailReport(db, from, to, cinemaID)
	if err != nil {
		return utils.ErrorResponse(c, 500, "Lỗi lấy báo cáo chi tiết", err)
	}

	return utils.SuccessResponse(c, 200, fiber.Map{
		"report":  report,
		"summary": summary,
		"period": fiber.Map{
			"from": from.Format("02/01/2006"),
			"to":   to.Format("02/01/2006"),
		},
	})
}
func StaffCheckInDetailReport(c *fiber.Ctx) error {
	staffIDStr := c.Params("staffId") // /admin/reports/staff-checkin-detail/:staffId
	if staffIDStr == "" {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "Thiếu ID nhân viên", nil)
	}

	staffID, err := strconv.ParseUint(staffIDStr, 10, 32)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "ID nhân viên không hợp lệ", nil)
	}

	fromStr := c.Query("from", time.Now().AddDate(0, 0, -7).Format("2006-01-02"))
	toStr := c.Query("to", time.Now().Format("2006-01-02"))

	from, _ := time.Parse("2006-01-02", fromStr)
	to, _ := time.Parse("2006-01-02", toStr)

	accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isManager {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Không có quyền", nil)
	}

	var cinemaID *uint
	if isManager {
		if accountInfo.CinemaId == nil {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Quản lý chưa được gán rạp", nil)
		}
		cinemaID = accountInfo.CinemaId
	}

	details, err := utils.GetStaffCheckInDetailReport(database.DB, from, to, cinemaID, uint(staffID))
	if err != nil {
		return utils.ErrorResponse(c, 500, "Lỗi lấy chi tiết check-in", err)
	}

	return utils.SuccessResponse(c, 200, fiber.Map{
		"details": details,
		"period": fiber.Map{
			"from": from.Format("02/01/2006"),
			"to":   to.Format("02/01/2006"),
		},
	})
}
func NoShowTicketReport(c *fiber.Ctx) error {
	// Bộ lọc
	fromStr := c.Query("from", time.Now().AddDate(0, 0, -7).Format("2006-01-02")) // 7 ngày gần nhất
	toStr := c.Query("to", time.Now().Format("2006-01-02"))
	cinemaIDStr := c.Query("cinemaId") // optional
	movieIDStr := c.Query("movieId")   // optional
	search := c.Query("search", "")    // mã vé, tên, sdt, email

	from, _ := time.Parse("2006-01-02", fromStr)
	to, _ := time.Parse("2006-01-02", toStr)
	pageStr := c.Query("page", "1")
	limitStr := c.Query("limit", "20") // mặc định 20 bản ghi/trang

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100 // giới hạn tối đa để tránh load nặng
	}
	offset := (page - 1) * limit
	var cinemaID *uint
	var movieID *uint
	if cinemaIDStr != "" {
		id64, err := strconv.ParseUint(cinemaIDStr, 10, 64)
		if err != nil {
			return utils.ErrorResponse(c, 400, "cinemaId không hợp lệ", nil)
		}

		id := uint(id64)
		cinemaID = &id
	}

	if movieIDStr != "" {
		id64, _ := strconv.ParseUint(movieIDStr, 10, 64)
		id := uint(id64)
		movieID = (*uint)(&id)
	}

	db := database.DB

	// Quyền: admin thấy tất cả, manager chỉ thấy rạp của mình
	accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isManager {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Không có quyền", nil)
	}
	if isManager && accountInfo.CinemaId != nil {
		cinemaID = accountInfo.CinemaId // manager chỉ thấy rạp mình quản lý
	}

	report, summary, total, err := utils.GetNoShowTicketReport(db, from, to, cinemaID, movieID, search, limit, offset)
	if err != nil {
		return utils.ErrorResponse(c, 500, "Lỗi lấy báo cáo vé no-show", err)
	}

	totalPages := (total + limit - 1) / limit // làm tròn lên

	return utils.SuccessResponse(c, 200, fiber.Map{
		"report":  report,
		"summary": summary,
		"pagination": fiber.Map{
			"currentPage": page,
			"totalPages":  totalPages,
			"totalItems":  total,
			"limit":       limit,
			"hasNext":     page < totalPages,
			"hasPrev":     page > 1,
		},
	})
}

func DashboardReport(c *fiber.Ctx) error {
	// Bộ lọc
	fromStr := c.Query("from", time.Now().AddDate(0, 0, -7).Format("2006-01-02")) // 7 ngày gần nhất
	toStr := c.Query("to", time.Now().Format("2006-01-02"))
	cinemaIDStr := c.Query("cinemaId") // optional
	movieIDStr := c.Query("movieId")   // optional
	search := c.Query("search", "")    // Tìm theo title, cinema name

	from, _ := time.Parse("2006-01-02", fromStr)
	to, _ := time.Parse("2006-01-02", toStr)
	pageStr := c.Query("page", "1")
	limitStr := c.Query("limit", "20")

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100 // Giới hạn để tránh load nặng
	}
	offset := (page - 1) * limit

	var cinemaID *uint
	var movieID *uint
	if cinemaIDStr != "" {
		id64, err := strconv.ParseUint(cinemaIDStr, 10, 64)
		if err != nil || id64 == 0 {
			return utils.ErrorResponse(c, 400, "cinemaId không hợp lệ", nil)
		}
		id := uint(id64)
		cinemaID = &id
	}
	if movieIDStr != "" {
		id64, err := strconv.ParseUint(movieIDStr, 10, 64)
		if err != nil || id64 == 0 {
			return utils.ErrorResponse(c, 400, "cinemaId không hợp lệ", nil)
		}
		id := uint(id64)
		movieID = &id
	}
	log.Printf(
		"Dashboard filter → cinemaID=%v movieID=%v search='%s'",
		cinemaID, movieID, search,
	)
	db := database.DB

	// Quyền: admin thấy tất cả, manager chỉ thấy rạp của mình
	accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)
	if !isAdmin && !isManager {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Không có quyền", nil)
	}
	if isManager && accountInfo.CinemaId != nil {
		cinemaID = accountInfo.CinemaId // Manager chỉ thấy rạp mình quản lý
	}

	report, err := utils.GetDashboardReport(db, from, to, cinemaID, movieID, search, limit, offset)
	if err != nil {
		return utils.ErrorResponse(c, 500, "Lỗi lấy báo cáo dashboard", err)
	}

	totalPages := report.Pagination.TotalPages

	return utils.SuccessResponse(c, 200, fiber.Map{
		"kpi":              report.Summary,        // Bao gồm KPI
		"top_movies":       report.TopMovies,      // Lấy từ query riêng nếu cần (hoặc append vào report)
		"revenue_cinemas":  report.RevenueCinemas, // Tương tự
		"occupancy_trends": report.Trends,
		"pagination": fiber.Map{
			"currentPage": page,
			"totalPages":  totalPages,
			"totalItems":  report.Pagination.TotalItems,
			"limit":       limit,
			"hasNext":     page < totalPages,
			"hasPrev":     page > 1,
		},
	})
}
