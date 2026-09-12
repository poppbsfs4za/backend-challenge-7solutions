package validator

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

type Validator struct {
	v *validator.Validate
}

func New() *Validator {
	return &Validator{v: validator.New()}
}

// Validate ต้องมี signature นี้เพื่อให้ Echo เรียกผ่าน c.Validate() ได้
func (cv *Validator) Validate(i any) error {
	if err := cv.v.Struct(i); err != nil {
		var errs validator.ValidationErrors
		if !errorsAs(err, &errs) {
			return err
		}
		msgs := make([]string, 0, len(errs))
		for _, e := range errs {
			msgs = append(msgs, describe(e))
		}
		return fmt.Errorf("%s", strings.Join(msgs, "; "))
	}
	return nil
}

func errorsAs(err error, target *validator.ValidationErrors) bool {
	e, ok := err.(validator.ValidationErrors)
	if ok {
		*target = e
	}
	return ok
}

// describe แปล error ของ validator เป็นข้อความที่คนอ่านรู้เรื่อง
func describe(e validator.FieldError) string {
	field := strings.ToLower(e.Field())
	switch e.Tag() {
	case "required":
		return field + " is required"
	case "email":
		return field + " must be a valid email address"
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, e.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, e.Param())
	default:
		return field + " is invalid"
	}
}
