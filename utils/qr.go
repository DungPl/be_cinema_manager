package utils

import (
	"bytes"
	"image/png"

	"github.com/skip2/go-qrcode"
)

// GenerateQRCode tạo QR code và trả về bytes PNG
func GenerateQRCode(content string, size int) ([]byte, error) {
	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return nil, err
	}

	// Tạo buffer
	buf := new(bytes.Buffer)
	err = png.Encode(buf, qr.Image(size))
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
