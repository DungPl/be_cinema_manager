package helper

import (
	"cinema_manager/model"
	"fmt"

	"github.com/gosimple/slug"
	"gorm.io/gorm"
)

func GenerateUniqueCinemaSlug(tx *gorm.DB, name string) string {
	base := slug.Make(name)
	result := base
	i := 1

	for {
		var count int64
		tx.Model(&model.Cinema{}).
			Where("slug = ?", result).
			Count(&count)

		if count == 0 {
			break
		}
		result = fmt.Sprintf("%s-%d", base, i)
		i++
	}

	return result
}

func GenerateUniquerateUniqueMoviSlug(tx *gorm.DB, title string) string {
	base := slug.Make(title)
	result := base
	i := 1

	for {
		var count int64
		tx.Model(&model.Movie{}).
			Where("slug = ?", result).
			Count(&count)

		if count == 0 {
			break
		}
		result = fmt.Sprintf("%s-%d", base, i)
		i++
	}

	return result
}
