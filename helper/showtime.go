package helper

import (
	"cinema_manager/database"
	"cinema_manager/model"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var scheduler *cron.Cron

func IndexInArray(arr []uint, value uint) int {
	for i, v := range arr {
		if v == value {
			return i
		}
	}
	return 0
}

func StartShowtimeScheduler() {
	scheduler = cron.New(cron.WithChain(
		cron.SkipIfStillRunning(cron.DefaultLogger),
	))

	// Chạy mỗi 5 phút (không cần mỗi phút)
	_, err := scheduler.AddFunc("*/5 * * * *", updateExpiredShowtimes)
	if err != nil {
		log.Printf("Lỗi khởi tạo scheduler: %v", err)
		return
	}

	scheduler.Start()
	log.Println("Scheduler suất chiếu đã khởi động (mỗi 5 phút)")
}

func updateExpiredShowtimes() {
	now := time.Now()
	result := database.DB.Model(&model.Showtime{}).
		Where("status = ? AND end_time < ?", "available", now).
		Update("status", "expired")

	if result.Error != nil {
		log.Printf("Lỗi cập nhật suất chiếu: %v", result.Error)
		return
	}

	if result.RowsAffected > 0 {
		log.Printf("Đã cập nhật %d suất chiếu thành 'expired'", result.RowsAffected)
	}
}

// Dừng scheduler khi tắt server
func StopShowtimeScheduler() {
	if scheduler != nil {
		scheduler.Stop()
		log.Println("Scheduler suất chiếu đã dừng")
	}
}

// FilterSlotsByFormat lọc danh sách khung giờ chiếu phù hợp với từng định dạng phim
func FilterSlotsByFormatAndTime(slots []string, format string, movieDuration int, currentDate time.Time, loc *time.Location) []string {
	var valid []string
	for _, slot := range slots {
		parts := strings.Split(slot, ":")
		if len(parts) != 2 {
			continue
		}
		hour, _ := strconv.Atoi(parts[0])
		minute, _ := strconv.Atoi(parts[1])

		// Tạo startTime giả để tính endTime
		startTime := time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), hour, minute, 0, 0, loc)
		endTime := startTime.Add(time.Minute * time.Duration(movieDuration))

		switch format {
		case "3D":
			// Trưa - tối: >=12:00 và <21:00 (bắt đầu trước 21:00)
			if hour >= 12 && hour < 21 {
				valid = append(valid, slot)
			}
		case "4DX":
			// Chiều - đêm: >=14:00
			if hour >= 14 {
				valid = append(valid, slot)
			}
		case "IMAX":
			// Tối - đêm, không quá 1h (giả định: bắt đầu >=18:00, kết thúc <=23:59 để tránh muộn)
			if hour >= 18 && endTime.Hour() < 24 { // hoặc endTime.Before(startTime.Add(24*time.Hour)) nhưng chặt hơn
				valid = append(valid, slot)
			}
		case "2D":
			// 2D: không giới hạn, trả hết
			valid = append(valid, slot)
		}
	}
	return valid
}

// formatTime chuẩn hóa lại khung giờ "15:04"
func FormatTime(hour, minute int) string {
	return time.Date(0, 0, 0, hour, minute, 0, 0, time.Local).Format("15:04")
}
