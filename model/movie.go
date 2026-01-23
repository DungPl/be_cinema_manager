package model

import (
	"cinema_manager/utils"
	"time"

	"mime/multipart"
)

type MovieFormat struct {
	MovieId  uint   `gorm:"primaryKey" json:"movieId"`
	FormatId uint   `gorm:"primaryKey" json:"formatId"`
	Movie    Movie  `gorm:"foreignKey:MovieId" json:"movie"`
	Format   Format `gorm:"foreignKey:FormatId" json:"format"`
}
type Movie struct {
	DTO
	Genre       string          `gorm:"not null;index" validate:"required" json:"genre"`            // Thể loại
	Title       string          `gorm:"not null;index" validate:"required" json:"title"`            //Tên phim
	Duration    int             `gorm:"not null" validate:"required" json:"duration"`               //thời lượng phim
	Language    string          `gorm:"not null;index" validate:"required" json:"language"`         //Ngôn ngữ
	Description string          `gorm:"not null, type:text" validate:"required" json:"description"` //Mô tả ( review phim)
	Country     string          `gorm:"not null" validate:"required" json:"country"`
	Posters     *[]MoviePoster  `gorm:"foreignKey:MovieId" json:"posters"`
	Trailers    *[]MovieTrailer `gorm:"foreignKey:MovieId" json:"trailers"`

	//DirectorId uint     `gorm:"not null" validate:"required" json:"directorId"`
	Directors []Director `gorm:"many2many:movie_directors;" json:"directors"`
	Actors    []Actor    `gorm:"many2many:movie_actors" json:"actors"`

	AgeRestriction     string            `gorm:"not null" validate:"required,oneof=P K T13 T16 T18" json:"ageRestriction"`
	Formats            []Format          `gorm:"many2many:movie_formats;" json:"formats"`
	Slug               string            `gorm:"uniqueIndex" json:"slug"`
	DateSoon           *utils.CustomDate `gorm:"type:date" json:"dateSoon"`                                 // Suất chiểu sớm
	DateRelease        utils.CustomDate  `gorm:"type:date;not null" validate:"required" json:"dateRelease"` //Ngày khởi chiếu
	DateEnd            *utils.CustomDate `gorm:"type:date" json:"dateEnd"`
	StatusMovie        string            `gorm:"not null" validate:"required,oneof=COMING_SOON NOW_SHOWING ENDED" json:"statusMovie"`
	IsAvailable        bool              `gorm:"not null, default:false"   json:"isAvailable"` //not available available
	AccountModeratorId uint              `json:"accountModeratorId"`
	AccountModerator   Account           `gorm:"foreignKey:AccountModeratorId" json:"accountModerator"`
}
type Movies []Movie
type MoviePoster struct {
	DTO
	MovieId   uint    `gorm:"not null;index" json:"movieId"`
	Movie     Movie   `gorm:"foreignKey:MovieId" json:"movie"`
	Url       *string `gorm:"type:varchar(255)" validate:"required,url" json:"url"`
	IsPrimary bool    `json:"isPrimary" validate:"boolean"`
	PublicID  *string
}

type MovieTrailer struct {
	DTO
	MovieId   uint    `gorm:"not null;index" json:"movieId"`
	Movie     Movie   `gorm:"foreignKey:MovieId" json:"movie"`
	Url       *string `gorm:"type:varchar(255)" validate:"required,url" json:"url"`
	IsPrimary bool    `json:"isPrimary" validate:"boolean"`
	PublicID  *string
	Duration  int
}
type Director struct {
	DTO
	Name        string     `gorm:"type:varchar(255);not null;unique" validate:"required" json:"name"`
	Birthday    *time.Time `json:"birthday" validate:"omitempty"`
	Nationality string     `gorm:"size:100" json:"nationality" validate:"omitempty,max=100"`
	Avatar      *string    `json:"avatar" validate:"omitempty,url"`
	Biography   *string    `gorm:"type:text" json:"biography"`
	DirectorUrl *string    `validate:"omitempty,url" json:"directorUrl"`

	Movies []Movie `gorm:"many2many:movie_directors;"`
}

type CreateDirectorInput struct {
	Name        string     `json:"name" validate:"required,min=2,max=255"`
	Birthday    *time.Time `json:"birthday" validate:"omitempty"`
	Nationality string     `json:"nationality" validate:"omitempty,max=100"`
	Avatar      *string    `json:"avatar" validate:"omitempty,url"`
	Biography   *string    `gorm:"type:text" json:"biography"`
	DirectorUrl *string    `validate:"omitempty,url" json:"directorUrl"`
}
type UpdateDirectorInput struct {
	Name        *string    `json:"name" `
	Birthday    *time.Time `json:"birthday" validate:"omitempty"`
	Nationality *string    `json:"nationality" validate:"omitempty,max=100"`
	Avatar      *string    `json:"avatar" validate:"omitempty,url"`
	Biography   *string    `gorm:"type:text;not null" json:"biography"`
	DirectorUrl *string    `validate:"omitempty,url" json:"directorUrl"`
}
type Actor struct {
	DTO
	Name        string  `gorm:"type:varchar(255);not null;unique" validate:"required" json:"name"`
	Biography   *string `gorm:"type:text" json:"biography"`
	Nationality *string `json:"nationality" validate:"omitempty,max=100"`
	ActorUrl    *string ` validate:"omitempty,url" json:"actorUrl"`
	Avatar      *string `json:"avatar" validate:"omitempty,url"`
	Movies      []Movie `gorm:"many2many:movie_actors;" json:"movies"`
}
type CreateActorsInput struct {
	Names []string `json:"names" validate:"required,min=1,dive,required"`
}
type UpdateActorInput struct {
	Name        *string `gorm:"type:varchar(255);not null;unique" validate:"omitempty" json:"name"`
	Nationality *string `json:"nationality" validate:"omitempty,max=100"`
	Biography   *string `json:"biography" validate:"omitempty,max=1000"`
	ActorUrl    *string `json:"actorUrl" validate:"omitempty,url"`
	Avatar      *string `json:"avatar" validate:"omitempty,url"`
}
type ActorResponse struct {
	ID        uint    `json:"id"`
	Name      string  `json:"name"`
	Biography *string `json:"biography,omitempty"`
	ActorUrl  *string `json:"actorUrl,omitempty"`
	Avatar    *string `json:"avatar,omitempty"`
}
type MovieDirector struct {
	MovieId    uint     `gorm:"not null;index" json:"movieId"`
	DirectorId uint     `gorm:"not null;index" json:"directorId"`
	Movie      Movie    `gorm:"foreignKey:MovieId" json:"movie"`
	Director   Director `gorm:"foreignKey:DirectorId" json:"director"`
}
type MovieActor struct {
	MovieId uint  `gorm:"not null;index" json:"movieId"`
	ActorId uint  `gorm:"not null;index" json:"actorId"`
	Movie   Movie `gorm:"foreignKey:MovieId" json:"movie"`
	Actor   Actor `gorm:"foreignKey:ActorId" json:"actor"`
}
type CreateMovieInput struct {
	Genre          string            `gorm:"not null;index" validate:"required" json:"genre"`            // Thể loại
	Title          string            `gorm:"not null;index" validate:"required" json:"title"`            //Tên phim
	Duration       int               `gorm:"not null" validate:"required" json:"duration"`               //thời lượng phim
	Language       string            `gorm:"not null;index" validate:"required" json:"language"`         //Ngôn ngữ
	Description    string            `gorm:"not null, type:text" validate:"required" json:"description"` //Mô tả ( review phim)
	Country        string            `gorm:"not null" validate:"required" json:"country"`
	DirectorIds    []uint            `json:"directorIds" validate:"required"` // ID hoặc tên mới
	DirectorName   *string           `json:"directorName" `                   // Tên mới nếu không có ID
	ActorIds       []uint            `json:"actorIds" validate:"required"`
	ActorNames     []string          `json:"actorNames" `
	FormatIds      []uint            `json:"formatIds" validate:"required,min=1,dive,required"`
	AgeRestriction string            `json:"ageRestriction" validate:"required,oneof=P K T13 T16 T18"`
	DateSoon       *utils.CustomDate `gorm:"type:date" json:"dateSoon"`                                 // Suất chiểu sớm
	DateRelease    utils.CustomDate  `gorm:"type:date;not null" validate:"required" json:"dateRelease"` //Ngày khởi chiếu
	DateEnd        *utils.CustomDate `gorm:"type:date" json:"dateEnd"`
}
type EditMovieInput struct {
	Genre          *string           `gorm:"index"  json:"genre"`           // Thể loại
	Title          *string           `gorm:"index"  json:"title"`           //Tên phim
	Duration       *int              `json:"duration"`                      //thời lượng phim
	Language       *string           `gorm:"index"  json:"language"`        //Ngôn ngữ
	Description    *string           `gorm:"type:text"  json:"description"` //Mô tả ( review phim)
	Country        *string           ` json:"country"`
	DirectorIds    *[]uint           `json:"directorIds"`  // ID hoặc tên mới
	DirectorName   *string           `json:"directorName"` // Tên mới nếu không có ID
	ActorIds       *[]uint           `json:"actorIds"`
	ActorNames     *[]string         `json:"actorNames"`
	AgeRestriction *string           `json:"ageRestriction" `
	FormatIds      *[]uint           `json:"formatIds" `
	StatusMovie    *string           `json:"statusMovie"`
	DateSoon       *utils.CustomDate `gorm:"type:date" json:"dateSoon"`     // Suất chiểu sớm
	DateRelease    *utils.CustomDate `gorm:"type:date"  json:"dateRelease"` //Ngày khởi chiếu
	DateEnd        *utils.CustomDate `gorm:"type:date" json:"dateEnd"`
}
type FilterMoviInput struct {
	Pagination
	Genre         string     `query:"genre"`
	Title         string     `query:"title"`
	Duration      int        `query:"duration"`
	Language      string     `query:"language"`
	Country       string     `query:"country"`
	DateRelease   *time.Time `query:"dateRelease"`
	ShowingStatus string     `query:"showingStatus" validate:"omitempty,oneof=COMING_SOON NOW_SHOWING ENDED"`
}

// UploadMovieMediaInput defines the structure for uploading movie poster and trailer
type UploadMovieMediaInput struct {
	Poster    *multipart.FileHeader `json:"poster" validate:"required"`
	Trailer   *multipart.FileHeader `json:"trailer" validate:"required"`
	MovieId   uint                  `json:"movieId" validate:"required"`
	IsPrimary bool                  `json:"isPrimary" validate:"boolean"` // Đánh dấu poster/trailer chính
}
