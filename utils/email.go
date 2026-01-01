package utils

import (
	"bytes"
	"html/template"
	"log"
	"os"
	"strconv"

	"gopkg.in/gomail.v2"
)

// OrderConfirmationData dữ liệu cho template email
type OrderConfirmationData struct {
	OrderCode     string
	MovieName     string
	Showtime      string
	Seats         string
	TotalAmount   float64
	PaymentMethod string
	DetailLink    string
	CancelledLink string
}

// SendOrderConfirmationEmail gửi email xác nhận đơn hàng (async)
func SendOrderConfirmationEmail(to string, data OrderConfirmationData) {
	go func() { // Async để không delay response
		tmplPath := "templates/order_confirmation.html" // Đường dẫn template
		tmpl, err := template.ParseFiles(tmplPath)
		if err != nil {
			log.Printf("Lỗi load template email: %v", err)
			return
		}

		var body bytes.Buffer
		if err := tmpl.Execute(&body, data); err != nil {
			log.Printf("Lỗi render template email: %v", err)
			return
		}

		host := os.Getenv("SMTP_HOST")
		portStr := os.Getenv("SMTP_PORT")
		username := os.Getenv("SMTP_USERNAME")
		password := os.Getenv("SMTP_PASSWORD")
		from := os.Getenv("SMTP_FROM")

		port, _ := strconv.Atoi(portStr)

		m := gomail.NewMessage()
		m.SetHeader("From", from)
		m.SetHeader("To", to)
		m.SetHeader("Subject", "Xác nhận đơn hàng #"+data.OrderCode)
		m.SetBody("text/html", body.String())

		d := gomail.NewDialer(host, port, username, password)
		if err := d.DialAndSend(m); err != nil {
			log.Printf("Lỗi gửi email: %v", err)
		}
	}()
}
