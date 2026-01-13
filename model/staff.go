package model

type Staff struct {
	DTO
	FirstName          string   `gorm:"not null" validate:"required" json:"firstname"`
	LastName           string   `gorm:"not null" validate:"required" json:"lastname"`
	Address            string   `json:"address"`
	PhoneNumber        string   `gorm:"not null" json:"phoneNumber"`
	IsActive           bool     `gorm:"not null;default:true" json:"isActive"`
	Email              string   `json:"email"`
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
type CreateStaffWithAccountInput struct {
	// Thông tin Staff
	Firstname          string `json:"firstname" validate:"required,min=1"`
	Lastname           string `json:"lastname" validate:"required,min=1"`
	PhoneNumber        string `json:"phoneNumber" validate:"required,min=9"`
	Email              string `json:"email" validate:"omitempty,email"`
	IdentificationCard string `json:"identificationCard" validate:"required,min=9"`
	Position           string `json:"position,omitempty"`

	// Thông tin Account
	Username string `json:"username" validate:"required,min=4"`
	Password string `json:"password" validate:"required,min=6"`
	Role     string `json:"role" validate:"required,oneof=ADMIN MANAGER MODERATOR SELLER"`
	CinemaId *uint  `json:"cinemaId,omitempty"`          // null nếu không ràng buộc rạp
	Active   bool   `json:"active" validate:"omitempty"` // mặc định true nếu không gửi
}

// Cập nhật thông tin tài khoản liên kết với staff
type UpdateStaffAccountInput struct {
	Role     *string `json:"role" validate:"omitempty,oneof=ADMIN MANAGER MODERATOR SELLER"`
	CinemaId *uint   `json:"cinemaId,omitempty"`
	Active   *bool   `json:"active,omitempty"`
	// Username không cho phép update (hoặc chỉ admin cấp cao mới được)
	// Password nên dùng endpoint change-password riêng
}
type FilterStaff struct {
	Pagination
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	SearchKey string `json:"searchKey"`
	Active    *bool  `json:"active"`
}
