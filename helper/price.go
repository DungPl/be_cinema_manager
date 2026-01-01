package helper

import (
	"cinema_manager/model"
	"strings"
	"time"
)

func CalculatePrice(startTime time.Time, format string, date time.Time) float64 {
	hour := startTime.Hour()
	basePrice := 50000.0

	// Định dạng
	switch format {
	case "IMAX":
		basePrice += 20000
	case "4DX":
		basePrice += 10000
	case "3D":
		basePrice += 5000
	}

	// Giờ vàng (18h-22h)
	if hour >= 18 && hour < 22 {
		basePrice += 10000
	}

	// Cuối tuần
	if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		basePrice += 10000
	}

	return basePrice
}

// RoomSupportsFormat kiểm tra phòng có hỗ trợ định dạng formatName hay không
func RoomSupportsFormat(roomFormats []model.Format, formatName string) bool {
	for _, f := range roomFormats {
		if strings.EqualFold(f.Name, formatName) { // so sánh không phân biệt hoa thường
			return true
		}
	}
	return false
}
func ParseRoomFormats(formatStr string) []string {
	formatStr = strings.TrimSpace(formatStr)
	if formatStr == "" {
		return []string{}
	}
	parts := strings.Split(formatStr, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// Contains kiểm tra xem 1 phần tử có trong mảng hay không
func Contains(arr []string, val string) bool {
	for _, a := range arr {
		if strings.EqualFold(a, val) { // không phân biệt hoa thường
			return true
		}
	}
	return false
}

func CalculateDynamicPrice(startTime time.Time, format string, date time.Time, isVietnamese bool) float64 {
	base := 50000.0
	hour := startTime.Hour()

	// Định dạng
	switch format {
	case "IMAX":
		base += 20000
	case "4DX":
		base += 10000
	case "3D":
		base += 5000
	}

	// Giờ vàng
	if hour >= 18 && hour < 22 {
		base += 10000
		if isVietnamese {
			base += 10000 // ưu đãi thêm cho phim Việt
		}
	}

	// Cuối tuần
	if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		base += 20000
	}

	return base
}
