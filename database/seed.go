package database

import (
	"cinema_manager/constants"
	"cinema_manager/model"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func parseDate(dateStr string) time.Time {
	t, _ := time.Parse("2006-01-02", dateStr)
	return t
}
func SeedData(db *gorm.DB) {
	bytes, err := bcrypt.GenerateFromPassword([]byte("123456cn"), 10)
	HashPassword := string(bytes)
	if err != nil {
		HashPassword = "123456cn"
	}
	accounts := []model.Account{
		{Username: "Administration", Password: HashPassword, Active: true, Role: constants.ROLE_ADMIN},
	}

	for _, account := range accounts {
		// Tạo mới nếu không tồn tại
		if err := db.Where(model.Account{Username: account.Username}).FirstOrCreate(&account).Error; err != nil {
			log.Println("failed to seed data for account:", account.Username, "error:", err)
		}
	}
	seatTypes := []model.SeatType{
		{Type: "NORMAL", PriceModifier: 1},
		{Type: "VIP", PriceModifier: 1.2},
		{Type: "COUPLE", PriceModifier: 2},
	}
	formats := []model.Format{
		{Name: "2D"},
		{Name: "3D"},
		{Name: "4DX"},
		{Name: "IMAX"},
	}
	holidays := []model.Holiday{
		// === SOLAR (Dương lịch) ===
		{
			Name:        "Tết Dương lịch 2025",
			Date:        parseDate("2025-01-01"),
			Type:        "solar",
			IsRecurring: true,
		},
		{
			Name:        "Giỗ Tổ Hùng Vương 2025",
			Date:        parseDate("2025-04-18"), // 10/3 Âm → 18/4/2025
			Type:        "solar",
			IsRecurring: true,
		},
		{
			Name:        "Ngày Giải phóng miền Nam 2025",
			Date:        parseDate("2025-04-30"),
			Type:        "solar",
			IsRecurring: true,
		},
		{
			Name:        "Quốc tế Lao động 2025",
			Date:        parseDate("2025-05-01"),
			Type:        "solar",
			IsRecurring: true,
		},
		{
			Name:        "Quốc khánh 2025",
			Date:        parseDate("2025-09-02"),
			Type:        "solar",
			IsRecurring: true,
		},

		// === LUNAR (Âm lịch) – Dùng ngày dương tương ứng ===
		{
			Name:        "30 Tết (nếu có) 2025",
			Date:        parseDate("2025-01-27"), // 30/12/2024 Âm → 27/1/2025
			Type:        "lunar",
			IsRecurring: true,
		},
		{
			Name:        "Mùng 1 Tết 2025",
			Date:        parseDate("2025-01-28"),
			Type:        "lunar",
			IsRecurring: true,
		},
		{
			Name:        "Mùng 2 Tết 2025",
			Date:        parseDate("2025-01-29"),
			Type:        "lunar",
			IsRecurring: true,
		},
		{
			Name:        "Mùng 3 Tết 2025",
			Date:        parseDate("2025-01-30"),
			Type:        "lunar",
			IsRecurring: true,
		},
		{
			Name:        "Rằm tháng Giêng 2025",
			Date:        parseDate("2025-02-12"),
			Type:        "lunar",
			IsRecurring: true,
		},
		{
			Name:        "Lễ Vu Lan 2025",
			Date:        parseDate("2025-08-14"),
			Type:        "lunar",
			IsRecurring: true,
		},
		{
			Name:        "Tết Trung Thu 2025",
			Date:        parseDate("2025-09-17"),
			Type:        "lunar",
			IsRecurring: true,
		},
		// === 2026 ===
		{
			Name:        "Tết Dương lịch 2026",
			Date:        parseDate("2026-01-01"),
			Type:        "solar",
			IsRecurring: true,
		},
		{
			Name:        "Giỗ Tổ Hùng Vương 2026",
			Date:        parseDate("2026-04-09"), // 10/3 Âm 2026
			Type:        "solar",
			IsRecurring: true,
		},
		{
			Name:        "Ngày Giải phóng miền Nam 2026",
			Date:        parseDate("2026-04-30"),
			Type:        "solar",
			IsRecurring: true,
		},
		{
			Name:        "Quốc tế Lao động 2026",
			Date:        parseDate("2026-05-01"),
			Type:        "solar",
			IsRecurring: true,
		},
		{
			Name:        "Quốc khánh 2026",
			Date:        parseDate("2026-09-02"),
			Type:        "solar",
			IsRecurring: true,
		},

		// === Lunar Tết 2026 ===
		{
			Name:        "30 Tết 2026",
			Date:        parseDate("2026-02-16"),
			Type:        "lunar",
			IsRecurring: true,
		},
		{
			Name:        "Mùng 1 Tết 2026",
			Date:        parseDate("2026-02-17"),
			Type:        "lunar",
			IsRecurring: true,
		},
		{
			Name:        "Mùng 2 Tết 2026",
			Date:        parseDate("2026-02-18"),
			Type:        "lunar",
			IsRecurring: true,
		},
		{
			Name:        "Mùng 3 Tết 2026",
			Date:        parseDate("2026-02-19"),
			Type:        "lunar",
			IsRecurring: true,
		},
		{
			Name:        "Rằm tháng Giêng 2026",
			Date:        parseDate("2026-03-03"),
			Type:        "lunar",
			IsRecurring: true,
		},
		{
			Name:        "Lễ Vu Lan 2026",
			Date:        parseDate("2026-08-18"),
			Type:        "lunar",
			IsRecurring: true,
		},
		{
			Name:        "Tết Trung Thu 2026",
			Date:        parseDate("2026-09-16"),
			Type:        "lunar",
			IsRecurring: true,
		},
	}
	for _, f := range formats {
		db.FirstOrCreate(&f, model.Format{Name: f.Name})
	}
	for _, h := range holidays {
		var exists model.Holiday
		result := db.
			Where("name = ? AND date = ? AND type = ?", h.Name, h.Date.Format("2006-01-02"), h.Type).
			First(&exists)

		if result.Error != nil {
			db.Create(&h)
		}
	}
	for i := range seatTypes {
		if err := db.Where("type = ?", seatTypes[i].Type).FirstOrCreate(&seatTypes[i]).Error; err != nil {
			log.Println("failed to seed data for seat type:", seatTypes[i].Type, "error:", err)
		}
	}
	var account model.Account
	db.Where(model.Account{Username: "Administration"}).First(&account)

	staffs := []model.Staff{
		{FirstName: "Bùi Tiến", LastName: "Dũng", PhoneNumber: "0369757203", IdentificationCard: "027203003306", AccountId: &account.ID, Role: constants.ROLE_ADMIN},
	}

	for _, staff := range staffs {
		// Tạo mới nếu không tồn tại
		if err := db.Where(model.Staff{IdentificationCard: staff.IdentificationCard}).FirstOrCreate(&staff).Error; err != nil {
			db.Where(model.Staff{IdentificationCard: staff.IdentificationCard}).Updates(&staff)
			log.Println("failed to seed data for staff:", staff.IdentificationCard, "error:", err)
		}
	}
}
