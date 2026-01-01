package utils

import "reflect"

func IsValidValueOfConstant(role string, constantValues []string) bool {
	for _, r := range constantValues {
		if r == role {
			return true
		}
	}
	return false
}

func IsNumber(value interface{}) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return true
	default:
		return false
	}
}

func IsArray(a interface{}) bool {
	kind := reflect.TypeOf(a).Kind()
	return kind == reflect.Array || kind == reflect.Slice
}
