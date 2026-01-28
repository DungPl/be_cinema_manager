package model

import "time"

type LanguageType string

const (
	LangViSub LanguageType = "VI_SUB"
	LangViDub LanguageType = "VI_DUB"
	LangEnSub LanguageType = "EN_SUB"
	LangEnDub LanguageType = "EN_DUB"
	LangVi    LanguageType = "VI"
)

type Showtime struct {
	DTO
	PublicCode   string       `gorm:"size:16;uniqueIndex" json:"publicCode"` // 👈 THÊM
	StartTime    time.Time    `validate:"required" json:"start"`
	EndTime      time.Time    `validate:"required" json:"end"`
	Price        float64      `json:"price"`
	Status       string       `json:"status"`
	Format       string       `gorm:"size:10" json:"format"` // 2D, 3D, IMAX, 4DX
	LanguageType LanguageType `gorm:"size:20"  default:"VI_SUB" json:"languageType"`
	MovieId      uint         `json:"movieId"`
	RoomId       uint         `json:"roomId"`
	Movie        Movie        `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;foreignKey:MovieId" json:"Movie"`
	Room         Room         `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;foreignKey:RoomId" json:"Room"`

	Tickets []Ticket `gorm:"foreignKey:ShowtimeId" json:"tickets"`
}
type ShowtimeSeat struct {
	DTO
	ShowtimeId uint       `gorm:"uiniqueIndex" json:"showtimeId"`
	SeatId     uint       `json:"seatid"`
	SeatRow    string     `json:"seatRow"`
	SeatNumber int        `json:"seatNumber"`
	SeatTypeId uint       `json:"seatTypeId"`
	Status     string     `gorm:"uiniqueIndex" json:"status"`
	ExpiredAt  *time.Time `json:"expiredAt"`
	HeldBy     string     `json:"heldBy"`
	Showtime   Showtime   `json:"Showtime"`
	SeatType   SeatType   ` json:"SeatType"`
	Seat       Seat       `json:"Seat"`
}
type ShowtimeResponse struct {
	DTO
	Format         string    `json:"format"`
	Price          float64   `json:"price"`
	Movie          Movie     `json:"movie"`
	Room           Room      `json:"room"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	FillRate       float64   `json:"fill_rate"`
	BookedSeats    int64     `json:"booked_seats"`
	TotalSeats     int64     `json:"total_seats"`
	LanguageType   string    `json:"language_type"`
	BookedRevenue  float64   `json:"booked_revenue"`  // Tổng tiền bán vé (không trừ hoàn)
	ActualRevenue  float64   `json:"actual_revenue"`  // Tiền rạp thực nhận (sau hoàn tiền)
	RefundedAmount float64   `json:"refunded_amount"` // Tổng tiền đã hoàn cho suất này
}
type CreateShowtimeBatchInput struct {
	MovieID      uint     `json:"movieId" validate:"required"`
	RoomIDs      []uint   `json:"roomIds" validate:"required,dive,required"`
	StartDate    string   `json:"startDate" validate:"required"` // YYYY-MM-DD
	EndDate      string   `json:"endDate" validate:"required"`
	LanguageType string   `gorm:"size:20" json:"languageType"` // VI_SUB, VI_DUB, EN_SUB, EN_DUB
	Formats      []string `json:"formats" validate:"required,dive,oneof=2D 3D IMAX 4DX"`
	TimeSlots    []string `json:"timeSlots" validate:"required,dive"` // ["18:30", "20:45"]
}
type FilterShowtimeInput struct {
	Pagination
	Province      string `query:"province"`
	District      string `query:"district"`
	Ward          string `query:"ward"`
	MovieId       uint   `json:"movieId" validate:"omitempty,gt=0"`
	RoomId        uint   `json:"roomId" validate:"omitempty,gt=0"`
	CinemaId      uint   `json:"cinemaId" validate:"omitempty,gt=0"`
	StartDate     string `query:"startDate"`
	EndDate       string `query:"endDate"`
	ShowingStatus string `json:"showingStatus" validate:"omitempty,oneof=UPCOMING ONGOING ENDED"`
}
type AutoGenerateScheduleInput struct {
	MovieID      uint     `json:"movieId" validate:"required"`
	RoomIDs      []uint   `json:"roomIds" validate:"required,dive,required"`
	StartDate    string   `json:"startDate" validate:"required"` // YYYY-MM-DD
	EndDate      string   `json:"endDate" validate:"required"`
	Formats      []string `json:"formats" validate:"required,dive,oneof=2D 3D IMAX 4DX"`
	LanguageType []string `gorm:"size:20" json:"languageType"`        // VI_SUB, VI_DUB, EN_SUB, EN_DUB
	TimeSlots    []string `json:"timeSlots" validate:"required,dive"` // ["18:30", "20:45"]
	IsVietnamese bool     `json:"isVietnamese"`
	Price        float64  `json:"price"` // ưu tiên khung giờ vàng
}
type UpdateShowtimeInput struct {
	MovieId   *uint      `json:"movieId" `
	RoomId    *uint      `json:"roomId" `
	StartTime *time.Time `json:"start_time" `
	EndTime   *time.Time `json:"endTime" `
	Price     *float64   `json:"price"`
}

// BulkShowtimeInput defines the input structure for a single showtime in bulk creation
type BulkShowtimeInput struct {
	MovieId   uint      `json:"movieId" validate:"required,gt=0"`
	RoomId    uint      `json:"roomId" validate:"required,gt=0"`
	StartTime time.Time `json:"startTime" validate:"required"`
	EndTime   time.Time `json:"endTime" validate:"required,gtfield=StartTime"`
	Price     float64   `json:"price" validate:"required,gt=0"`
}

// CreateBulkShowtimesInput defines the input structure for bulk creation
type CreateBulkShowtimesInput struct {
	Showtimes []BulkShowtimeInput `json:"showtimes" validate:"required,dive"`
}
