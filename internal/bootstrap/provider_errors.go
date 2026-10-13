package bootstrap

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/voocel/litellm"
)

// Nhóm lỗi provider (tập đóng). Worker của web và phần kiểm tra key dùng chung
// để mọi nơi gọi cùng một tên cho cùng một nguyên nhân.
const (
	ProviderErrAuthFailed   = "auth_failed"
	ProviderErrPlanMismatch = "plan_mismatch"
	ProviderErrRateLimited  = "rate_limited"
	ProviderErrQuota        = "quota"
	ProviderErrNetwork      = "network"
	ProviderErrUnknown      = "unknown"
)

// ErrKindProviderRejected là nhãn Event.Kind của host cho lỗi provider từ chối
// hẳn, và cũng là tiền tố ổn định trong Error() của ProviderRejectedError.
// Lỗi bị ép thành chuỗi (vd. trong kết quả JSON của sub-agent) vẫn nhận ra nhờ tiền tố này.
const ErrKindProviderRejected = "provider_rejected"

const providerRejectedPrefix = ErrKindProviderRejected + ":"

// providerCodeAllowlist: chỉ các mã gốc này được đưa ra ngoài (message cho user,
// response kiểm key). Mã khác có thể chứa nội dung tuỳ ý của provider nên bị bỏ.
var providerCodeAllowlist = map[string]bool{
	"1113": true, // z.ai/Zhipu: sai gói, hết quota, hoặc model không có trong gói
}

var (
	// Code dạng số trong body JSON thì litellm bỏ qua (chỉ đọc chuỗi), nên phải dò
	// trong message. Khớp nguyên từ để "11130" không bị nhận nhầm.
	code1113Pattern = regexp.MustCompile(`\b1113\b`)
	// rejectedCodePattern đọc lại mã từ Error() của ProviderRejectedError.
	rejectedCodePattern = regexp.MustCompile(`\bcode=(\S+)`)
)

// ClassifyProviderError xếp lỗi gọi model vào một nhóm cố định. code là mã gốc
// của provider, chỉ khác rỗng khi nằm trong allowlist.
func ClassifyProviderError(err error) (kind, code string) {
	if err == nil {
		return "", ""
	}
	var rejected *ProviderRejectedError
	if errors.As(err, &rejected) {
		return rejected.Kind, rejected.Code
	}
	var le *litellm.LiteLLMError
	if !errors.As(err, &le) {
		var netErr net.Error
		if errors.As(err, &netErr) {
			return ProviderErrNetwork, ""
		}
		return ProviderErrUnknown, ""
	}
	if isPlanMismatch(le) {
		return ProviderErrPlanMismatch, "1113"
	}
	if providerCodeAllowlist[le.Code] {
		code = le.Code
	}
	switch le.Type {
	case litellm.ErrorTypeAuth:
		return ProviderErrAuthFailed, code
	case litellm.ErrorTypeQuota:
		return ProviderErrQuota, code
	case litellm.ErrorTypeRateLimit:
		return ProviderErrRateLimited, code
	case litellm.ErrorTypeNetwork, litellm.ErrorTypeTimeout:
		return ProviderErrNetwork, code
	}
	return ProviderErrUnknown, code
}

// isPlanMismatch nhận diện mã 1113 của z.ai/Zhipu: HTTP 429 nhưng không phải
// rate limit, nên retry không bao giờ thành công.
func isPlanMismatch(le *litellm.LiteLLMError) bool {
	if le.Code != "" {
		return le.Code == "1113"
	}
	// Chỉ dò message với provider loại glm (z.ai, bigmodel.cn) để tránh khớp nhầm.
	// Giới hạn: body {"error":{"code":1113,...}} (code số lồng trong error) bị
	// litellm bỏ code và message không chứa mã, nên không nhận ra được. Docs z.ai
	// trả code dạng chuỗi nên hiện chưa xử lý.
	return le.Provider == "glm" && code1113Pattern.MatchString(le.Message)
}

// ProviderRejectedError đánh dấu lỗi provider mà gọi lại bao nhiêu lần cũng
// không qua (sai gói, hết quota). Nó nằm ngoài cùng chuỗi lỗi nên vòng retry của
// agentcore và llmretry (đều lấy RetryableError đầu tiên trong chuỗi) thấy
// Retryable() == false; Unwrap giữ lỗi gốc cho ai cần đọc chi tiết.
type ProviderRejectedError struct {
	Kind     string
	Code     string
	Provider string
	Model    string
	Err      error
}

func (e *ProviderRejectedError) Error() string {
	return fmt.Sprintf("%s %s code=%s provider=%s model=%s: %v",
		providerRejectedPrefix, e.Kind, e.Code, e.Provider, e.Model, e.Err)
}

func (e *ProviderRejectedError) Unwrap() error { return e.Err }

// Retryable luôn false: thử lại chỉ tốn thêm lượt gọi và giữ slot job.
func (e *ProviderRejectedError) Retryable() bool { return false }

// rejectIfFatal bọc lỗi plan_mismatch thành ProviderRejectedError; lỗi khác giữ nguyên.
func rejectIfFatal(provider, model string, err error) error {
	if err == nil {
		return nil
	}
	var already *ProviderRejectedError
	if errors.As(err, &already) {
		return err
	}
	kind, code := ClassifyProviderError(err)
	if kind != ProviderErrPlanMismatch {
		return err
	}
	return &ProviderRejectedError{Kind: kind, Code: code, Provider: provider, Model: model, Err: err}
}

// ParseProviderRejected nhận ra ProviderRejectedError đã bị ép thành chuỗi.
// code chỉ khác rỗng khi nằm trong allowlist.
func ParseProviderRejected(msg string) (code string, ok bool) {
	i := strings.Index(msg, providerRejectedPrefix)
	if i < 0 {
		return "", false
	}
	if m := rejectedCodePattern.FindStringSubmatch(msg[i:]); m != nil && providerCodeAllowlist[m[1]] {
		code = m[1]
	}
	return code, true
}

// ProviderRejection trả mã gốc khi err là lỗi provider từ chối hẳn, đọc từ chuỗi
// lỗi còn nguyên hoặc từ text khi chuỗi đã bị ép thành chuỗi.
func ProviderRejection(err error, text string) (code string, ok bool) {
	var rejected *ProviderRejectedError
	if errors.As(err, &rejected) {
		return rejected.Code, true
	}
	if err != nil {
		if code, ok := ParseProviderRejected(err.Error()); ok {
			return code, true
		}
	}
	return ParseProviderRejected(text)
}
