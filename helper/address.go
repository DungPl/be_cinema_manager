package helper

import (
	"cinema_manager/model"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func normalizeVN(s string) string {
	s = strings.TrimSpace(s)
	replacements := map[string]string{
		"Đ.":  "Đường ",
		"P.":  "Phường ",
		"Q.":  "Quận ",
		"TP.": "Thành phố ",
	}
	for k, v := range replacements {
		s = strings.ReplaceAll(s, k, v)
	}
	// ❌ bỏ mã bưu điện
	s = strings.TrimFunc(s, func(r rune) bool {
		return r >= '0' && r <= '9'
	})
	return strings.TrimSpace(s)
}
func SafeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Hàm xác minh địa chỉ với Nominatim
func VerifyAddress(address model.CreateAddressInput) (lat, lng float64, err error) {
	// Try multiple address formats
	var queries []string

	street := normalizeVN(SafeString(address.Street))
	ward := normalizeVN(SafeString(address.Ward))
	district := normalizeVN(SafeString(address.District))
	province := normalizeVN(SafeString(address.Province))

	queries = []string{
		fmt.Sprintf("%s, %s, %s, %s, Việt Nam", street, ward, district, province),
		fmt.Sprintf("%s, %s, %s, Việt Nam", street, district, province),
		fmt.Sprintf("%s, %s, Việt Nam", district, province),
		fmt.Sprintf("%s, Việt Nam", province),
	}

	client := &http.Client{}
	for i, query := range queries {
		url := fmt.Sprintf("https://nominatim.openstreetmap.org/search?format=json&q=%s&limit=1&countrycodes=vn",
			url.QueryEscape(query),
		)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to create request for query %d: %v", i, err)
		}
		req.Header.Set("User-Agent", "CinemaApp/1.0 (btdx123@gmail.com)") // Replace with your email

		resp, err := client.Do(req)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to call Nominatim API for query %d: %v", i, err)
		}
		defer resp.Body.Close()

		// Log raw response for debugging

		var results []struct {
			Lat string `json:"lat"`
			Lon string `json:"lon"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
			return 0, 0, fmt.Errorf("failed to decode API response for query %d: %v", i, err)
		}

		if len(results) > 0 {
			lat, err = strconv.ParseFloat(results[0].Lat, 64)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to parse latitude for query %d: %v", i, err)
			}
			lng, err = strconv.ParseFloat(results[0].Lon, 64)
			if err != nil {
				return 0, 0, fmt.Errorf("failed to parse longitude for query %d: %v", i, err)
			}
			log.Printf("Found coordinates for query '%s': lat=%f, lng=%f", query, lat, lng)
			return lat, lng, nil
		}
	}

	return 0, 0, fmt.Errorf("invalid address: no results found for any query")
}
