package helper

import (
	"cinema_manager/model"
	"strconv"
	"strings"
)

type FormatConfig struct {
	MaxPerDay     int
	StartHour     int // 18 → 18:00
	EndHour       int // 22 → 22:00
	OffsetMinutes int // offset giữa các phòng
}

var SpecialFormatConfig = map[string]FormatConfig{
	"3D": {
		MaxPerDay:     4,
		StartHour:     18,
		EndHour:       22,
		OffsetMinutes: 15,
	},
	"4DX": {
		MaxPerDay:     3,
		StartHour:     18,
		EndHour:       22,
		OffsetMinutes: 20,
	},
	"IMAX": {
		MaxPerDay:     3,
		StartHour:     19,
		EndHour:       23,
		OffsetMinutes: 20,
	},
}

func GetFormatConfig(format string, template *model.ScheduleTemplate) FormatConfig {
	if config, exists := SpecialFormatConfig[format]; exists {
		return config
	}
	// Mặc định cho 2D
	return FormatConfig{
		MaxPerDay:     8,
		StartHour:     10,
		EndHour:       23,
		OffsetMinutes: 15,
	}
}
func ParseHour(timeStr string) int {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0
	}
	hour, _ := strconv.Atoi(parts[0])
	return hour
}
