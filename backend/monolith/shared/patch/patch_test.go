package patch

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type testDomain struct {
	Id        string     `db:"id"`
	Name      string     `db:"name"`
	Email     string     `db:"email"`
	Password  string     `db:"password"`
	Role      string     `db:"role"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

type testPatch struct {
	Name     *string `json:"name,omitempty"`
	Email    *string `json:"email,omitempty"`
	Password *string `json:"password,omitempty"`
	Role     *string `json:"role,omitempty"`
}

func strPtr(s string) *string { return &s }

// --- Basic ---

func TestPatch_AutoMerge(t *testing.T) {
	dst := testDomain{Id: "1", Name: "old", Email: "old@test.com"}
	patch := testPatch{Name: strPtr("new")}

	res, err := Patch(&dst, patch,
		WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "new" {
		t.Errorf("Name = %q, want %q", dst.Name, "new")
	}
	if dst.Email != "old@test.com" {
		t.Errorf("Email should not change, got %q", dst.Email)
	}
	if len(res.Updated) != 1 || res.Updated[0] != "Name" {
		t.Errorf("Updated = %v, want [Name]", res.Updated)
	}
}

func TestPatch_NilFieldsSkipped(t *testing.T) {
	dst := testDomain{Id: "1", Name: "original"}
	_, err := Patch(&dst, testPatch{}, WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "original" {
		t.Errorf("Name = %q, want %q", dst.Name, "original")
	}
}

func TestPatch_MultipleFields(t *testing.T) {
	dst := testDomain{Id: "1", Name: "old", Email: "old@test.com"}
	patch := testPatch{Name: strPtr("new"), Email: strPtr("new@test.com")}

	res, err := Patch(&dst, patch, WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "new" || dst.Email != "new@test.com" {
		t.Errorf("unexpected: Name=%q Email=%q", dst.Name, dst.Email)
	}
	if len(res.Updated) != 2 {
		t.Errorf("Updated = %v, want 2", res.Updated)
	}
}

func TestPatch_PatchAsPointer(t *testing.T) {
	dst := testDomain{Id: "1", Name: "old"}
	_, err := Patch(&dst, &testPatch{Name: strPtr("new")}, WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "new" {
		t.Errorf("Name = %q, want %q", dst.Name, "new")
	}
}

// --- Ignore ---

func TestPatch_IgnoreViaOption(t *testing.T) {
	now := time.Now()
	dst := testDomain{Id: "1", Name: "old", CreatedAt: now, UpdatedAt: now}
	_, err := Patch(&dst, testPatch{Name: strPtr("new")}, WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dst.CreatedAt.Equal(now) || !dst.UpdatedAt.Equal(now) {
		t.Errorf("timestamp fields were modified")
	}
}

func TestPatch_IgnoreViaTag(t *testing.T) {
	type tagged struct {
		Name string `patch:"ignore"`
	}
	type taggedPatch struct {
		Name *string
	}
	dst := tagged{Name: "original"}
	_, err := Patch(&dst, taggedPatch{Name: strPtr("new")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "original" {
		t.Errorf("ignore-tagged field was modified: %q", dst.Name)
	}
}

func TestPatch_OptionOverridesTag(t *testing.T) {
	type tagged struct {
		Name string `patch:"manual"`
	}
	type taggedPatch struct {
		Name *string
	}
	dst := tagged{Name: "old"}
	_, err := Patch(&dst, taggedPatch{Name: strPtr("new")}, WithIgnore("Name"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "old" {
		t.Errorf("option-ignored field was modified: %q", dst.Name)
	}
}

// --- Manual ---

func TestPatch_ManualViaOption(t *testing.T) {
	dst := testDomain{Id: "1", Name: "old"}
	patch := testPatch{Name: strPtr("new"), Password: strPtr("secret123")}

	res, err := Patch(&dst, patch,
		WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"),
		WithManual("Password"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "new" {
		t.Errorf("Name = %q, want %q", dst.Name, "new")
	}
	if dst.Password != "" {
		t.Errorf("Password should NOT be auto-merged, got %q", dst.Password)
	}
	if len(res.Manual) != 1 || res.Manual[0].Name != "Password" {
		t.Errorf("Manual = %v, want [{Password secret123}]", res.Manual)
	}
}

func TestPatch_ManualViaTag(t *testing.T) {
	type tagged struct {
		Name  string `patch:"manual"`
		Email string
	}
	type taggedPatch struct {
		Name  *string
		Email *string
	}
	dst := tagged{Name: "old", Email: "old@test.com"}
	res, err := Patch(&dst, taggedPatch{Name: strPtr("new"), Email: strPtr("new@test.com")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Email != "new@test.com" {
		t.Errorf("Email = %q", dst.Email)
	}
	if dst.Name != "old" {
		t.Errorf("manual-tagged field was auto-merged: %q", dst.Name)
	}
	if len(res.Manual) != 1 || res.Manual[0].Name != "Name" {
		t.Errorf("Manual = %v", res.Manual)
	}
}

func TestPatch_ManualNilSkipped(t *testing.T) {
	type tagged struct {
		Name string `patch:"manual"`
	}
	type taggedPatch struct {
		Name *string
	}
	dst := tagged{Name: "old"}
	res, err := Patch(&dst, taggedPatch{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Manual) != 0 {
		t.Errorf("Manual = %v, want empty", res.Manual)
	}
}

// --- ManualHandler ---

func TestPatch_ManualHandler(t *testing.T) {
	dst := testDomain{Id: "1", Name: "old", Password: "old-hash"}
	patch := testPatch{Password: strPtr("plaintext")}

	var handled []string
	res, err := Patch(&dst, patch,
		WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"),
		WithManual("Password"),
		WithManualHandler(func(name string, value any) error {
			handled = append(handled, name)
			if name == "Password" {
				dst.Password = "hashed-" + value.(string)
			}
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Password != "hashed-plaintext" {
		t.Errorf("Password = %q, want %q", dst.Password, "hashed-plaintext")
	}
	if len(handled) != 1 || handled[0] != "Password" {
		t.Errorf("handled = %v, want [Password]", handled)
	}
	if len(res.Manual) != 1 {
		t.Errorf("Manual should still contain the field")
	}
}

func TestPatch_ManualHandlerError(t *testing.T) {
	dst := testDomain{Id: "1"}
	patch := testPatch{Password: strPtr("secret")}

	_, err := Patch(&dst, patch,
		WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"),
		WithManual("Password"),
		WithManualHandler(func(name string, value any) error {
			return errors.New("handler failed")
		}),
	)
	if err == nil {
		t.Fatal("expected error from manual handler")
	}
	if !strings.Contains(err.Error(), "handler failed") {
		t.Errorf("error = %q", err.Error())
	}
}

// --- WithBefore (multiple) ---

func TestPatch_MultipleBeforeHooks(t *testing.T) {
	dst := testDomain{Id: "1", Name: "  hello  "}
	patch := testPatch{Name: strPtr("  hello  ")}

	_, err := Patch(&dst, patch,
		WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"),
		WithBefore(func(name string, value any) (any, error) {
			if name == "Name" {
				return strings.TrimSpace(value.(string)), nil
			}
			return value, nil
		}),
		WithBefore(func(name string, value any) (any, error) {
			if name == "Name" {
				return strings.ToUpper(value.(string)), nil
			}
			return value, nil
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "HELLO" {
		t.Errorf("Name = %q, want %q (trim then upper)", dst.Name, "HELLO")
	}
}

func TestPatch_BeforeHookError(t *testing.T) {
	dst := testDomain{Id: "1"}
	patch := testPatch{Name: strPtr("new")}

	_, err := Patch(&dst, patch,
		WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"),
		WithBefore(func(name string, value any) (any, error) {
			return nil, errors.New("transform failed")
		}),
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "transform failed") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestPatch_BeforeHookReturnsNil(t *testing.T) {
	dst := testDomain{Id: "1"}
	patch := testPatch{Name: strPtr("new")}

	_, err := Patch(&dst, patch,
		WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"),
		WithBefore(func(name string, value any) (any, error) {
			return nil, nil
		}),
	)
	if err == nil {
		t.Fatal("expected error for nil return")
	}
	if !strings.Contains(err.Error(), "transform returned nil") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestPatch_BeforeNotCalledForManualFields(t *testing.T) {
	dst := testDomain{Id: "1"}
	patch := testPatch{Password: strPtr("secret")}

	hookCalled := false
	_, err := Patch(&dst, patch,
		WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"),
		WithManual("Password"),
		WithBefore(func(name string, value any) (any, error) {
			hookCalled = true
			return value, nil
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hookCalled {
		t.Error("before hook should NOT be called for manual fields")
	}
}

// --- ErrSkip ---

func TestPatch_ErrSkip(t *testing.T) {
	dst := testDomain{Id: "1", Name: "old"}
	patch := testPatch{Name: strPtr("new")}

	res, err := Patch(&dst, patch,
		WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"),
		WithBefore(func(name string, value any) (any, error) {
			return nil, ErrSkip
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "old" {
		t.Errorf("Name was modified despite ErrSkip: %q", dst.Name)
	}
	if len(res.Updated) != 0 {
		t.Errorf("Updated = %v, want empty", res.Updated)
	}
}

func TestPatch_ErrSkipInGlobalOnlyAffectsSkippedField(t *testing.T) {
	dst := testDomain{Id: "1", Name: "old", Email: "old@test.com"}
	patch := testPatch{Name: strPtr("new"), Email: strPtr("new@test.com")}

	res, err := Patch(&dst, patch,
		WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"),
		WithBefore(func(name string, value any) (any, error) {
			if name == "Name" {
				return nil, ErrSkip
			}
			return value, nil
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "old" {
		t.Errorf("Name should be unchanged: %q", dst.Name)
	}
	if dst.Email != "new@test.com" {
		t.Errorf("Email = %q, want %q", dst.Email, "new@test.com")
	}
	if len(res.Updated) != 1 || res.Updated[0] != "Email" {
		t.Errorf("Updated = %v, want [Email]", res.Updated)
	}
}

// --- WithTransform (per-field) ---

func TestPatch_TransformPerField(t *testing.T) {
	dst := testDomain{Id: "1", Name: "old", Email: "OLD@TEST.COM"}
	patch := testPatch{Name: strPtr("new"), Email: strPtr("OLD@TEST.COM")}

	_, err := Patch(&dst, patch,
		WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"),
		WithTransform("Email", func(name string, value any) (any, error) {
			return strings.ToLower(value.(string)), nil
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "new" {
		t.Errorf("Name = %q", dst.Name)
	}
	if dst.Email != "old@test.com" {
		t.Errorf("Email = %q, want %q", dst.Email, "old@test.com")
	}
}

func TestPatch_TransformOverridesBefore(t *testing.T) {
	dst := testDomain{Id: "1", Password: "old"}
	patch := testPatch{Password: strPtr("secret")}

	_, err := Patch(&dst, patch,
		WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"),
		WithTransform("Password", func(name string, value any) (any, error) {
			return "hashed-" + value.(string), nil
		}),
		WithBefore(func(name string, value any) (any, error) {
			// should NOT be called for Password
			t.Error("before hook should not be called when per-field transform exists")
			return value, nil
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Password != "hashed-secret" {
		t.Errorf("Password = %q", dst.Password)
	}
}

func TestPatch_TransformErrSkip(t *testing.T) {
	dst := testDomain{Id: "1", Name: "old"}
	patch := testPatch{Name: strPtr("new")}

	res, err := Patch(&dst, patch,
		WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"),
		WithTransform("Name", func(name string, value any) (any, error) {
			return nil, ErrSkip
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "old" {
		t.Errorf("Name was modified despite ErrSkip: %q", dst.Name)
	}
	if len(res.Updated) != 0 {
		t.Errorf("Updated = %v, want empty", res.Updated)
	}
}

func TestPatch_TransformTypeMismatch(t *testing.T) {
	dst := testDomain{Id: "1"}
	patch := testPatch{Name: strPtr("new")}

	_, err := Patch(&dst, patch,
		WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"),
		WithTransform("Name", func(name string, value any) (any, error) {
			return 12345, nil
		}),
	)
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
	if !strings.Contains(err.Error(), "type mismatch") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestPatch_TransformNotCalledForNilPatch(t *testing.T) {
	dst := testDomain{Id: "1"}

	called := false
	_, err := Patch(&dst, testPatch{},
		WithIgnore("CreatedAt", "UpdatedAt", "DeletedAt"),
		WithTransform("Name", func(name string, value any) (any, error) {
			called = true
			return value, nil
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("transform should not be called when patch field is nil")
	}
}

// --- Option conflict ---

func TestPatch_IgnoreManualConflict(t *testing.T) {
	dst := testDomain{Id: "1"}
	patch := testPatch{Name: strPtr("new")}

	_, err := Patch(&dst, patch,
		WithIgnore("Name"),
		WithManual("Name"),
	)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !strings.Contains(err.Error(), "both ignore and manual") {
		t.Errorf("error = %q", err.Error())
	}
}

// --- Safety ---

func TestPatch_TypeMismatchError(t *testing.T) {
	type dstStruct struct {
		Count int
	}
	type patchStruct struct {
		Count *string
	}
	_, err := Patch(&dstStruct{}, patchStruct{Count: strPtr("x")})
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
}

func TestPatch_NonPointerPatchFieldError(t *testing.T) {
	type dstStruct struct {
		Name string
	}
	type patchStruct struct {
		Name string
	}
	_, err := Patch(&dstStruct{}, patchStruct{Name: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "must be a pointer") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestPatch_InvalidDst(t *testing.T) {
	_, err := Patch("nope", testPatch{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPatch_InvalidPatch(t *testing.T) {
	dst := testDomain{Id: "1"}
	_, err := Patch(&dst, "nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPatch_NilDst(t *testing.T) {
	_, err := Patch((*testDomain)(nil), testPatch{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPatch_NilPatch(t *testing.T) {
	dst := testDomain{Id: "1"}
	_, err := Patch(&dst, (*testPatch)(nil))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPatch_UnexportedSkipped(t *testing.T) {
	type dstStruct struct {
		Name   string
		secret string
	}
	type patchStruct struct {
		Name   *string
		secret *string
	}
	dst := dstStruct{Name: "old"}
	_, err := Patch(&dst, patchStruct{Name: strPtr("new"), secret: strPtr("hacked")})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if dst.Name != "new" {
		t.Errorf("Name = %q", dst.Name)
	}
}

func TestPatch_AnonymousSkipped(t *testing.T) {
	type Base struct {
		Id string
	}
	type dstStruct struct {
		Base
		Name string
	}
	type patchStruct struct {
		Id   *string
		Name *string
	}
	dst := dstStruct{Base: Base{Id: "1"}, Name: "old"}
	_, err := Patch(&dst, patchStruct{Id: strPtr("2"), Name: strPtr("new")})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if dst.Id != "1" {
		t.Errorf("embedded Id modified: %q", dst.Id)
	}
	if dst.Name != "new" {
		t.Errorf("Name = %q", dst.Name)
	}
}

// --- Integration ---

func TestPatch_RealWorldUserUpdate(t *testing.T) {
	type User struct {
		Id        string
		Name      string
		Email     string
		Password  string
		CreatedAt time.Time
		UpdatedAt time.Time
	}
	type UserPatch struct {
		Name     *string
		Email    *string
		Password *string
	}

	now := time.Now()
	user := User{Id: "u1", Name: "old", Email: "OLD@test.com", Password: "hash", CreatedAt: now, UpdatedAt: now}

	result, err := Patch(&user, UserPatch{
		Name:     strPtr("new"),
		Email:    strPtr("NEW@TEST.COM"),
		Password: strPtr("plaintext"),
	},
		WithIgnore("CreatedAt", "UpdatedAt"),
		WithManual("Password"),
		WithTransform("Email", func(name string, value any) (any, error) {
			return strings.ToLower(value.(string)), nil
		}),
		WithManualHandler(func(name string, value any) error {
			if name == "Password" {
				user.Password = "hashed-" + value.(string)
			}
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.Name != "new" {
		t.Errorf("Name = %q", user.Name)
	}
	if user.Email != "new@test.com" {
		t.Errorf("Email = %q, want %q", user.Email, "new@test.com")
	}
	if user.Password != "hashed-plaintext" {
		t.Errorf("Password = %q", user.Password)
	}
	if !user.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt modified")
	}
	if len(result.Updated) != 2 {
		t.Errorf("Updated = %v, want [Name Email]", result.Updated)
	}
	if len(result.Manual) != 1 {
		t.Errorf("Manual = %v, want 1", result.Manual)
	}
}
