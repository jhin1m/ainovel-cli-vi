package api

import "time"

// User là tài khoản đã đăng nhập.
type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	IsAdmin     bool      `json:"is_admin"`
	CreatedAt   time.Time `json:"created_at"`
}

// SignupRequest: POST /api/auth/signup.
type SignupRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	InviteCode  string `json:"invite_code"`
}

// LoginRequest: POST /api/auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ChangePasswordRequest: POST /api/me/password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// APIKey là key LLM đã lưu. Không bao giờ chứa key rõ, chỉ 4 ký tự cuối.
type APIKey struct {
	ID        string    `json:"id"`
	Provider  string    `json:"provider"`
	Label     string    `json:"label"`
	Last4     string    `json:"last4"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateAPIKeyRequest: POST /api/keys. Phải nhập lại mật khẩu.
type CreateAPIKeyRequest struct {
	Provider        string `json:"provider"`
	Label           string `json:"label"`
	APIKey          string `json:"api_key"`
	CurrentPassword string `json:"current_password"`
}

// ProviderInfo: một provider được phép và các model có giá (GET /api/providers).
type ProviderInfo struct {
	Name   string        `json:"name"`
	Label  string        `json:"label"`
	Models []ModelOption `json:"models"`
}

// ModelOption là model chọn được cho truyện; giá USD / 1M token.
type ModelOption struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	ContextWindow   int     `json:"context_window"`
	InputCostPer1M  float64 `json:"input_cost_per_1m"`
	OutputCostPer1M float64 `json:"output_cost_per_1m"`
}

// ErrorResponse là định dạng lỗi chung của mọi API.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail: code ổn định cho máy, message tiếng Việt cho người.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
