package model

import "time"

type Customer struct {
	DTO
	Email    string `gorm:"unique;not null" json:"email"`
	Phone    string `gorm:"not null" json:"phone"`
	Password string `gorm:"not null" json:"-"`
	UserName string `json:"username"`

	FirstName *string `json:"firstname"`
	LastName  *string `json:"lastname"`

	AvatarUrl *string `json:"avatarUrl"`
	Gender    *bool   `json:"gender"`

	IsActive bool `gorm:"default:true" json:"isActive"`
}

type Customers []Customer

type RegisterCustomerInput struct {
	UserName string `validate:"required" json:"username"`
	Email    string `validate:"required,email" json:"email"`
	Phone    string `validate:"required" json:"phone"`
	Password string `validate:"required" json:"password"`
}

type EditCustomerInput struct {
	FirstName   *string `json:"firstname"`
	LastName    *string `json:"lastname"`
	PhoneNumber *string `json:"phoneNumber"`
	Gender      *bool   `json:"gender"`
	AvatarUrl   *string `json:"avatarUrl"`
}
type CustomerChangePassword struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	RepeatPassword  string `json:"repeatPassword"`
}
type FilterCustomer struct {
	Pagination
	SearchKey   string `json:"searchKey"`
	PhoneNumber string `json:"phoneNumber"`
	Active      *bool  `json:"active"`
}
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"email"`
}
type PasswordResetToken struct {
	ID         uint   `gorm:"primaryKey"`
	CustomerID uint   `gorm:"index"`
	TokenHash  string `gorm:"uniqueIndex"`
	ExpiresAt  time.Time
	Used       bool `gorm:"default:false"`
	CreatedAt  time.Time
}
type ResetPasswordRequest struct {
	Token          string `json:"token" validate:"required"`
	NewPassword    string `json:"newPassword" validate:"required,min=8"`
	RepeatPassword string `json:"repeatPassword"`
}
