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
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

var roomLimits = map[model.RoomType]struct {
	MinRows, MaxRows       int
	MinColumns, MaxColumns int
}{
	model.Small:  {MinRows: 5, MaxRows: 8, MinColumns: 10, MaxColumns: 14},
	model.Medium: {MinRows: 9, MaxRows: 12, MinColumns: 11, MaxColumns: 16},
	model.Large:  {MinRows: 13, MaxRows: 20, MinColumns: 12, MaxColumns: 20},
	model.IMAX:   {MinRows: 15, MaxRows: 25, MinColumns: 14, MaxColumns: 24},
	model.FourDX: {MinRows: 8, MaxRows: 15, MinColumns: 10, MaxColumns: 18},
}

func CreateRoom() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var input model.CreateScreeningRoomInput
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
		accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin && !isManager {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
		}
		if isManager {
			if accountInfo.CinemaId == nil || *accountInfo.CinemaId != input.CinemaId {
				return utils.ErrorResponse(c, fiber.StatusForbidden, "Không có quyền chỉnh sửa phòng này", errors.New("manager not assigned to this cinema"))
			}
		}
		// Kiểm tra Cinema tồn tại
		var cinema model.Cinema
		if err := database.DB.First(&cinema, input.CinemaId).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Rạp chiếu phim không tồn tại", fmt.Errorf("cinemaId not found"), "cinemaId")
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Lỗi truy vấn cơ sở dữ liệu: %s", err.Error()),
			})
		}
		// Validate input

		validate.RegisterValidation("rowsRegex", func(fl validator.FieldLevel) bool {
			rows := fl.Field().String()
			if len(rows) < 9 {
				return false
			}
			seen := make(map[rune]bool)
			for i, r := range rows {
				if r < 'A' || r > 'K' || seen[r] {
					return false
				}
				if i > 0 && r != rune(rows[i-1])+1 {
					return false // Phải là các chữ cái liên tiếp
				}
				seen[r] = true
			}
			return true
		})
		// Kiểm tra định dạng phim (Formats)
		var formats []model.Format
		if input.FormatIDs != nil && len(input.FormatIDs) > 0 {
			if err := database.DB.Where("id IN ?", input.FormatIDs).Find(&formats).Error; err != nil {
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi DB", err)
			}
			if len(formats) != len(input.FormatIDs) {
				return utils.ErrorResponse(c, fiber.StatusBadRequest, "Một số định dạng không tồn tại", nil)
			}
		}
		if input.Type == "" {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Vui lòng chọn loại phòng", nil, "type")
		}

		// Kiểm tra Type hợp lệ
		validType := false
		for _, t := range []model.RoomType{model.Small, model.Medium, model.Large, model.IMAX, model.FourDX} {
			if input.Type == t {
				validType = true
				break
			}
		}
		if !validType {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Loại phòng không hợp lệ", nil, "type")
		}

		// Tính số hàng từ chuỗi Row (ví dụ: "ABCDEFGHI" → 9 hàng)
		numRows := len(input.Row)

		// Kiểm tra giới hạn theo loại phòng
		limit, exists := roomLimits[input.Type]
		if !exists {
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Cấu hình giới hạn phòng bị thiếu", nil)
		}

		if numRows < limit.MinRows || numRows > limit.MaxRows {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest,
				fmt.Sprintf("Loại phòng %s phải có từ %d đến %d hàng ghế (hiện tại: %d)",
					input.Type, limit.MinRows, limit.MaxRows, numRows), nil, "row")
		}

		if input.Columns < limit.MinColumns || input.Columns > limit.MaxColumns {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest,
				fmt.Sprintf("Loại phòng %s phải có từ %d đến %d cột (hiện tại: %d)",
					input.Type, limit.MinColumns, limit.MaxColumns, input.Columns), nil, "columns")
		}

		// Kiểm tra VipColMin/Max chỉ được gửi khi có VIP (tùy chọn nâng cao)
		if input.VipColMin > 0 || input.VipColMax > 0 {
			if input.VipColMin < 1 || input.VipColMax > input.Columns || input.VipColMin > input.VipColMax {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Cột VIP không hợp lệ", nil, "vipColMin")
			}
		}
		if input.RoomNumber == 0 {
			var maxRoomNumber uint
			err := database.DB.Model(&model.Room{}).
				Where("cinema_id = ?", input.CinemaId).
				Select("COALESCE(MAX(room_number), 0) + 1").
				Scan(&maxRoomNumber).Error
			if err != nil {
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi tính số phòng tự động", err)
			}
			input.RoomNumber = maxRoomNumber
		} else {
			// Kiểm tra RoomNumber không trùng trong cùng Cinema nếu cung cấp thủ công
			var existingRoom model.Room
			if err := database.DB.Where("cinema_id = ? AND room_number = ? ", input.CinemaId, input.RoomNumber).First(&existingRoom).Error; err == nil {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Số phòng đã tồn tại trong rạp này", fmt.Errorf("roomNumber already exists"), "roomNumber")
			}
		}
		// Tạo tên phòng: Room <RoomNumber> - <Cinema.Name>
		roomName := fmt.Sprintf("Room %d - %s", input.RoomNumber, strings.TrimSpace(cinema.Name))

		// Kiểm tra RoomNumber không trùng trong cùng Cinema
		var existingRoom model.Room
		if err := database.DB.Where("cinema_id = ? AND room_number = ? AND deleted_at IS NULL", input.CinemaId, input.RoomNumber).First(&existingRoom).Error; err == nil {
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Số phòng đã tồn tại trong rạp này", fmt.Errorf("roomNumber already exists"), "roomNumber")
		}

		// Lưu input và roomName vào context
		c.Locals("inputCreateScreeningRoom", input)
		c.Locals("roomName", roomName)
		c.Locals("formats", formats)
		return c.Next()
	}
}

func EditRoom(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// param
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}

		// body parse
		var input model.EditRoomInput
		if err := c.BodyParser(&input); err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, fmt.Sprintf("Không thể phân tích yêu cầu: %s", err.Error()), err)
		}

		// validate struct (validator will check required tags etc)
		validate := validator.New()
		if err := validate.Struct(&input); err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, err.Error(), err)
		}

		db := database.DB

		// check room exists
		var room model.Room
		if err := db.First(&room, valueKey).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return utils.ErrorResponse(c, fiber.StatusNotFound, "Không tìm thấy phòng chiếu", nil)
			}
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi truy vấn cơ sở dữ liệu", err)
		}

		// auth
		accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin && !isManager {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
		}
		if isManager && (accountInfo.CinemaId == nil || *accountInfo.CinemaId != room.CinemaId) {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Không có quyền chỉnh sửa phòng này", nil)
		}

		// if CinemaId provided (pointer), check cinema exists (deref safely)
		var targetCinemaID uint = room.CinemaId
		if input.CinemaId != nil {
			targetCinemaID = *input.CinemaId
			var cinema model.Cinema
			if err := db.First(&cinema, targetCinemaID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Rạp không tồn tại", nil, "cinemaId")
				}
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi truy vấn rạp", err)
			}
		}

		// If RoomNumber provided, check duplicate (deref pointer)
		if input.RoomNumber != nil && *input.RoomNumber > 0 && *input.RoomNumber != room.RoomNumber {
			var existing model.Room
			if err := db.Where("cinema_id = ? AND room_number = ? AND id != ?", targetCinemaID, *input.RoomNumber, valueKey).
				First(&existing).Error; err == nil {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Số phòng đã tồn tại", nil, "roomNumber")
			}
		}

		// If FormatIds provided (pointer to slice), deref and validate
		var formatIDs []uint
		if input.FormatIds != nil && len(*input.FormatIds) > 0 {
			formatIDs = *input.FormatIds // deref into local slice (non-pointer)
			var count int64
			if err := db.Model(&model.Format{}).Where("id IN ?", formatIDs).Count(&count).Error; err != nil {
				return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi truy vấn định dạng", err)
			}
			if count != int64(len(formatIDs)) {
				return utils.ErrorResponse(c, fiber.StatusBadRequest, "Một số định dạng không tồn tại", nil)
			}
		}

		// Build room name (use provided roomNumber if given, else existing)
		roomNum := room.RoomNumber
		if input.RoomNumber != nil && *input.RoomNumber > 0 {
			roomNum = *input.RoomNumber
		}
		// get cinema name (targetCinemaID determined above)
		var cinema model.Cinema
		if err := db.First(&cinema, targetCinemaID).Error; err != nil {
			// should not happen because we checked earlier when provided, but guard anyway
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi truy vấn rạp", err)
		}
		if input.Type != nil {
			roomType := *input.Type // Dereference pointer

			// Kiểm tra Type có hợp lệ không
			validTypes := map[model.RoomType]bool{
				model.Small:  true,
				model.Medium: true,
				model.Large:  true,
				model.IMAX:   true,
				model.FourDX: true,
			}

			if !validTypes[roomType] {
				return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Loại phòng không hợp lệ", nil, "type")
			}

			// === Chỉ validate giới hạn nếu có thay đổi Row hoặc Columns ===
			if input.Seat != nil {
				var numRows int
				var columns int
				var roomType model.RoomType = room.Type // dùng Type hiện tại nếu không thay đổi

				if input.Type != nil {
					roomType = *input.Type
				}
				limit, exists := roomLimits[roomType]
				if !exists {
					return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Cấu hình giới hạn phòng bị thiếu", nil)
				}
				// Nếu gửi row/columns, validate chúng
				if input.Seat.Row != nil {
					numRows = len(*input.Seat.Row)
					if numRows < limit.MinRows || numRows > limit.MaxRows {
						return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest,
							fmt.Sprintf("Loại phòng %s phải có từ %d đến %d hàng ghế (hiện tại: %d)", roomType, limit.MinRows, limit.MaxRows, numRows),
							nil, "seat.row")
					}
				}
				if input.Seat.Columns != nil {
					columns = *input.Seat.Columns
					if columns < limit.MinColumns || columns > limit.MaxColumns {
						return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest,
							fmt.Sprintf("Loại phòng %s phải có từ %d đến %d cột (hiện tại: %d)", roomType, limit.MinColumns, limit.MaxColumns, columns),
							nil, "seat.columns")
					}
				}
				// Validate VIP nếu gửi (require cả min và max)
				if input.Seat.VipColMin != nil || input.Seat.VipColMax != nil {
					if input.Seat.VipColMin == nil || input.Seat.VipColMax == nil {
						return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Phải gửi cả vipColMin và vipColMax", nil, "seat.vipColMin")
					}
					min := *input.Seat.VipColMin
					max := *input.Seat.VipColMax

					// Lấy columns để check (nếu không gửi, dùng existing từ room/seats)
					if columns == 0 {
						if input.Seat.Columns != nil {
							columns = *input.Seat.Columns
						} else if len(room.Seats) > 0 {
							for _, s := range room.Seats {
								if s.Column > columns {
									columns = s.Column
								}
							}
						}
						if columns == 0 {
							return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Không thể xác định số cột cho VIP validation", nil, "seat.columns")
						}
					}

					if min < 3 || min > 5 || max < 9 || max > 14 || min > max || max > columns {
						return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest,
							"Cột VIP không hợp lệ (vipColMin: 3-5, vipColMax: 9-14, min ≤ max, max ≤ columns)", nil, "seat.vipColMin")
					}
				}
			}
		}
		roomName := fmt.Sprintf("Room %d - %s", roomNum, strings.TrimSpace(cinema.Name))

		// store into locals for handler
		c.Locals("editRoomInput", input)
		c.Locals("roomName", roomName)
		c.Locals("roomId", uint(valueKey)) // store as uint (match model IDs)
		c.Locals("formatIDs", formatIDs)   // []uint (non-pointer, safe for GORM)

		return c.Next()
	}
}
func DeleteRoom(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}
		_, isAdmin, _, _, _ := helper.GetInfoAccountFromToken(c)
		if !isAdmin {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("không có quyền vô hiệu hóa phòng"))
		}
		c.Locals("roomId", uint(valueKey))
		return c.Next()
	}
}
func DisableRoom(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin && !isManager {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("không có quyền vô hiệu hóa phòng"))
		}

		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}

		db := database.DB
		tx := db.Begin()

		// Lấy danh sách phòng hợp lệ
		var existingRooms model.Room
		if err := database.DB.First(&existingRooms, valueKey).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Không tìm thấy phòng chiếu",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Lỗi truy vấn cơ sở dữ liệu: %s", err.Error()),
			})
		}

		// Kiểm tra quyền của manager và trạng thái phòng

		if existingRooms.Status == constants.STATUS_ROOM_CANCEL {
			tx.Rollback()
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, fmt.Sprintf("Phòng %d đã bị khóa", existingRooms.ID), errors.New("room already canceled"), "ids")
		}

		if isManager && (accountInfo.CinemaId == nil || *accountInfo.CinemaId != existingRooms.CinemaId) {
			tx.Rollback()
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền vô hiệu hóa phòng của rạp khác", errors.New("manager not assigned to this cinema"))
		}

		// Kiểm tra lịch chiếu đang hoạt động
		var activeShowtimes int64
		if err := tx.Model(&model.Showtime{}).Where("room_id = ? AND start_time >= ?", valueKey, time.Now()).Count(&activeShowtimes).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi kiểm tra lịch chiếu", err)
		}
		if activeShowtimes > 0 {
			tx.Rollback()
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Không thể vô hiệu hóa phòng có lịch chiếu đang hoạt động", errors.New("active showtimes exist"), "ids")
		}

		// Truyền danh sách ID xuống handler
		c.Locals("roomId", uint(valueKey))
		return c.Next()
	}
}

func EnableRoom(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)

		if !isAdmin && !isManager {
			return utils.ErrorResponse(c, fiber.StatusForbidden, constants.NOT_ADMIN, errors.New("not admin"))
		}
		params := c.Params(key)
		valueKey, err := strconv.Atoi(params)
		if err != nil {
			return utils.ErrorResponse(c, fiber.StatusBadRequest, constants.DATA_INPUT_IS_NOT_NUMBER, errors.New("params invalid"))
		}
		db := database.DB
		tx := db.Begin()
		// Kiểm tra các phòng tồn tại và đang bị khóa
		var room model.Room
		if err := db.Where("id = ?  AND status = ?", valueKey, "cancel").First(&room).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Lỗi truy vấn cơ sở dữ liệu: %s", err.Error()),
			})
		}
		// Kiểm tra quyền của manager
		if isManager && (accountInfo.CinemaId == nil || *accountInfo.CinemaId != room.CinemaId) {
			tx.Rollback()
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền mở khóa phòng của rạp khác", errors.New("manager not assigned to this cinema"))
		}

		// Kiểm tra trạng thái
		if room.Status != "cancel" {
			tx.Rollback()
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, fmt.Sprintf("Phòng %d hiện không bị khóa", room.ID), errors.New("room not canceled"), "status")
		}

		c.Locals("roomId", uint(valueKey))
		return c.Next()

	}
}
