package model

type Format struct {
	DTO
	Name string `gorm:"unique;not null;size:20" json:"name"` // 2D, 3D, 4DX, IMAX
}
type RoomFormat struct {
	RoomID   uint   `gorm:"primaryKey" json:"roomId"`
	FormatID uint   `gorm:"primaryKey" json:"formatId"`
	Room     Room   `gorm:"foreignKey:RoomID" json:"-"`
	Format   Format `gorm:"foreignKey:FormatID" json:"format"`
}
type RoomType string

const (
	Small  RoomType = "Small"
	Medium RoomType = "Medium"
	Large  RoomType = "Large"
	IMAX   RoomType = "IMAX"
	FourDX RoomType = "4DX"
)

type Room struct {
	DTO
	Name       string `gorm:"not null" validate:"required" json:"name"`
	RoomNumber uint   `json:"roomNumber" validate:"required,min=1"`
	Capacity   *int   `  json:"capacity"`
	Row        string `json:"row" validate:"required, min=9"`

	Type          RoomType `json:"type"`
	Status        string   `gorm:"not null" validate:"required" json:"status"`
	CinemaId      uint     `json:"cinemaId"`
	Cinema        Cinema   `gorm:"foreignKey:CinemaId;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"cinema"`
	Seats         []Seat   `gorm:"foreignKey:RoomId;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"seats"`
	Formats       []Format `gorm:"many2many:room_formats;" json:"formats"`
	HasCoupleSeat bool     `gorm:"default:false" json:"hasCoupleSeat"`
}
type CreateScreeningRoomInput struct {
	CinemaId      uint     `json:"cinemaId" validate:"required"`
	RoomNumber    uint     `json:"roomNumber" validate:"required,min=1"`
	Type          RoomType `json:"type" `
	Row           string   `json:"row" validate:"required"`                     // số hàng ghế A B C D E F G H I J K
	Columns       int      `json:"columns" validate:"required"`                 // số ghế trên 1 hàng
	FormatIDs     []uint   `json:"formatIds" validate:"required,dive,required"` // "2D,3D,IMAX"
	VipColMin     int      `json:"vipColMin" validate:"omitempty"`
	VipColMax     int      `json:"vipColMax" validate:"omitempty"`
	HasCoupleSeat bool     `gorm:"default:false" json:"hasCoupleSeat"`
}
type EditRoomInput struct {
	CinemaId   *uint            `json:"cinemaId" `
	RoomNumber *uint            `json:"roomNumber"`
	Type       *RoomType        `json:"type"`
	FormatIds  *[]uint          `json:"formatIds"`
	Status     *string          `json:"status" validate:"omitempty,oneof=available maintenance closed"`
	Seat       *UpdateSeatInput `json:"seat"`
}
type UpdateSeatInput struct {
	HasCoupleSeat *bool   ` json:"hasCoupleSeat"`
	Columns       *int    `json:"columns" `
	Row           *string `json:"row"` // e.g., "A", "B"
	VipColMin     *int    `json:"vipColMin" validate:"omitempty,min=3,max=5"`
	VipColMax     *int    `json:"vipColMax" validate:"omitempty"`
}
