package validation

import "platformctl/internal/models"

func ValidateApp(v *Validator, app *models.App) error {
	return v.Struct(app)
}
