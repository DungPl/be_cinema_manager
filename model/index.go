package model

import "time"

type TokenData struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type TokenClaim struct {
	CustomerId uint   `json:"customerId"`
	AccountId  uint   `json:"accountId"`
	Username   string `json:"username"`
	CinemaId   *uint  `json:"cinemaId"`
}

type DTO struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	DeletedAt time.Time `json:"deletedAt,omitempty"`
}
type ResponseCustom struct {
	Rows       any   `json:"rows"`
	Limit      *int  `json:"limit"`
	Page       *int  `json:"page"`
	TotalCount int64 `json:"totalCount"`
}

type ArrayId struct {
	IDs []uint ` json:"ids"`
}
type Pagination struct {
	Limit *int `json:"limit"`
	Page  *int `json:"page"`
}
type AdminChangePassword struct {
	AccountId      uint   `json:"accountId"`
	NewPassword    string `json:"newPassword"`
	RepeatPassword string `json:"repeatPassword"`
}

type StaffChangePassword struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	RepeatPassword  string `json:"repeatPassword"`
}
