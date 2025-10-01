package validation

import (
	"testing"

	"encore.dev/beta/errs"
)

// Test structs with various validation rules
type TestStruct struct {
	Name        string `validate:"required,min=2,max=10"`
	Email       string `validate:"required,email"`
	Age         int    `validate:"required,min=18,max=100"`
	Optional    string `validate:"omitempty,min=1"`
	Phone       string `validate:"omitempty,len=10"`
	Status      string `validate:"required,oneof=active inactive"`
	Description string `validate:"omitempty,max=100"`
}

type EmptyStruct struct{}

type SimpleStruct struct {
	Value string `validate:"required"`
}

type ComplexStruct struct {
	ID       int    `validate:"required,min=1"`
	Name     string `validate:"required,min=2"`
	Email    string `validate:"required,email"`
	Age      int    `validate:"required,min=18,max=65"`
	Status   string `validate:"required,oneof=open closed pending"`
	Optional string `validate:"omitempty,min=1"`
}

func TestStruct_ValidInput(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{
			name: "Valid TestStruct",
			input: TestStruct{
				Name:        "John",
				Email:       "john@example.com",
				Age:         25,
				Optional:    "optional",
				Phone:       "1234567890",
				Status:      "active",
				Description: "Valid description",
			},
		},
		{
			name: "Valid TestStruct with minimal required fields",
			input: TestStruct{
				Name:   "Jo",
				Email:  "jo@test.com",
				Age:    18,
				Status: "inactive",
			},
		},
		{
			name:  "Empty struct (no validation rules)",
			input: EmptyStruct{},
		},
		{
			name: "Valid SimpleStruct",
			input: SimpleStruct{
				Value: "test",
			},
		},
		{
			name: "Valid ComplexStruct",
			input: ComplexStruct{
				ID:     1,
				Name:   "Test",
				Email:  "test@example.com",
				Age:    25,
				Status: "open",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Struct(tt.input)
			if err != nil {
				t.Errorf("Struct() returned error for valid input: %v", err)
			}
		})
	}
}

func TestStruct_InvalidInput(t *testing.T) {
	tests := []struct {
		name          string
		input         any
		expectedField string
		expectedTag   string
		expectedCode  errs.ErrCode
	}{
		{
			name: "Missing required field",
			input: TestStruct{
				Email:  "john@example.com",
				Age:    25,
				Status: "active",
			},
			expectedField: "Name",
			expectedTag:   "required",
			expectedCode:  errs.InvalidArgument,
		},
		{
			name: "Name too short",
			input: TestStruct{
				Name:   "J",
				Email:  "john@example.com",
				Age:    25,
				Status: "active",
			},
			expectedField: "Name",
			expectedTag:   "min",
			expectedCode:  errs.InvalidArgument,
		},
		{
			name: "Name too long",
			input: TestStruct{
				Name:   "VeryLongName",
				Email:  "john@example.com",
				Age:    25,
				Status: "active",
			},
			expectedField: "Name",
			expectedTag:   "max",
			expectedCode:  errs.InvalidArgument,
		},
		{
			name: "Invalid email",
			input: TestStruct{
				Name:   "John",
				Email:  "invalid-email",
				Age:    25,
				Status: "active",
			},
			expectedField: "Email",
			expectedTag:   "email",
			expectedCode:  errs.InvalidArgument,
		},
		{
			name: "Age too young",
			input: TestStruct{
				Name:   "John",
				Email:  "john@example.com",
				Age:    17,
				Status: "active",
			},
			expectedField: "Age",
			expectedTag:   "min",
			expectedCode:  errs.InvalidArgument,
		},
		{
			name: "Age too old",
			input: TestStruct{
				Name:   "John",
				Email:  "john@example.com",
				Age:    101,
				Status: "active",
			},
			expectedField: "Age",
			expectedTag:   "max",
			expectedCode:  errs.InvalidArgument,
		},
		{
			name: "Invalid status",
			input: TestStruct{
				Name:   "John",
				Email:  "john@example.com",
				Age:    25,
				Status: "invalid",
			},
			expectedField: "Status",
			expectedTag:   "oneof",
			expectedCode:  errs.InvalidArgument,
		},
		{
			name: "Phone number wrong length",
			input: TestStruct{
				Name:   "John",
				Email:  "john@example.com",
				Age:    25,
				Status: "active",
				Phone:  "123", // Too short
			},
			expectedField: "Phone",
			expectedTag:   "len",
			expectedCode:  errs.InvalidArgument,
		},
		{
			name: "Description too long",
			input: TestStruct{
				Name:        "John",
				Email:       "john@example.com",
				Age:         25,
				Status:      "active",
				Description: "This is a very long description that exceeds the maximum length of 100 characters and should fail validation",
			},
			expectedField: "Description",
			expectedTag:   "max",
			expectedCode:  errs.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Struct(tt.input)
			if err == nil {
				t.Errorf("Struct() should have returned error for invalid input")
				return
			}

			// Check if it's an Encore error
			encoreErr, ok := err.(*errs.Error)
			if !ok {
				t.Errorf("Expected Encore error, got %T", err)
				return
			}

			// Check error code
			if encoreErr.Code != tt.expectedCode {
				t.Errorf("Expected error code %v, got %v", tt.expectedCode, encoreErr.Code)
			}

			// Check error message contains expected field and tag
			msg := encoreErr.Message
			if !contains(msg, tt.expectedField) {
				t.Errorf("Error message should contain field '%s', got: %s", tt.expectedField, msg)
			}
			if !contains(msg, tt.expectedTag) {
				t.Errorf("Error message should contain tag '%s', got: %s", tt.expectedTag, msg)
			}
		})
	}
}

func TestStruct_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{
			name:  "Nil input",
			input: nil,
		},
		{
			name:  "Non-struct input",
			input: "not a struct",
		},
		{
			name:  "Pointer to struct",
			input: &TestStruct{Name: "John", Email: "john@example.com", Age: 25, Status: "active"},
		},
		{
			name:  "Interface input",
			input: interface{}(TestStruct{Name: "John", Email: "john@example.com", Age: 25, Status: "active"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These should either pass validation or return appropriate errors
			err := Struct(tt.input)
			// We don't assert specific behavior here since it depends on validator implementation
			// but we ensure the function doesn't panic
			_ = err
		})
	}
}

func TestStruct_ErrorMessageFormat(t *testing.T) {
	input := TestStruct{
		Email:  "john@example.com",
		Age:    25,
		Status: "active",
		// Name is missing (required)
	}

	err := Struct(input)
	if err == nil {
		t.Fatal("Expected validation error")
	}

	encoreErr, ok := err.(*errs.Error)
	if !ok {
		t.Fatalf("Expected Encore error, got %T", err)
	}

	// Check error message format
	expectedPrefix := "Validation failed for field 'Name' with rule 'required'"
	if encoreErr.Message != expectedPrefix {
		t.Errorf("Expected error message '%s', got '%s'", expectedPrefix, encoreErr.Message)
	}
}

func TestStruct_MultipleErrors(t *testing.T) {
	// Test that only the first error is returned
	input := TestStruct{
		// Multiple fields missing/invalid
		Email:  "invalid-email",
		Age:    5, // Too young
		Status: "invalid-status",
	}

	err := Struct(input)
	if err == nil {
		t.Fatal("Expected validation error")
	}

	encoreErr, ok := err.(*errs.Error)
	if !ok {
		t.Fatalf("Expected Encore error, got %T", err)
	}

	// Should return the first error (Name is required)
	if !contains(encoreErr.Message, "Name") || !contains(encoreErr.Message, "required") {
		t.Errorf("Expected first error about Name field, got: %s", encoreErr.Message)
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && contains(s[1:], substr)
}
