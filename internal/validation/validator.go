package validation

import (
	"regexp"

	"github.com/go-playground/validator/v10"
	"golang.org/x/mod/semver"
)

const (
	dns1123LabelPattern   = "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
	vaultPathPattern      = "^[A-Za-z0-9/_-]+$"
	customerBranchPattern = "^customer/[a-z0-9-]+$"
)

// Validator wraps the go-playground validator with GitOps-specific rules.
type Validator struct {
	validate *validator.Validate
}

func NewValidator() *Validator {
	v := validator.New()
	_ = v.RegisterValidation("dns1123label", dns1123Label)
	_ = v.RegisterValidation("semver", semverTag)
	_ = v.RegisterValidation("vaultpath", vaultPathTag)
	_ = v.RegisterValidation("customer_branch", customerBranchTag)

	return &Validator{validate: v}
}

func (v *Validator) Struct(input any) error {
	return v.validate.Struct(input)
}

func dns1123Label(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if len(value) == 0 || len(value) > 63 {
		return false
	}
	return regexp.MustCompile(dns1123LabelPattern).MatchString(value)
}

func semverTag(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return false
	}
	if semver.IsValid(value) {
		return true
	}
	return semver.IsValid("v" + value)
}

func vaultPathTag(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return false
	}
	return regexp.MustCompile(vaultPathPattern).MatchString(value)
}

func customerBranchTag(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return false
	}
	return regexp.MustCompile(customerBranchPattern).MatchString(value)
}
