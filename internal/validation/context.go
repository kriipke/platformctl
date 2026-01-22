package validation

import "platformctl/internal/models"

func ValidateContext(v *Validator, context *models.Context) error {
	return v.Struct(context)
}
