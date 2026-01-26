package helper

import (
	"cinema_manager/database"

	"cinema_manager/model"
	"fmt"
	"time"

	"gorm.io/gorm"
)

func SuggestTemplate(movie *model.Movie, startDate, endDate time.Time, isVietnamese *bool) *model.ScheduleTemplate {
	isWeekend := startDate.Weekday() >= time.Friday
	viet := isVietnamese != nil && *isVietnamese

	var templates []model.ScheduleTemplate
	query := database.DB.Where("deleted_at IS NULL")

	if viet {
		query = query.Where("movie_types @> ?", `["vietnamese"]`)
	} else if movie.Genre == "Action" || movie.Genre == "Sci-Fi" {
		query = query.Where("movie_types @> ?", `["blockbuster"]`)
	}

	if isWeekend {
		query = query.Where("day_types @> ?", `["weekend"]`)
	}

	query.Order("priority DESC").Limit(1).Find(&templates)
	if len(templates) > 0 {
		return &templates[0]
	}
	return nil
}

func ApplyTemplateToInput(input *model.AutoGenerateScheduleInput, t *model.ScheduleTemplate) {
	input.TimeSlots = t.TimeSlots
	input.Formats = t.Formats
	if len(input.RoomIDs) > t.MaxRooms {
		input.RoomIDs = input.RoomIDs[:t.MaxRooms]
	}
}

func GetValidFormats(roomIDs []uint, formats []string) []string {
	valid := []string{}
	for _, f := range formats {
		supported := true
		for _, roomID := range roomIDs {
			var room model.Room
			if err := database.DB.Preload("Formats").First(&room, roomID).Error; err != nil {
				continue
			}
			found := false
			for _, rf := range room.Formats {
				if rf.Name == f {
					found = true
					break
				}
			}
			if !found {
				supported = false
				break
			}
		}
		if supported {
			valid = append(valid, f)
		}
	}
	return valid
}
func IsValidShowDate(date time.Time, movie *model.Movie) bool {
	release := movie.DateRelease.Time
	if date.Format("2006-01-02") < release.Format("2006-01-02") {
		return false
	}
	if movie.DateEnd != nil && date.Format("2006-01-02") > movie.DateEnd.Time.Format("2006-01-02") {
		return false
	}
	return true
}

func GetMaxPerDay(format string, t *model.ScheduleTemplate) int {
	if t != nil {
		return t.MaxPerDay
	}
	switch format {
	case "3D":
		return 4
	case "4DX", "IMAX":
		return 3
	default:
		return 8
	}
}

const (
	MinGapBetweenRooms = 75 * time.Minute
	BreakStartHour     = 11
	BreakEndHour       = 14
	ExtraBreakTime     = 30 * time.Minute
)

func BuildStartTime(date time.Time, slot string, loc *time.Location, format string, roomIDs []uint, roomID uint) time.Time {
	startStr := fmt.Sprintf("%s %s", date.Format("2006-01-02"), slot)
	startTime, _ := time.ParseInLocation("2006-01-02 15:04", startStr, loc)

	// if startTime.Hour() >= BreakStartHour && startTime.Hour() < BreakEndHour {
	// 	startTime = time.Date(date.Year(), date.Month(), date.Day(), BreakEndHour, 0, 0, 0, loc).Add(ExtraBreakTime)
	// }

	idx := IndexInArray(roomIDs, roomID)
	offset := 15 * time.Minute
	if format == "4DX" || format == "IMAX" {
		offset = 20 * time.Minute
	}
	return startTime.Add(time.Duration(idx) * offset)
}

func HasConflict(tx *gorm.DB, roomID uint, start, end time.Time) bool {
	var count int64
	tx.Model(&model.Showtime{}).
		Where("room_id = ? AND (start_time < ? AND end_time > ?)", roomID, end, start).
		Count(&count)
	return count > 0
}
func HasInterRoomGapConflict(
	tx *gorm.DB,
	movieID uint,
	roomID uint,
	start time.Time,
	gap time.Duration,
) bool {
	var count int64

	tx.Model(&model.Showtime{}).
		Where(`
			movie_id = ?
			AND room_id != ?
			AND start_time < ?
			AND (? - start_time) < ?
		`,
			movieID,
			roomID,
			start,
			start,
			gap,
		).
		Count(&count)

	return count > 0
}

func HasNearbyConflict(tx *gorm.DB, cinemaID, movieID, roomID uint, start time.Time, gap time.Duration) bool {
	var count int64
	tx.Model(&model.Showtime{}).
		Joins("JOIN rooms r ON r.id = showtimes.room_id").
		Where("r.cinema_id= ? AND movie_id = ? AND room_id != ? AND ABS(EXTRACT(EPOCH FROM (start_time - ?)) / 60) < ?", cinemaID, movieID, roomID, start, int(gap.Minutes())).
		Count(&count)
	return count > 0
}
