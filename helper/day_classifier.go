package helper

import (
	"cinema_manager/database"
	"cinema_manager/model"
	"time"
)

type DayInfo struct {
	Date           time.Time
	Weekday        time.Weekday
	IsWeekend      bool
	IsFriday       bool
	IsHoliday      bool
	IsLunarHoliday bool
	IsEarly        bool
	IsSpecial      bool
	DayTypes       []string
	HolidayName    string
}

func ClassifyDay(date time.Time, movieReleaseDate time.Time) *DayInfo {
	info := &DayInfo{
		Date:     date,
		Weekday:  date.Weekday(),
		DayTypes: []string{},
	}

	// 1. Xác định thứ trong tuần
	classifyWeekday(info)

	// 2. Kiểm tra ngày lễ dương lịch
	checkSolarHoliday(info)

	// 3. Kiểm tra Tết âm lịch
	checkLunarHoliday(info)

	// 4. Kiểm tra suất chiếu sớm
	checkEarlyShow(info, movieReleaseDate)

	// 5. (Tùy chọn) Kiểm tra sự kiện đặc biệt từ DB
	// checkSpecialEvent(info)

	return info
}

// === 1. Thứ trong tuần ===
func classifyWeekday(info *DayInfo) {
	switch info.Weekday {
	case time.Monday, time.Tuesday, time.Wednesday, time.Thursday:
		info.DayTypes = append(info.DayTypes, "weekday")
	case time.Friday:
		info.DayTypes = append(info.DayTypes, "friday", "weekday")
		info.IsFriday = true
	case time.Saturday:
		info.DayTypes = append(info.DayTypes, "saturday", "weekend")
		info.IsWeekend = true
	case time.Sunday:
		info.DayTypes = append(info.DayTypes, "sunday", "weekend")
		info.IsWeekend = true
	}
}

// === 2. Ngày lễ dương lịch ===
func checkSolarHoliday(info *DayInfo) {
	var holiday model.Holiday
	err := database.DB.
		Where("date = ? AND type = 'solar' AND is_recurring = true", info.Date.Format("2006-01-02")).
		Or("date = ? AND type = 'solar' AND is_recurring = false", info.Date.Format("2006-01-02")).
		First(&holiday).Error

	if err == nil {
		info.IsHoliday = true
		info.DayTypes = append(info.DayTypes, "holiday")
		info.HolidayName = holiday.Name
	}
}

// === 3. Tết âm lịch ===
func checkLunarHoliday(info *DayInfo) {
	year := info.Date.Year()
	if year < 1900 || year > 2100 {
		return
	}

	solar := info.Date
	_, lunarMonth, lunarDay, _ := SolarToLunar(solar)

	// Tết âm lịch
	if lunarMonth == 1 && (lunarDay == 1 || lunarDay == 2 || lunarDay == 3) {
		info.IsLunarHoliday = true
		info.DayTypes = append(info.DayTypes, "lunar_holiday")
		info.HolidayName = "Tết Âm lịch"
	}
}
func SolarToLunar(t time.Time) (lunarYear, lunarMonth, lunarDay int, isLeap bool) {
	lunarYear = t.Year()
	lunarMonth = int(t.Month())
	lunarDay = t.Day()

	// Tính offset từ 1900-01-31
	offset := (t.YearDay() - 31) + (t.Year()-1900)*365 + (t.Year()-1900)/4
	if t.Month() < 3 {
		offset -= (t.Year() - 1900) / 4
	}

	// Lấy dữ liệu năm âm
	info := lunarOffset[t.Year()-1900]
	leap := 0
	if info&0x0000F != 0 {
		leap = int(info & 0x0000F)
	}

	for m := 1; m <= 12; m++ {
		days := 29
		if info&0x10000 != 0 {
			days = 30
		}
		info <<= 1

		if offset < days {
			if leap > 0 && m > leap {
				lunarMonth = m
			} else if leap > 0 && m == leap+1 {
				isLeap = true
				lunarMonth = m - 1
			} else {
				lunarMonth = m
			}
			lunarDay = offset + 1
			return
		}
		offset -= days
	}
	return
}

// === 4. Suất chiếu sớm (7 ngày trước công chiếu) ===
func checkEarlyShow(info *DayInfo, releaseDate time.Time) {
	earlyStart := releaseDate.AddDate(0, 0, -7)
	earlyEnd := releaseDate.Add(-time.Second) // trước 00:00 ngày công chiếu

	if !info.Date.Before(earlyStart) && info.Date.Before(earlyEnd) {
		info.IsEarly = true
		info.DayTypes = append(info.DayTypes, "early")
	}
}
