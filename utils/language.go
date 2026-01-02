package utils

type LanguageType string

const (
	VISub LanguageType = "VI_SUB" // Phụ đề Việt
	VIDub LanguageType = "VI_DUB" // Lồng tiếng Việt
	ENSub LanguageType = "EN_SUB" // Phụ đề Anh
	ENDub LanguageType = "EN_DUB" // Lồng tiếng Anh
)

// Map từ LanguageType sang label tiếng Việt đẹp
var languageLabels = map[LanguageType]string{
	VISub: "Phụ đề Việt",
	VIDub: "Lồng tiếng Việt",
	ENSub: "Phụ đề Anh",
	ENDub: "Lồng tiếng Anh",
}

// Hàm getLanguageLabel
func GetLanguageLabel(langType LanguageType) string {
	if label, ok := languageLabels[langType]; ok {
		return label
	}
	// Nếu không tìm thấy (ví dụ: null hoặc giá trị lạ)
	return "Không xác định"
}
