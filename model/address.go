package model

type Address struct {
	DTO
	HouseNumber *string `  json:"house_number"` // số nhà
	Province    *string `  json:"province"`     // Tỉnh
	District    *string `  json:"district"`     // Huyện
	Ward        *string `  json:"ward"`         // phường
	Street      *string ` json:"street"`        // đường phố
	FullAddress string  `gorm:"not null" validate:"required" json:"fullAddress"`
	Latitude    float64 `json:"latitude"` // Thêm để lưu tọa độ
	Longitude   float64 `json:"longitude"`
	CinemaId    uint    `json:"cinemaId"`
	Cinema      *Cinema `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL, foreignKey:CinemaId" json:"cinema"`
}
type CreateAddressInput struct {
	ID          *uint   `json:"id"`
	HouseNumber *string ` json:"house_number"`
	Province    *string `  json:"province"`
	District    *string `  json:"district"`
	Ward        *string `  json:"ward"`
	Street      *string ` json:"street"`
	FullAddress string  `gorm:"not null" json:"fullAddress"`
	Latitude    float64 `json:"latitude"`  // optional: dùng nếu front-end cung cấp
	Longitude   float64 `json:"longitude"` // optional
}
