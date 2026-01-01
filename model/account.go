package model

type Account struct {
	DTO
	Username     string `gorm:"uniqueIndex;not null" validate:"required,min=3,max=50" json:"username"`
	Password     string `gorm:"not null" validate:"required,min=6,max=50" json:"password"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	Active       bool   `gorm:"not null;default:true" json:"active"`
	Role         string `json:"role"`
	CinemaId     *uint  `json:"cinemaId"`
	Staff        *Staff `gorm:"foreignKey:AccountId" json:"staff"`
	Cinema       Cinema `gorm:"foreignKey:CinemaId;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"cinema"`
}

type Accounts []Account

type CreateAccountInput struct {
	Username string `gorm:"uniqueIndex;not null" validate:"required,min=3,max=50" json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"` // MANAGER MODERATOR SELLER
	CinemaId *uint  `json:"cinemaId"`
}
type UpdateManagerCinemaInput struct {
	CinemaId *uint `json:"cinemaId" validate:"omitempty"` // Allow null to remove assignment
}
type FilterAccount struct {
	Pagination
	SearchKey string  `json:"searchKey"`
	Active    *bool   `json:"active"`
	Role      *string `json:"role"`
	IsUsed    *bool   `json:"isUsed"`
}
