package jsonschema

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestInvalidJSONTypeErrorIs(t *testing.T) {
	data := json.RawMessage(`{"type": invalid}`)
	err := invalidJSONTypeError(data)
	if !errors.Is(err, ErrInvalidJSON) {
		t.Errorf("%T did not register as ErrInvalidJSON", err)
	}

	err2 := invalidJSONTypeError(data)

	if !errors.Is(err, err2) {
		t.Errorf("%v should report true for %v", err, err2)
	}

	var ierr *InvalidJSONTypeError
	if !errors.As(err, &ierr) {
		t.Errorf("%v should be able to use errors.As for %T", err, ierr)
	}
}

func TestIsValidationError(t *testing.T) {
	var ve error = &ValidationError{}

	if !IsValidationError(ve) {
		t.Errorf("%v should be a validation error", ve)
	}
}

func TestInvalidJSONTypeAs(t *testing.T) {
	var err error = invalidJSONTypeError("error_value")

	var ptrerr *InvalidJSONTypeError

	if !errors.As(err, &ptrerr) {
		t.Errorf("%v should be able to use errors.As for %T", err, ptrerr)
	} else if string(*ptrerr) != "string" {
		t.Errorf("expected \"string\", got %s", string(*ptrerr))
	}

	var regerr InvalidJSONTypeError
	if !errors.As(err, &regerr) {
		t.Errorf("%v should be able to use errors.As for %T", err, err)
	} else if string(regerr) != "string" {
		t.Errorf("expected \"string\", got %s", string(regerr))
	}
}

func TestInfiniteLoopErrorAs(t *testing.T) {
	var err error = InfiniteLoopError("infinite_loop")

	var ptrerr *InfiniteLoopError

	if !errors.As(err, &ptrerr) {
		t.Errorf("%v should be able to use errors.As for %T", err, ptrerr)
	} else if string(*ptrerr) != "infinite_loop" {
		t.Errorf("expected \"infinite_loop\", got %s", string(*ptrerr))
	}

	var regerr InfiniteLoopError
	if !errors.As(err, &regerr) {
		t.Errorf("%v should be able to use errors.As for %T", err, err)
	} else if string(regerr) != "infinite_loop" {
		t.Errorf("expected \"infinite_loop\", got %s", string(regerr))
	}
}
