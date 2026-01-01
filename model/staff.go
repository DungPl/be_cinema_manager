package model

type Staff struct {
	DTO
	FirstName          string   `gorm:"not null" validate:"required" json:"firstname"`
	LastName           string   `gorm:"not null" validate:"required" json:"lastname"`
	Address            string   `json:"address"`
	PhoneNumber        string   `gorm:"not null" json:"phoneNumber"`
	IsActive           bool     `gorm:"not null;default:true" json:"isActive"`
	IdentificationCard string   `gorm:"not null;uniqueIndex;require" validate:"required,min=12,max=12" json:"identificationCard"`
	Role               string   `json:"role"`
	Note               string   `json:"note"`
	AccountId          *uint    `json:"accountId"`
	Account            *Account `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;foreignKey:AccountId" json:"account"`
}

type Staffs []Staff

type CreateStaffInput struct {
	FirstName          string `gorm:"not null" validate:"required" json:"firstname"`
	LastName           string `gorm:"not null" validate:"required" json:"lastname"`
	Address            string `json:"address"`
	PhoneNumber        string `json:"phoneNumber"`
	IdentificationCard string `json:"identificationCard"`
	Role               string `json:"role"`
	AccountId          *uint  `json:"accountId"`
}

type FilterStaff struct {
	Pagination
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	SearchKey string `json:"searchKey"`
	Active    *bool  `json:"active"`
}
