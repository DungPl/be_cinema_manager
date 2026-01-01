package handler

import (
	"cinema_manager/constants"
	"cinema_manager/database"
	"cinema_manager/helper"
	"cinema_manager/model"
	"cinema_manager/utils"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func GetRoom(c *fiber.Ctx) error {

	accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)

	if !isAdmin && !isManager {
		return utils.ErrorResponse(c, fiber.StatusForbidden, constants.CAN_NOT_EDIT_CINEMA, errors.New("not permission"))
	}

	// Lấy cinemaId từ param
	cinemaId, err := strconv.ParseUint(c.Params("cinemaId"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ID rạp chiếu phim không hợp lệ",
		})
	}
	// Nếu là manager thì chỉ được xem các phòng thuộc rạp của mình
	if isManager {
		if accountInfo.CinemaId == nil || uint64(*accountInfo.CinemaId) != cinemaId {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền xem phòng của rạp khác", errors.New("cinema permission denied"))
		}
	}

	var rooms []model.Room
	if err := database.DB.
		Where("cinema_id = ? ", cinemaId).
		Preload("Cinema").
		Preload("Cinema.Chain").Find(&rooms).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể lấy danh sách phòng chiếu: %s", err.Error()),
		})
	}
	return utils.SuccessResponse(c, fiber.StatusOK, rooms)

}
func GetRoomsByCinemaId(c *fiber.Ctx) error {
	param := c.Params("cinemaId")
	id, err := strconv.Atoi(param)
	cinemaId := uint(id)
	accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)

	// Chỉ admin hoặc manager được phép xem chi tiết phòng
	if !isAdmin && !isManager {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền xem chi tiết phòng", errors.New("not permission"))
	}
	if isManager {
		if accountInfo.CinemaId == nil {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Manager không được gán rạp chiếu", errors.New("manager has no assigned cinema"))
		}

		if *accountInfo.CinemaId != cinemaId {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền xem chi tiết phòng của rạp khác", errors.New("manager not assigned to this cinema"))
		}
	}
	var rooms []model.Room
	err = database.DB.
		Preload("Cinema").
		Preload("Cinema.Chain").
		Preload("Seats").
		Preload("Seats.SeatType").
		Preload("Formats").
		Where("cinema_id = ?", cinemaId).
		Order("name ASC").
		Find(&rooms).Error
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Failed to fetch rooms", err)
	}
	return utils.SuccessResponse(c, fiber.StatusOK, rooms)

}
func GetRoomById(c *fiber.Ctx) error {
	accountInfo, isAdmin, isManager, _, _ := helper.GetInfoAccountFromToken(c)

	// Chỉ admin hoặc manager được phép xem chi tiết phòng
	if !isAdmin && !isManager {
		return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền xem chi tiết phòng", errors.New("not permission"))
	}

	// Lấy roomId từ param
	roomIdStr := c.Params("roomId")
	roomId, err := strconv.ParseUint(roomIdStr, 10, 64)
	if err != nil {
		return utils.ErrorResponse(c, fiber.StatusBadRequest, "ID chuỗi rạp không hợp lệ", err)
	}

	db := database.DB
	var room model.Room

	// Lấy chi tiết phòng cùng thông tin rạp và chuỗi rạp
	if err := db.
		Preload("Cinema").
		Preload("Cinema.Chain").
		Preload("Seats").
		First(&room, roomId).Error; err != nil {
		// ✅ Nếu không tìm thấy → trả lỗi
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Không tìm thấy phòng chiếu", err)
		}

		// ✅ Nếu có lỗi khác → lỗi server
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi khi truy vấn dữ liệu phòng chiếu", err)
	}

	if isManager {
		if accountInfo.CinemaId == nil {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Manager không được gán rạp chiếu", errors.New("manager has no assigned cinema"))
		}

		if uint(*accountInfo.CinemaId) != room.CinemaId {
			return utils.ErrorResponse(c, fiber.StatusForbidden, "Bạn không có quyền xem chi tiết phòng của rạp khác", errors.New("manager not assigned to this cinema"))
		}
	}
	return utils.SuccessResponse(c, fiber.StatusOK, room)
}

func CreateRoom(c *fiber.Ctx) error {
	db := database.DB
	roomInput, ok := c.Locals("inputCreateScreeningRoom").(model.CreateScreeningRoomInput)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	roomName, ok := c.Locals("roomName").(string)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	formats, ok := c.Locals("formats").([]model.Format)
	if !ok {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_PARSE_DATA_TO_LOCALS, errors.New("PARSE DATA TO LOCALS FAIL"))
	}
	tx := db.Begin()
	newRoom := &model.Room{
		Name:          roomName,
		RoomNumber:    roomInput.RoomNumber,
		CinemaId:      roomInput.CinemaId,
		Status:        "available",
		Row:           roomInput.Row,
		Type:          roomInput.Type,
		Formats:       formats,
		HasCoupleSeat: roomInput.HasCoupleSeat,
	}

	if err := tx.Create(newRoom).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Tạo phòng thất bại", err)
	}

	// Gắn định dạng vào phòng chiếu
	// if err := tx.Model(&newRoom).Association("Formats").Append(formats); err != nil {
	// 	tx.Rollback()
	// 	return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Không thể gắn định dạng cho phòng chiếu", err)
	// }
	// Xác định hàng cuối VIP và hàng COUPLE
	rows := roomInput.Row
	hasK := strings.Contains(rows, "K")
	lastVipRow := "H"
	if hasK {
		lastVipRow = "I"
	}
	lastRow := string(rows[len(rows)-1]) // Hàng cuối (I hoặc K)
	// Lấy ID của các SeatType
	var normalType, vipType, coupleType model.SeatType
	if err := tx.Where("type = ?", "NORMAL").First(&normalType).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không tìm thấy loại ghế NORMAL",
		})
	}
	if err := tx.Where("type = ?", "VIP").First(&vipType).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không tìm thấy loại ghế VIP",
		})
	}
	if err := tx.Where("type = ?", "COUPLE").First(&coupleType).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không tìm thấy loại ghế COUPLE",
		})
	}
	var lastCol int
	if roomInput.Columns%2 == 0 {
		lastCol = roomInput.Columns - 2
	} else {
		lastCol = roomInput.Columns - 1
	}
	// Tạo SeatLayout
	//tx := db.Begin()
	seatsToCreate := []model.Seat{}
	for _, rowLabel := range rows {
		rowStr := string(rowLabel)

		columns := roomInput.Columns
		if rowStr == lastRow && roomInput.HasCoupleSeat {
			columns = lastCol
		}

		// ===== GHẾ COUPLE (CHỈ KHI BẬT FLAG) =====
		if rowStr == lastRow && roomInput.HasCoupleSeat {
			for col := 1; col <= columns; col += 2 {
				coupleId := utils.Ptr(uint(uuid.New().ID()))

				seatsToCreate = append(seatsToCreate,
					model.Seat{
						RoomId:      newRoom.ID,
						Row:         rowStr,
						Column:      col,
						SeatTypeId:  coupleType.ID,
						CoupleId:    coupleId,
						IsAvailable: true,
					},
					model.Seat{
						RoomId:      newRoom.ID,
						Row:         rowStr,
						Column:      col + 1,
						SeatTypeId:  coupleType.ID,
						CoupleId:    coupleId,
						IsAvailable: true,
					},
				)
			}
			continue
		}

		// ===== GHẾ NORMAL / VIP =====
		for col := 1; col <= columns; col++ {
			seatTypeId := normalType.ID

			if rowStr >= "D" && rowStr <= lastVipRow && col >= 3 && col <= 12 {
				seatTypeId = vipType.ID
			}

			seatsToCreate = append(seatsToCreate, model.Seat{
				RoomId:      newRoom.ID,
				Row:         rowStr,
				Column:      col,
				SeatTypeId:  seatTypeId,
				IsAvailable: true,
			})
		}
	}

	// Tạo hàng loạt ghế (tối ưu hơn)
	if len(seatsToCreate) > 0 {
		if err := tx.Create(&seatsToCreate).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Tạo ghế thất bại", err)
		}
	}

	// Cập nhật Capacity
	newRoom.Capacity = utils.Ptr(len(seatsToCreate))
	if err := tx.Save(newRoom).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Cập nhật Capacity thất bại", err)
	}
	var createdRoom model.Room
	if err := tx.
		Preload("Cinema").
		Preload("Formats").
		Preload("Cinema.Chain").
		Preload("Seats").
		Preload("Seats.SeatType").
		First(&createdRoom, newRoom.ID).Error; err != nil { // Dùng ID thay vì name + cinema_id
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Tải thông tin phòng thất bại", err)
	}

	// BÂY GIỜ MỚI COMMIT
	if err := tx.Commit().Error; err != nil {
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Commit thất bại", err)
	}

	// Trả về phòng đã preload đầy đủ
	return utils.SuccessResponse(c, fiber.StatusCreated, createdRoom)
}

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

func EditRoom(c *fiber.Ctx) error {
	db := database.DB
	input, ok := c.Locals("editRoomInput").(model.EditRoomInput)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy dữ liệu đầu vào",
		})
	}
	roomName, ok := c.Locals("roomName").(string)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy tên phòng",
		})
	}
	roomId, ok := c.Locals("roomId").(uint)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy ID phòng",
		})
	}
	formatIDs := c.Locals("formatIDs").([]uint)
	tx := db.Begin()
	var room model.Room
	if err := tx.Preload("Formats").Preload("Seats").First(&room, roomId).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return utils.ErrorResponse(c, fiber.StatusNotFound, "Phòng không tồn tại", nil)
		}
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi DB", err)
	}
	if input.RoomNumber != nil {
		if *input.RoomNumber > 0 {
			room.RoomNumber = *input.RoomNumber
		}

	}
	if input.CinemaId != nil {
		var cinema model.Cinema
		if err := tx.First(&cinema, *input.CinemaId).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Rạp chiếu phim không tồn tại", err, "cinemaId")
		}
		room.CinemaId = *input.CinemaId
	}
	if input.Type != nil {
		newType := *input.Type

		// Kiểm tra Type hợp lệ
		validTypes := map[model.RoomType]bool{
			model.Small: true, model.Medium: true, model.Large: true,
			model.IMAX: true, model.FourDX: true,
		}
		if !validTypes[newType] {
			tx.Rollback()
			return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Loại phòng không hợp lệ", nil, "type")
		}

		room.Type = newType
	}
	room.Name = roomName

	if len(formatIDs) > 0 {
		// Xóa cũ
		if err := tx.Where("room_id = ?", room.ID).Delete(&model.RoomFormat{}).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Lỗi xóa định dạng", err)
		}
		room.Formats = nil
		// Thêm mới
		for _, fid := range formatIDs {
			if err := tx.Create(&model.RoomFormat{RoomID: room.ID, FormatID: fid}).Error; err != nil {
				tx.Rollback()
				return utils.ErrorResponse(c, 500, "Lỗi thêm định dạng", err)
			}
		}
	}
	if input.Seat != nil && input.Seat.HasCoupleSeat != nil {
		room.HasCoupleSeat = *input.Seat.HasCoupleSeat
	}
	// -------------------- CẬP NHẬT GHẾ --------------------
	updateSeats := input.Seat != nil && (input.Seat.Row != nil || input.Seat.Columns != nil || input.Seat.VipColMin != nil || input.Seat.VipColMax != nil || input.Seat.HasCoupleSeat != nil)
	if updateSeats {
		// Lấy row và column hiện tại
		currentRow := room.Row
		if input.Seat.Row != nil {
			currentRow = *input.Seat.Row
		}

		currentColumns := 0
		if input.Seat.Columns != nil {
			currentColumns = *input.Seat.Columns
		} else if len(room.Seats) > 0 {
			for _, s := range room.Seats {
				if s.Column > currentColumns {
					currentColumns = s.Column
				}
			}
		}
		if currentColumns == 0 {
			tx.Rollback()
			return utils.ErrorResponse(c, fiber.StatusBadRequest, "Số cột không hợp lệ", nil)
		}

		// Kiểm tra có vé đã đặt không
		var bookedCount int64
		if err := tx.Model(&model.Ticket{}).
			Where("seat_id IN (SELECT id FROM seats WHERE room_id = ?)", room.ID).
			Count(&bookedCount).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Lỗi kiểm tra vé đã đặt", err)
		}
		if bookedCount > 0 {
			tx.Rollback()
			return utils.ErrorResponseHaveKey(c, 400, "Không thể sửa ghế: đã có khách đặt vé", nil, "seat")
		}

		// --- XÓA DỮ LIỆU CŨ ---
		// 1. Xóa showtime_seats
		if err := tx.Exec("DELETE FROM showtime_seats WHERE seat_id IN (SELECT id FROM seats WHERE room_id = ?)", room.ID).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Xóa showtime_seats thất bại", err)
		}

		if err := tx.Unscoped().Where("seat_id IN (SELECT id FROM seats WHERE room_id = ?)", room.ID).Delete(&model.Ticket{}).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Xóa tickets thất bại", err)
		}

		if err := tx.Where("room_id = ?", room.ID).Delete(&model.Seat{}).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Xóa seats cũ thất bại", err)
		}

		// Kiểm tra còn ghế trong tx
		var remaining int64
		tx.Model(&model.Seat{}).Where("room_id = ?", room.ID).Count(&remaining)
		log.Printf("Seats remaining after delete: %d", remaining)
		if remaining > 0 {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Xóa ghế cũ không thành công, còn ghế trong DB", nil)
		}
		room.Seats = nil
		// --- TẠO GHẾ MỚI ---
		var normalType, vipType, coupleType model.SeatType
		for _, t := range []string{"NORMAL", "VIP", "COUPLE"} {
			var st model.SeatType
			if err := tx.Where("type = ?", t).First(&st).Error; err != nil {
				tx.Rollback()
				return utils.ErrorResponse(c, 500, fmt.Sprintf("Không tìm thấy loại ghế %s", t), err)
			}
			switch t {
			case "NORMAL":
				normalType = st
			case "VIP":
				vipType = st
			case "COUPLE":
				coupleType = st
			}
		}

		vipMin, vipMax := 3, 12
		if input.Seat.VipColMin != nil {
			vipMin = *input.Seat.VipColMin
		}
		if input.Seat.VipColMax != nil {
			vipMax = *input.Seat.VipColMax
		}
		if vipMin > vipMax || vipMin < 1 || vipMax > currentColumns {
			tx.Rollback()
			return utils.ErrorResponseHaveKey(c, 400, "Phạm vi VIP không hợp lệ", nil, "seat.vipColMin")
		}

		seats := []model.Seat{}
		lastRow := string(currentRow[len(currentRow)-1])
		lastCol := currentColumns
		if currentColumns%2 == 0 {
			lastCol = currentColumns - 2
		} else {
			lastCol = currentColumns - 1
		}
		lastVipRow := "H"
		if strings.Contains(currentRow, "K") {
			lastVipRow = "I"
		}

		for _, r := range currentRow {
			rowLabel := string(r)

			colCount := currentColumns
			if rowLabel == lastRow && room.HasCoupleSeat {
				colCount = lastCol
			}

			// ===== GHẾ COUPLE =====
			if rowLabel == lastRow && room.HasCoupleSeat {
				for col := 1; col <= colCount; col += 2 {
					coupleId := utils.Ptr(uint(uuid.New().ID()))

					seats = append(seats,
						model.Seat{
							RoomId:      room.ID,
							Row:         rowLabel,
							Column:      col,
							SeatTypeId:  coupleType.ID,
							CoupleId:    coupleId,
							IsAvailable: true,
						},
						model.Seat{
							RoomId:      room.ID,
							Row:         rowLabel,
							Column:      col + 1,
							SeatTypeId:  coupleType.ID,
							CoupleId:    coupleId,
							IsAvailable: true,
						},
					)
				}
				continue
			}

			// ===== GHẾ NORMAL / VIP =====
			for col := 1; col <= colCount; col++ {
				seatType := normalType.ID

				if rowLabel >= "D" && rowLabel <= lastVipRow && col >= vipMin && col <= vipMax {
					seatType = vipType.ID
				}

				seats = append(seats, model.Seat{
					RoomId:      room.ID,
					Row:         rowLabel,
					Column:      col,
					SeatTypeId:  seatType,
					IsAvailable: true,
				})
			}
		}

		if err := tx.Create(&seats).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Tạo ghế mới thất bại", err)
		}
		var showtimes []model.Showtime
		if err := tx.Where("room_id = ?", room.ID).Find(&showtimes).Error; err != nil {
			tx.Rollback()
			return utils.ErrorResponse(c, 500, "Lỗi tìm suất chiếu của phòng", err)
		}

		for _, st := range showtimes {
			for _, seat := range seats {
				showtimeSeat := model.ShowtimeSeat{
					ShowtimeId: st.ID,
					SeatId:     seat.ID,
					SeatRow:    seat.Row,
					SeatNumber: seat.Column,
					SeatTypeId: seat.SeatTypeId,
					Status:     "AVAILABLE",
					HeldBy:     "",
					ExpiredAt:  nil, // Mặc định AVAILABLE
				}
				if err := tx.Create(&showtimeSeat).Error; err != nil {
					tx.Rollback()
					return utils.ErrorResponse(c, 500, "Tạo showtime_seat thất bại", err)
				}
			}
		}
		// Cập nhật room info
		room.Row = currentRow
		room.Capacity = utils.Ptr(len(seats))
	}
	// Preload trong transaction
	if err := tx.Save(&room).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, constants.ERROR_EDIT, err)
	}
	var updatedRoom model.Room
	if err := tx. // ← FIX Ở ĐÂY: Bỏ NewDB: true
			Preload("Cinema").
			Preload("Cinema.Chain").
			Preload("Formats").
			Preload("Seats").
			Preload("Seats.SeatType").
			First(&updatedRoom, room.ID).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, 500, "Tải thông tin phòng thất bại", err)
	}
	tx.Commit()
	if tx.Error != nil { // Thêm check commit
		log.Printf("Commit thất bại: %v", tx.Error)
		return utils.ErrorResponse(c, 500, "Commit transaction thất bại", tx.Error)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, updatedRoom)
}
func DeleteRoom(c *fiber.Ctx) error {
	db := database.DB
	arrayId := c.Locals("deleteIds").(model.ArrayId)
	ids := arrayId.IDs

	// Bắt đầu transaction để đảm bảo toàn vẹn dữ liệu
	tx := db.Begin()

	// 1. Kiểm tra có suất chiếu đang hoạt động hoặc trong tương lai không
	var activeShowtimes int64
	if err := tx.Model(&model.Showtime{}).
		Where("room_id in ? AND end_time >= ?", ids, time.Now()).
		Count(&activeShowtimes).Error; err != nil {
		tx.Rollback()
		log.Printf("DeleteRoom: Failed to check showtimes - roomId=%v, error=%v", ids, err)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi kiểm tra lịch chiếu", err)
	}

	if activeShowtimes > 0 {
		tx.Rollback()
		return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest,
			"Không thể xóa phòng vì đang có suất chiếu đang chạy hoặc sắp diễn ra",
			errors.New("active showtimes exist"), "showtimes")
	}

	// 2. Kiểm tra có ghế nào đang được đặt không (trong bảng booking hoặc seat tạm giữ)
	var bookedSeats int64
	if err := tx.Model(&model.Seat{}).
		Where("room_id in  ? AND (is_booked = ? OR is_available = ?)", ids, true, false).
		Count(&bookedSeats).Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi kiểm tra ghế đã đặt", err)
	}

	if bookedSeats > 0 {
		tx.Rollback()
		return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest,
			"Không thể xóa phòng vì có ghế đang được khách đặt hoặc giữ chỗ",
			errors.New("booked seats exist"), "seats")
	}

	// 3. XÓA THẬT SỰ các bảng liên quan (tùy cấu trúc DB của bạn)
	// Lưu ý: Thứ tự xóa quan trọng để tránh lỗi foreign key

	// Xóa ghế của phòng (nếu có bảng Seat riêng theo room)
	if err := tx.Where("room_id in ?", ids).Delete(&model.Seat{}).Error; err != nil {
		tx.Rollback()
		log.Printf("DeleteRoom: Failed to delete seats - roomId=%v, error=%v", ids, err)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi xóa dữ liệu ghế", err)
	}

	// Xóa phòng (cuối cùng)
	if err := tx.Where("id in  ?", ids).Delete(&model.Room{}).Error; err != nil {
		tx.Rollback()
		log.Printf("DeleteRoom: Failed to delete room - id=%v, error=%v", ids, err)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Không thể xóa phòng chiếu", err)
	}

	// Commit nếu mọi thứ OK
	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Lỗi commit giao dịch: " + err.Error(),
		})
	}

	// Log hành động quan trọng
	log.Printf("ADMIN DELETE ROOM SUCCESS: roomId=%d by user=%s", ids, c.Locals("user_email"))

	return utils.SuccessResponse(c, fiber.StatusOK, fiber.Map{
		"message": "Xóa phòng chiếu thành công",
		"roomId":  ids,
		"deleted": true,
	})
}
func DisableRoom(c *fiber.Ctx) error {
	db := database.DB
	roomId, ok := c.Locals("roomId").(uint)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy danh sách ID phòng",
		})
	}
	tx := db.Begin()
	// Check active showtimes
	var activeShowtimes int64
	if err := tx.Model(&model.Showtime{}).Where("room_id = ? AND start_time >= ?", roomId, time.Now()).Count(&activeShowtimes).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to check active showtimes: ids=%v, error=%v", roomId, err)
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Lỗi kiểm tra lịch chiếu", err)
	}
	if activeShowtimes > 0 {
		tx.Rollback()
		log.Printf("Active showtimes exist for rooms: ids=%v, count=%d", roomId, activeShowtimes)
		return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Không thể vô hiệu hóa phòng có lịch chiếu đang hoạt động", errors.New("active showtimes exist"), "ids")
	}
	var bookedSeats int64
	if err := tx.Model(&model.Seat{}).Where("room_id = ? AND is_available = ?", roomId, false).Count(&bookedSeats).Error; err != nil {
		return err
	}
	if bookedSeats > 0 {
		return utils.ErrorResponseHaveKey(c, fiber.StatusBadRequest, "Không thể cập nhật vì có ghế đã đặt", fmt.Errorf("booked seats exist"), "seats")
	}

	if err := db.Model(&model.Room{}).
		Where("id = ? AND status != ?", roomId, "maintenance").
		Update("status", "cancel").Error; err != nil {
		tx.Rollback()
		return utils.ErrorResponse(c, fiber.StatusInternalServerError, "Không thể cập nhật trạng thái phòng", err)
	}
	// Commit giao dịch
	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể commit giao dịch: %s", err.Error()),
		})
	}
	return utils.SuccessResponse(c, fiber.StatusOK, roomId)
}
func EnableRoom(c *fiber.Ctx) error {
	db := database.DB
	roomId, ok := c.Locals("roomId").(uint)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Không thể lấy danh sách ID phòng",
		})
	}

	tx := db.Begin()
	if err := tx.Model(&model.Room{}).
		Where("id = ? AND status = ?", roomId, "cancel").
		Update("status", "available").Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể khôi phục phòng: %s", err.Error()),
		})
	}

	// Commit giao dịch
	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Không thể commit giao dịch: %s", err.Error()),
		})
	}
	return utils.SuccessResponse(c, fiber.StatusOK, roomId)
}
func GetFormats(c *fiber.Ctx) error {
	var formats []model.Format
	db := database.DB

	if err := db.Find(&formats).Error; err != nil {
		return utils.ErrorResponse(c, 500, "Không thể lấy danh sách định dạng", err)
	}

	return utils.SuccessResponse(c, fiber.StatusOK, formats)
}
