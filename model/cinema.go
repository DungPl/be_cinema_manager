package model

type CinemaChain struct {
	DTO
	Name        string   `gorm:"not null" validate:"required" json:"name"`
	Description string   `json:"description"`
	LogoUrl     string   `json:"logoUrl"`
	Active      *bool    `gorm:"not null, default:true" json:"isActive"`
	Cinemas     []Cinema `gorm:"foreignKey:ChainId"`
}
type CreateCinemaChainInput struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description" validate:"omitempty"`
	Active      *bool  `gorm:"not null, default:true" json:"isActive"`
}
type EditCinemaChainInput struct {
	Name        *string `json:"name" `
	Description *string `json:"description" validate:"omitempty"`
	Logo        *string `form:"logoUrl" validate:"omitempty,url"`
	Active      *bool   `gorm:"not null, default:true" json:"isActive"`
}
type Cinema struct {
	DTO
	Slug        string      `gorm:"uniqueIndex" json:"slug"`
	Name        string      `gorm:"not null, unique" validate:"required" json:"name"`
	Phone       string      `json:"phone"`
	Active      *bool       `gorm:"not null, default:true" json:"isActive"`
	ChainId     uint        `gorm:"not null" json:"chainId"`
	Description *string     `json:"description"`
	Chain       CinemaChain `gorm:"foreignKey:ChainId;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"chain"`
	Promotions  []Promotion `gorm:"foreignKey:CinemaId" json:"promotions"` // 👈 nên để plural vì 1 rạp có thể có nhiều khuyến mãi
	Rooms       []Room      `gorm:"foreignKey:CinemaId" json:"rooms"`
	Addresses   []Address   `json:"address"` // preload địa chỉ
	RoomCount   int64       `json:"roomCount" gorm:"-"`
}
type Cinemas []Cinema
type CreateCinemaInput struct {
	Name        string             `gorm:"not null" validate:"required" json:"name"`
	ChainId     uint               `json:"chainId" validate:"required"`
	Address     CreateAddressInput `json:"address" validate:"required"`
	Description *string            `json:"description"`
}
type EditCinemaInput struct {
	Name        *string             `gorm:"not null" validate:"required" json:"name"`
	Phone       *string             `json:"phone"`
	Active      *bool               `gorm:"not null, default:true" json:"isActive"`
	ChainId     *uint               `json:"chainId" `
	Address     *CreateAddressInput `json:"address"`
	Description *string             `json:"description"`
}

type FilterCinema struct {
	Pagination

	SearchKey string `json:"searchKey"`
	Name      string `json:"name"`
	District  string `json:"district"`
	Province  string `json:"province"`
	ChainId   uint   `json:"chainId"`
	ChainName string `json:"chainName"`
}
type FilterCinemaChain struct {
	Pagination
	SearchKey string `json:"searchKey"`
}
