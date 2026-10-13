// Package httpx chứa các mảnh HTTP dùng chung của server: JSON, lỗi, middleware,
// xác định IP client sau proxy.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ErrorBody là định dạng lỗi thống nhất: {"error":{"code":"...","message":"..."}}.
// message bằng tiếng Việt, hiển thị thẳng cho người dùng.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error là lỗi nghiệp vụ mang sẵn status HTTP và mã lỗi.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// NewError tạo lỗi nghiệp vụ.
func NewError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// Các lỗi hay dùng.
var (
	ErrNotFound     = NewError(http.StatusNotFound, "not_found", "Không tìm thấy")
	ErrUnauthorized = NewError(http.StatusUnauthorized, "unauthorized", "Bạn cần đăng nhập")
	ErrInternal     = NewError(http.StatusInternalServerError, "internal", "Lỗi máy chủ, vui lòng thử lại sau")
)

// WriteJSON ghi JSON với status cho trước.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError ghi lỗi chuẩn.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorBody{Error: ErrorDetail{Code: code, Message: message}})
}

// WriteErr ghi một error bất kỳ: *Error giữ nguyên status/code, lỗi khác thành 500
// (chi tiết chỉ vào log, không lộ ra client).
func WriteErr(w http.ResponseWriter, r *http.Request, err error) {
	var he *Error
	if errors.As(err, &he) {
		WriteError(w, he.Status, he.Code, he.Message)
		return
	}
	LoggerFrom(r.Context()).Error("lỗi xử lý request", "err", err)
	WriteError(w, ErrInternal.Status, ErrInternal.Code, ErrInternal.Message)
}

// Decode giải mã body JSON vào v: từ chối field lạ, body rỗng, dữ liệu thừa.
func Decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr):
			return NewError(http.StatusRequestEntityTooLarge, "body_too_large", "Dữ liệu gửi lên quá lớn")
		case errors.Is(err, io.EOF):
			return NewError(http.StatusBadRequest, "bad_request", "Thiếu dữ liệu gửi lên")
		default:
			return NewError(http.StatusBadRequest, "bad_request", fmt.Sprintf("Dữ liệu không hợp lệ: %s", decodeErrMessage(err)))
		}
	}
	if dec.More() {
		return NewError(http.StatusBadRequest, "bad_request", "Dữ liệu không hợp lệ: thừa nội dung sau JSON")
	}
	return nil
}

// decodeErrMessage chỉ giữ tên field, không trích giá trị người dùng gửi (có thể là mật khẩu/key).
func decodeErrMessage(err error) string {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return fmt.Sprintf("field %q sai kiểu", typeErr.Field)
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return "JSON sai cú pháp"
	}
	msg := err.Error()
	const unknown = "json: unknown field "
	if len(msg) > len(unknown) && msg[:len(unknown)] == unknown {
		return "field lạ " + msg[len(unknown):]
	}
	return "JSON không đọc được"
}
