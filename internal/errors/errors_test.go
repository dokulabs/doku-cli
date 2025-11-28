package errors

import (
	"errors"
	"testing"
)

func TestServiceError(t *testing.T) {
	baseErr := errors.New("connection refused")
	svcErr := NewServiceError("postgres", baseErr)

	if svcErr.Service != "postgres" {
		t.Errorf("Service = %q, want %q", svcErr.Service, "postgres")
	}

	if !errors.Is(svcErr, baseErr) {
		t.Error("ServiceError should unwrap to base error")
	}

	expected := "service 'postgres': connection refused"
	if svcErr.Error() != expected {
		t.Errorf("Error() = %q, want %q", svcErr.Error(), expected)
	}
}

func TestContainerError(t *testing.T) {
	baseErr := errors.New("no such container")
	containerErr := NewContainerError("doku-postgres", "start", baseErr)

	if containerErr.ContainerName != "doku-postgres" {
		t.Errorf("ContainerName = %q, want %q", containerErr.ContainerName, "doku-postgres")
	}

	if containerErr.Action != "start" {
		t.Errorf("Action = %q, want %q", containerErr.Action, "start")
	}

	if !errors.Is(containerErr, baseErr) {
		t.Error("ContainerError should unwrap to base error")
	}

	expected := "container 'doku-postgres' start: no such container"
	if containerErr.Error() != expected {
		t.Errorf("Error() = %q, want %q", containerErr.Error(), expected)
	}
}

func TestConfigError(t *testing.T) {
	t.Run("with underlying error", func(t *testing.T) {
		baseErr := errors.New("parse error")
		cfgErr := NewConfigError("port", "invalid port number", baseErr)

		if cfgErr.Field != "port" {
			t.Errorf("Field = %q, want %q", cfgErr.Field, "port")
		}

		if !errors.Is(cfgErr, baseErr) {
			t.Error("ConfigError should unwrap to base error")
		}

		expected := "config error in 'port': invalid port number: parse error"
		if cfgErr.Error() != expected {
			t.Errorf("Error() = %q, want %q", cfgErr.Error(), expected)
		}
	})

	t.Run("without underlying error", func(t *testing.T) {
		cfgErr := NewConfigError("domain", "cannot be empty", nil)

		expected := "config error in 'domain': cannot be empty"
		if cfgErr.Error() != expected {
			t.Errorf("Error() = %q, want %q", cfgErr.Error(), expected)
		}
	})
}

func TestValidationError(t *testing.T) {
	valErr := NewValidationError()

	if valErr.HasErrors() {
		t.Error("New ValidationError should not have errors")
	}

	valErr.Add("port is required")
	valErr.Add("name cannot be empty")

	if !valErr.HasErrors() {
		t.Error("ValidationError should have errors after Add")
	}

	if len(valErr.Errors) != 2 {
		t.Errorf("len(Errors) = %d, want 2", len(valErr.Errors))
	}

	expected := "validation failed: port is required; name cannot be empty"
	if valErr.Error() != expected {
		t.Errorf("Error() = %q, want %q", valErr.Error(), expected)
	}
}

func TestWrap(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		result := Wrap(nil, "context")
		if result != nil {
			t.Error("Wrap(nil) should return nil")
		}
	})

	t.Run("with error", func(t *testing.T) {
		baseErr := errors.New("base error")
		result := Wrap(baseErr, "failed to %s", "connect")

		if result == nil {
			t.Error("Wrap should not return nil for non-nil error")
		}

		if !errors.Is(result, baseErr) {
			t.Error("Wrapped error should contain base error")
		}

		expected := "failed to connect: base error"
		if result.Error() != expected {
			t.Errorf("Error() = %q, want %q", result.Error(), expected)
		}
	})
}

func TestWrapService(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		result := WrapService("postgres", nil)
		if result != nil {
			t.Error("WrapService(nil) should return nil")
		}
	})

	t.Run("with error", func(t *testing.T) {
		baseErr := errors.New("connection failed")
		result := WrapService("redis", baseErr)

		var svcErr *ServiceError
		if !errors.As(result, &svcErr) {
			t.Error("WrapService should return ServiceError")
		}

		if svcErr.Service != "redis" {
			t.Errorf("Service = %q, want %q", svcErr.Service, "redis")
		}
	})
}

func TestWrapContainer(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		result := WrapContainer("container", "start", nil)
		if result != nil {
			t.Error("WrapContainer(nil) should return nil")
		}
	})

	t.Run("with error", func(t *testing.T) {
		baseErr := errors.New("container exited")
		result := WrapContainer("doku-postgres", "run", baseErr)

		var containerErr *ContainerError
		if !errors.As(result, &containerErr) {
			t.Error("WrapContainer should return ContainerError")
		}

		if containerErr.ContainerName != "doku-postgres" {
			t.Errorf("ContainerName = %q, want %q", containerErr.ContainerName, "doku-postgres")
		}

		if containerErr.Action != "run" {
			t.Errorf("Action = %q, want %q", containerErr.Action, "run")
		}
	})
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"ErrNotFound", ErrNotFound, true},
		{"wrapped ErrNotFound", Wrap(ErrNotFound, "service"), true},
		{"contains 'not found'", errors.New("resource not found"), true},
		{"contains 'no such'", errors.New("no such file"), true},
		{"unrelated error", errors.New("connection refused"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNotFound(tt.err)
			if result != tt.expected {
				t.Errorf("IsNotFound() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsAlreadyExists(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"ErrAlreadyExists", ErrAlreadyExists, true},
		{"wrapped ErrAlreadyExists", Wrap(ErrAlreadyExists, "volume"), true},
		{"contains 'already exists'", errors.New("container already exists"), true},
		{"contains 'duplicate'", errors.New("duplicate key"), true},
		{"unrelated error", errors.New("connection refused"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAlreadyExists(tt.err)
			if result != tt.expected {
				t.Errorf("IsAlreadyExists() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsPermissionDenied(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"ErrPermissionDenied", ErrPermissionDenied, true},
		{"wrapped ErrPermissionDenied", Wrap(ErrPermissionDenied, "file"), true},
		{"contains 'permission denied'", errors.New("permission denied for file"), true},
		{"contains 'access denied'", errors.New("access denied"), true},
		{"unrelated error", errors.New("connection refused"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPermissionDenied(tt.err)
			if result != tt.expected {
				t.Errorf("IsPermissionDenied() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIs(t *testing.T) {
	err := Wrap(ErrNotFound, "service")
	if !Is(err, ErrNotFound) {
		t.Error("Is should return true for wrapped error")
	}
}

func TestAs(t *testing.T) {
	baseErr := errors.New("test")
	svcErr := NewServiceError("postgres", baseErr)

	var target *ServiceError
	if !As(svcErr, &target) {
		t.Error("As should succeed for ServiceError")
	}

	if target.Service != "postgres" {
		t.Errorf("Service = %q, want %q", target.Service, "postgres")
	}
}

func TestMust(t *testing.T) {
	// Test with nil error - should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Error("Must(nil) should not panic")
		}
	}()
	Must(nil)
}

func TestMustPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Must(error) should panic")
		}
	}()
	Must(errors.New("test error"))
}

func TestMustValue(t *testing.T) {
	// Test with nil error - should return value
	result := MustValue("hello", nil)
	if result != "hello" {
		t.Errorf("MustValue() = %q, want %q", result, "hello")
	}
}

func TestMustValuePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustValue with error should panic")
		}
	}()
	MustValue("hello", errors.New("test error"))
}
