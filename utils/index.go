package utils

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func ErrorResponse(c *fiber.Ctx, status int, message string, err error) error {
	// return c.Status(status).JSON(fiber.Map{
	// 	"status":  "error",
	// 	"message": message,
	// 	"errors":  err.Error(),
	// })
	var errMsg interface{}
	if err != nil {
		errMsg = err.Error()
	} else {
		errMsg = nil
	}
	return c.Status(status).JSON(fiber.Map{
		"message": message,
		"error":   errMsg,
	})
}

func ErrorResponseHaveKey(c *fiber.Ctx, status int, message string, err error, keyError string) error {
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	} else {
		errMsg = ""
	}
	return c.Status(status).JSON(fiber.Map{
		"status":   "error",
		"message":  message,
		"errors":   errMsg,
		"keyError": keyError,
	})
}

// getFirstValue lấy giá trị đầu tiên từ slice, nếu rỗng thì trả về ""
func GetFirstValue(values map[string][]string, key string) string {
	if v, ok := values[key]; ok && len(v) > 0 {
		return v[0]
	}
	return ""
}
func StringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func SuccessResponse(c *fiber.Ctx, status int, data any) error {
	return c.Status(status).JSON(fiber.Map{
		"status": "success",
		"data":   data,
	})
}

func ApplyPagination(query *gorm.DB, limit, page *int) *gorm.DB {
	// Kiểm tra nếu có limit thì thêm điều kiện Limit
	if limit != nil && *limit > 0 && page != nil && *page >= 1 {
		query = query.Limit(*limit)
		offset := *limit * (*page - 1)
		query = query.Offset(offset)
	}

	return query
}
func IsValidMMYYYY(dateStr string) bool {
	// Kiểm tra độ dài chuỗi
	if len(dateStr) != 7 {
		return false
	}
	// Kiểm tra định dạng MM-YYYY
	parts := strings.Split(dateStr, "-")
	if len(parts) != 2 {
		return false
	}

	// Chuyển đổi tháng và năm thành số nguyên
	month, err := strconv.Atoi(parts[0])
	if err != nil || month < 1 || month > 12 {
		return false
	}
	year, err := strconv.Atoi(parts[1])
	if err != nil || year < 1900 || year > 9999 { // Đặt một khoảng hợp lý cho năm
		return false
	}
	//Kiểm tra xem ngày có hợp lệ không
	_, err = time.Parse("01-2006", dateStr)
	if err != nil {
		return false
	}
	return true
}
func CalculateGrowth(today, yesterday float64) float64 {
	if yesterday == 0 {
		if today == 0 {
			return 0
		}
		return 100 // từ 0 lên >0
	}
	return ((today - yesterday) / yesterday) * 100
}
func Ptr[T any](v T) *T {
	return &v
}
