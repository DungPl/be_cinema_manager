package validate

import (
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

func GetShowtimeById(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}
		_, isAdmin, isQuanLy, _, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin && !isQuanLy {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền "))
		}
		// Kiểm tra lịch chiếu tồn tại
		var showtime model.Showtime
		if err := database.DB.Where("id = ? ", valueKey).First(&showtime).Error; err != nil {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Lịch chiếu không tồn tại", err, "showtimeId")
		}
		c.Locals("showtimeId", uint(valueKey))
		return c.Next()
	}
}
func EditShowtime(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}
		var input model.UpdateShowtimeInput
		// Parse JSON từ request body vào struct
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid input %s", err.Error()),
			})
		}

		// Validate input
		if err := validate.Struct(input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin && !isManager {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền "))
		}
		// Kiểm tra lịch chiếu tồn tại
		var showtime model.Showtime
		if err := database.DB.Where("id = ?", valueKey).First(&showtime).Error; err != nil {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Lịch chiếu không tồn tại", err, "showtimeId")
		}
		// Kiểm tra phòng và phim tồn tại
		var room model.Room
		if input.RoomId != nil {
			if err := database.DB.Where("id = ?  AND status = ?", input.RoomId, "active").First(&room).Error; err != nil {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Phòng chiếu không tồn tại hoặc không hoạt động", err, "roomId")
			}
			if isManager {
				if accountInfo.CinemaId == nil {
					return utils.ErrorResponse(c, fiber.StatusForbidden, "Manager chưa được gán rạp", nil)
				}

				if room.CinemaId != *accountInfo.CinemaId {
					return utils.ErrorResponseHaveKey(c, fiber.StatusForbidden,
						fmt.Sprintf("Bạn không có quyền sửa lịch cho phòng %d (thuộc rạp khác)", *input.RoomId),
						nil, "roomIds")
				}
			}
		}
		var movie model.Movie
		if input.MovieId != nil {
			if err := database.DB.Where("id = ?  AND status_movie = ?", input.MovieId, "NOW_SHOWING").First(&movie).Error; err != nil {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Phim không tồn tại hoặc không đang chiếu", err, "movieId")
			}
		}
		// Kiểm tra thời gian hợp lệ (StartTime trong tương lai hoặc hôm nay)
		currentTime := time.Now().In(time.FixedZone("ICT", 7*3600))
		if input.StartTime != nil {
			if input.StartTime.Before(currentTime) {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Thời gian bắt đầu phải từ hiện tại trở đi", nil, "startTime")
			}
		}
		if showtime.EndTime.Before(currentTime) {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Thời gian chiếu đã kết thúc", nil, "EndTime")
		}
		if showtime.StartTime.Before(currentTime) && showtime.EndTime.After(currentTime) {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Suất chiếu đã diễn ra", nil, "")
		}
		// Kiểm tra xung đột lịch chiếu (loại trừ chính lịch chiếu đang cập nhật)
		var conflictCount int64
		if err := database.DB.Model(&model.Showtime{}).
			Where("room_id = ? AND id != ? AND ((start_time <= ? AND end_time >= ?) OR (start_time <= ? AND end_time >= ?))",
				input.RoomId, valueKey, input.StartTime, input.StartTime, input.EndTime, input.EndTime).
			Count(&conflictCount).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Lỗi kiểm tra xung đột lịch chiếu: %s", err.Error()),
			})
		}
		if conflictCount > 0 {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Xung đột lịch chiếu trong khoảng thời gian này", nil, "startTime")
		}
		var ticketCount int64
		database.DB.Model(&model.Ticket{}).Where("showtime_id = ?", valueKey).Count(&ticketCount)

		if ticketCount > 0 {
			if input.RoomId != nil && *input.RoomId != showtime.RoomId {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Không thể đổi phòng", nil, "room")
			}
			if input.MovieId != nil && *input.MovieId != showtime.MovieId {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Không thể đổi phim", nil, "movie")
			}
		}
		c.Locals("inputEditShowtime", input)
		c.Locals("showtimeId", uint(valueKey))
		return c.Next()
	}
}
func DeleteShowtime(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}
		// Kiểm tra lịch chiếu tồn tại
		var showtime model.Showtime
		if err := database.DB.Where("id = ? ", valueKey).First(&showtime).Error; err != nil {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Lịch chiếu không tồn tại", err, "showtimeId")
		}
		accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin && !isManager {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền "))
		}
		if isManager {
			if accountInfo.CinemaId == nil {
				return utils.ErrorResponse(c, fiber.StatusForbidden, "Manager chưa được gán rạp", nil)
			}

			if showtime.Room.CinemaId != *accountInfo.CinemaId {
				return utils.ErrorResponseHaveKey(c, fiber.StatusForbidden,
					fmt.Sprintf("Bạn không có quyền xóa lịch cho phòng %d (thuộc rạp khác)", showtime.RoomId),
					nil, "roomIds")
			}
		}
		// Kiểm tra vé liên quan
		var ticketCount int64
		if err := database.DB.Model(&model.Ticket{}).Where("showtime_id = ?", valueKey).Count(&ticketCount).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Lỗi kiểm tra vé: %s", err.Error()),
			})
		}
		if ticketCount > 0 {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Không thể xóa lịch chiếu vì đã có vé được đặt", nil, "showtimeId")
		}

		c.Locals("showtimeId", valueKey)
		return c.Next()
	}
}
func CreateShowtimeBatch() fiber.Handler {
	return func(c *fiber.Ctx) error {
		accountInfo, isAdmin, isQuanLy, _, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin && !isQuanLy {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("bạn không có thẩm quyền "))
		}

		var input model.CreateShowtimeBatchInput

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

		// Parse ngày
		startDate, err1 := time.Parse("2006-01-02", input.StartDate)
		endDate, err2 := time.Parse("2006-01-02", input.EndDate)
		if err1 != nil || err2 != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Ngày không đúng định dạng (YYYY-MM-DD)", nil)
		}
		if endDate.Before(startDate) {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Ngày kết thúc phải sau ngày bắt đầu", nil)
		}
		// Lấy phim + duration
		db := database.DB

		var movie model.Movie
		if err := db.Where("id = ? AND status = ?", input.MovieID, "NOW_SHOWING").First(&movie).Error; err != nil {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Phim không tồn tại hoặc không đang chiếu", err, "movieId")
		}
		for _, roomID := range input.RoomIDs {
			var room model.Room
			if err := database.DB.Preload("Formats").First(&room, roomID).Error; err != nil {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Phòng không tồn tại", err, "roomIds")
			}
			if isQuanLy && room.CinemaId != *accountInfo.CinemaId {
				return utils.ErrorResponseHaveKey(c, fiber.StatusForbidden,
					fmt.Sprintf("Bạn không có quyền tạo lịch cho phòng %d (thuộc rạp khác)", roomID),
					nil, "roomIds")
			}
			if room.Status != "active" {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Phòng không hoạt động", nil, "roomIds")
			}

			for _, format := range input.Formats {
				hasFormat := false
				for _, f := range room.Formats {
					if f.Name == format {
						hasFormat = true
						break
					}
				}
				if !hasFormat {
					return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest,
						fmt.Sprintf("Phòng %d không hỗ trợ định dạng %s", roomID, format), nil, "formats")
				}
			}
		}

		// ✅ Lưu thông tin vào context để handler dùng lại
		c.Locals("batchInput", input)
		c.Locals("movieDuration", movie.Duration)
		c.Locals("startDate", startDate)
		c.Locals("endDate", endDate)
		//c.Locals("isVietnamese", movie.IsVietnamese) // giả sử có field

		return c.Next()

	}
}
func AutoGenerateShowtimeSchedule() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.AutoGenerateScheduleInput
		accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin && !isManager {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Không có quyền", nil)
		}
		//db := database.DB

		if err := c.BodyParser(&input); err != nil {
			return utils.ErrorResponse(c, 400, "Dữ liệu không hợp lệ", err)
		}
		if err := validate.Struct(input); err != nil {
			return utils.ErrorResponse(c, 400, err.Error(), err)
		}

		// Parse ngày

		startDate, err := time.Parse("2006-01-02", input.StartDate)
		if err != nil {
			return utils.ErrorResponse(c, 400, "startDate sai định dạng", err)
		}
		endDate, err := time.Parse("2006-01-02", input.EndDate)
		if err != nil {
			return utils.ErrorResponse(c, 400, "endDate sai định dạng", err)
		}
		if endDate.Before(startDate) {
			return utils.ErrorResponse(c, 400, "endDate phải sau startDate", nil)
		}
		for _, roomID := range input.RoomIDs {
			var room model.Room
			if err := database.DB.Preload("Formats").First(&room, roomID).Error; err != nil {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Phòng không tồn tại", err, "roomIds")
			}
			if isManager {
				if accountInfo.CinemaId == nil {
					return utils.ErrorResponse(c, fiber.StatusForbidden, "Manager chưa được gán rạp", nil)
				}

				if room.CinemaId != *accountInfo.CinemaId {
					return utils.ErrorResponseHaveKey(c, fiber.StatusForbidden,
						fmt.Sprintf("Bạn không có quyền tạo lịch cho phòng %d (thuộc rạp khác)", roomID),
						nil, "roomIds")
				}
			}
			if room.Status != "available" {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Phòng không hoạt động", nil, "roomIds")
			}

			for _, format := range input.Formats {
				hasFormat := false
				for _, f := range room.Formats {
					if f.Name == format {
						hasFormat = true
						break
					}
				}
				if !hasFormat {
					return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest,
						fmt.Sprintf("Phòng %d không hỗ trợ định dạng %s", roomID, format), nil, "formats")
				}
			}
		}
		// Lấy phim + duration
		var movie model.Movie
		if err := database.DB.First(&movie, input.MovieID).Error; err != nil {
			return utils.ErrorResponse(c, 404, "Phim không tồn tại", err)
		}
		if movie.StatusMovie == "ENDED" || movie.IsAvailable == false {
			return utils.ErrorResponse(c, 400, "Phim đã bị vô hiệu hóa", nil)
		}

		// 2) Kiểm tra ngày kết thúc
		now := time.Now()
		if movie.DateEnd != nil && movie.DateEnd.Time.Before(now) {
			return utils.ErrorResponse(c, 400, "Phim đã hết thời gian chiếu", nil)
		}
		c.Locals("input", input)
		c.Locals("movie", movie)
		c.Locals("startDate", startDate)
		c.Locals("endDate", endDate)
		c.Locals("useTemplate", false)
		c.Locals("template", nil)
		return c.Next()
	}
}

func AutoSchedule() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// === LẤY THÔNG TIN NGƯỜI DÙNG TỪ TOKEN ===
		accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin && !isManager {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Không có quyền", nil)
		}

		// === PARSE INPUT ===
		var input struct {
			TemplateID uint     `json:"templateId" validate:"required"`
			ApplyDates []string `json:"applyDates" validate:"required,min=1,dive,required,datetime=2006-01-02"`
		}
		if err := c.BodyParser(&input); err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Dữ liệu không hợp lệ", err)
		}
		if err := validate.Struct(input); err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, err.Error(), nil)
		}

		// === LẤY TEMPLATE + ITEMS + PHÒNG ===
		var template struct {
			ID       uint
			MovieID  uint
			CinemaID uint // ← Bắt buộc có
			Items    []struct {
				RoomID uint   `gorm:"column:room_id"`
				Time   string `gorm:"column:time"`
				Format string `gorm:"column:format"`
			}
		}

		if err := database.DB.
			Table("schedule_templates").
			Where("id = ?", input.TemplateID).
			Preload("Items").
			First(&template).Error; err != nil {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Template không tồn tại", err)
		}

		// === KIỂM TRA QUYỀN THEO RẠP (CHỈ MANAGER) ===
		if isManager && accountInfo.CinemaId != nil {
			// Kiểm tra: template phải thuộc rạp của manager
			if template.CinemaID != *accountInfo.CinemaId {
				return utils.ErrorResponse(c, fiber.StatusForbidden,
					"Bạn chỉ được tạo lịch cho rạp của mình", nil)
			}

			// Kiểm tra từng phòng trong template
			for _, item := range template.Items {
				var room model.Room
				if err := database.DB.Select("cinema_id").First(&room, item.RoomID).Error; err != nil {
					return utils.ErrorResponse(c, fiber.StatusBadRequest, "Phòng không tồn tại", err)
				}
				if room.CinemaId != *accountInfo.CinemaId {
					return utils.ErrorResponse(c, fiber.StatusForbidden,
						fmt.Sprintf("Phòng %d không thuộc rạp của bạn", item.RoomID), nil)
				}
			}
		}

		// === LẤY PHIM ===
		var movie struct {
			Duration int
		}
		if err := database.DB.
			Model(&model.Movie{}).
			Where("id = ?", template.MovieID).
			Select("duration").
			First(&movie).Error; err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Phim không tồn tại", err)
		}

		// === KIỂM TRA PHÒNG + ĐỊNH DẠNG (giữ nguyên) ===
		// ... (giống code cũ)

		// === LƯU VÀO LOCALS ===
		c.Locals("template", template)
		c.Locals("movieDuration", movie.Duration)
		c.Locals("applyDates", input.ApplyDates)

		return c.Next()

	}
}
