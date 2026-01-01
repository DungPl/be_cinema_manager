package model

type ScheduleTemplate struct {
	DTO

	Name        string `gorm:"size:100;not null" json:"name" validate:"required,min=3,max=100"`
	Description string `gorm:"type:text" json:"description"`

	DayTypes   []string `gorm:"type:json;serializer:json" json:"dayTypes" validate:"required,dive,oneof=weekday weekend friday saturday sunday holiday early"`
	MovieTypes []string `gorm:"type:json;serializer:json" json:"movieTypes" validate:"required,dive,oneof=blockbuster vietnamese kids art horror romance"`
	TimeSlots  []string `gorm:"type:json;serializer:json" json:"timeSlots" validate:"required,dive,timeslot"`
	Formats    []string `gorm:"type:json;serializer:json" json:"formats" validate:"required,dive,oneof=2D 3D IMAX 4DX"`

	MaxRooms  int `gorm:"not null;default:1;check:max_rooms >= 1" json:"maxRooms" validate:"required,min=1,max=10"`
	MaxPerDay int `gorm:"not null;default:1;check:max_per_day >= 1" json:"maxPerDay" validate:"required,min=1,max=10"`

	Priority int `gorm:"not null;default:50;check:priority >= 0 AND priority <= 100" json:"priority" validate:"required,min=0,max=100"`

	CreatedBy uint `gorm:"not null" json:"createdBy"`
}
type FilterScheduleTemplateInput struct {
	Pagination
	DayTypes   string `query:"dayTypes" validate:"omitempty,regex=^[a-zA-Z]+(,[a-zA-Z]+)*$"`
	MovieTypes string `query:"movieTypes" validate:"omitempty,regex=^[a-zA-Z]+(,[a-zA-Z]+)*$"`
	TimeSlots  string `query:"timeSlots" validate:"omitempty,regex=^[0-9:]+(,[0-9:]+)*$"`
}
type CreateScheduleTemplateInput struct {
	Name        string   `json:"name" validate:"required,min=3,max=100"`
	Description string   `json:"description"`
	DayTypes    []string `json:"dayTypes" validate:"required,dive,oneof=weekday weekend friday saturday sunday holiday early"`
	MovieTypes  []string `json:"movieTypes" validate:"required,dive,oneof=blockbuster vietnamese kids art horror romance"`
	TimeSlots   []string `json:"timeSlots" validate:"required,dive,timeslot"`
	Formats     []string `json:"formats" validate:"required,dive,oneof=2D 3D IMAX 4DX"`
	MaxRooms    int      `json:"maxRooms" validate:"required,min=1,max=10"`
	MaxPerDay   int      `json:"maxPerDay" validate:"required,min=1,max=10"`
	Priority    int      `json:"priority" validate:"required,min=0,max=100"`
}

type UpdateScheduleTemplateInput struct {
	Name        *string  `json:"name" validate:"omitempty,min=3,max=100"`
	Description *string  `json:"description"`
	DayTypes    []string `json:"dayTypes" validate:"omitempty,dive,oneof=weekday weekend friday saturday sunday holiday early"`
	MovieTypes  []string `json:"movieTypes" validate:"omitempty,dive,oneof=blockbuster vietnamese kids art horror romance"`
	TimeSlots   []string `json:"timeSlots" validate:"omitempty,dive,timeslot"`
	Formats     []string `json:"formats" validate:"omitempty,dive,oneof=2D 3D IMAX 4DX"`
	MaxRooms    *int     `json:"maxRooms" validate:"omitempty,min=1,max=10"`
	MaxPerDay   *int     `json:"maxPerDay" validate:"omitempty,min=1,max=10"`
	Priority    *int     `json:"priority" validate:"omitempty,min=0,max=100"`
}
type AutoGenerateScheduleTemplateInput struct {
	MovieId      uint     `json:"movieId" validate:"required"`
	RoomIds      []uint   `json:"roomIds" validate:"required,dive,required,min=1"`
	StartDate    string   `json:"startDate" validate:"required"` // YYYY-MM-DD
	EndDate      string   `json:"endDate" validate:"required"`
	Formats      []string `json:"formats" validate:"required,dive,oneof=2D 3D IMAX 4DX"`
	TemplateId   *uint    `json:"templateId,omitempty"` // optional
	TemplateName *string  `json:"templateName,omitempty"`
	IsVietnamese *bool    `json:"isVietnamese,omitempty"`
}
