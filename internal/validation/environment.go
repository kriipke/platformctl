package validation

import "platformctl/internal/models"

func ValidateEnvironment(v *Validator, environment *models.Environment) error {
	return v.Struct(environment)
}
