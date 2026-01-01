package helper

import (
	"fmt"
	"regexp"
	"strings"
)

// Chuẩn hóa tên rạp: loại bỏ tên hãng thừa ở phần sau
func NormalizeCinemaName(chainName, inputName string) string {
	chainName = strings.TrimSpace(chainName)
	inputName = strings.TrimSpace(inputName)

	if inputName == "" {
		return chainName // fallback
	}

	// Tạo regex để tìm và loại bỏ chainName ở đầu phần inputName (không phân biệt hoa thường)
	re := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(chainName) + `\s*`)
	cleaned := re.ReplaceAllString(inputName, "")

	// Nếu sau khi loại bỏ còn lại rỗng → chỉ dùng inputName gốc
	if strings.TrimSpace(cleaned) == "" {
		return fmt.Sprintf("%s %s", chainName, inputName)
	}

	return fmt.Sprintf("%s %s", chainName, cleaned)
}
