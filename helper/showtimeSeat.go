package helper

import (
	"cinema_manager/model"
	"errors"

	"gorm.io/gorm"
)

func CreateShowtimeSeats(tx *gorm.DB, showtimeId uint, roomId uint) error {
	var seats []model.Seat

	// Lấy danh sách ghế trong phòng
	if err := tx.Where("room_id = ?", roomId).Find(&seats).Error; err != nil {
		return err
	}

	if len(seats) == 0 {
		return errors.New("Room has no seats")
	}

	// Tạo danh sách showtime_seat
	var showtimeSeats []model.ShowtimeSeat

	for _, seat := range seats {
		showtimeSeats = append(showtimeSeats, model.ShowtimeSeat{
			ShowtimeId: showtimeId,
			SeatId:     seat.ID,
			SeatRow:    seat.Row,
			SeatNumber: seat.Column,
			SeatTypeId: seat.SeatTypeId,
			Status:     "AVAILABLE",
			HeldBy:     "",
			ExpiredAt:  nil,
		})
	}

	// Insert hàng loạt → nhanh hơn 100x
	return tx.Create(&showtimeSeats).Error
}
