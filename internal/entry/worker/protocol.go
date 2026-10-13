package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/voocel/ainovel-cli/internal/server/api"
)

// maxLineBytes giới hạn một dòng JSON trên stdin (prompt + key + pricing dư sức).
const maxLineBytes = 1 << 20

// Giới hạn độ dài input phía worker; server còn chặn chặt hơn (4000/2000 ký tự).
const (
	maxPromptRunes = 16000
	maxTextRunes   = 8000
)

var (
	reasoningEfforts = map[string]bool{"": true, "off": true, "low": true, "medium": true, "high": true, "xhigh": true, "max": true}
	styleNameRe      = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
)

// strictUnmarshal giải mã một dòng JSON, từ chối field lạ và dữ liệu thừa phía sau.
func strictUnmarshal(line []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("dữ liệu thừa sau message JSON")
	}
	return nil
}

// peekType đọc field type mà không áp đặt schema, để chọn struct giải mã chặt.
func peekType(line []byte) (string, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(line, &head); err != nil {
		return "", fmt.Errorf("JSON không hợp lệ: %w", err)
	}
	if head.Type == "" {
		return "", fmt.Errorf("thiếu field type")
	}
	return head.Type, nil
}

// decodeInit giải mã và kiểm tra message init. Lỗi trả về không bao giờ chứa
// giá trị field (tránh lộ API key vào log hay stdout).
func decodeInit(line []byte) (api.WorkerInit, error) {
	var msg api.WorkerInit
	typ, err := peekType(line)
	if err != nil {
		return msg, err
	}
	if typ != api.MsgInit {
		return msg, fmt.Errorf("message đầu tiên phải là init, nhận %q", typ)
	}
	if err := strictUnmarshal(line, &msg); err != nil {
		return api.WorkerInit{}, fmt.Errorf("init không hợp lệ: %s", sanitizeDecodeError(err))
	}
	if err := validateInit(msg); err != nil {
		return api.WorkerInit{}, err
	}
	return msg, nil
}

// sanitizeDecodeError giữ lại tên field/kiểu lỗi, bỏ phần có thể trích giá trị.
func sanitizeDecodeError(err error) string {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return fmt.Sprintf("field %q sai kiểu", typeErr.Field)
	}
	msg := err.Error()
	if strings.HasPrefix(msg, "json: unknown field ") {
		return "field lạ " + strings.TrimPrefix(msg, "json: unknown field ")
	}
	return "JSON không hợp lệ"
}

func validateInit(msg api.WorkerInit) error {
	if msg.Protocol != api.WorkerProtocol {
		return fmt.Errorf("protocol %d không được hỗ trợ (cần %d)", msg.Protocol, api.WorkerProtocol)
	}
	switch msg.Action {
	case api.ActionStart:
		if strings.TrimSpace(msg.Prompt) == "" {
			return fmt.Errorf("action start cần prompt")
		}
	case api.ActionSteer:
		if strings.TrimSpace(msg.Text) == "" {
			return fmt.Errorf("action steer cần text")
		}
	case api.ActionResume, api.ActionAdvance:
	default:
		return fmt.Errorf("action %q không hợp lệ", msg.Action)
	}
	if len([]rune(msg.Prompt)) > maxPromptRunes {
		return fmt.Errorf("prompt quá dài")
	}
	if len([]rune(msg.Text)) > maxTextRunes {
		return fmt.Errorf("text quá dài")
	}
	if msg.BookDir == "" {
		return fmt.Errorf("thiếu book_dir")
	}
	if msg.AdvanceMode != api.AdvanceAuto && msg.AdvanceMode != api.AdvanceReview {
		return fmt.Errorf("advance_mode %q không hợp lệ", msg.AdvanceMode)
	}
	if _, ok := LookupProvider(msg.LLM.Provider); !ok {
		return fmt.Errorf("provider %q không được phép", msg.LLM.Provider)
	}
	if strings.TrimSpace(msg.LLM.Model) == "" {
		return fmt.Errorf("thiếu llm.model")
	}
	if strings.TrimSpace(msg.LLM.APIKey) == "" {
		return fmt.Errorf("thiếu llm.api_key")
	}
	if !reasoningEfforts[msg.LLM.ReasoningEffort] {
		return fmt.Errorf("reasoning_effort %q không hợp lệ", msg.LLM.ReasoningEffort)
	}
	if msg.Language != "" && msg.Language != "vi" && msg.Language != "zh" {
		return fmt.Errorf("language %q không hợp lệ", msg.Language)
	}
	if msg.Style != "" && !styleNameRe.MatchString(msg.Style) {
		return fmt.Errorf("style không hợp lệ")
	}
	if !(msg.BudgetUSD > 0) {
		return fmt.Errorf("budget_usd phải > 0")
	}
	if msg.Pricing == nil {
		return fmt.Errorf("thiếu pricing")
	}
	if msg.Pricing.ID == "" || !(msg.Pricing.InputCostPer1M > 0) || !(msg.Pricing.OutputCostPer1M > 0) {
		return fmt.Errorf("pricing thiếu giá cho model")
	}
	return nil
}

// decodeCommand giải mã một lệnh runtime.
func decodeCommand(line []byte) (api.WorkerCommand, error) {
	var cmd api.WorkerCommand
	typ, err := peekType(line)
	if err != nil {
		return cmd, err
	}
	switch typ {
	case api.MsgSteer, api.MsgSetAdvanceMode, api.MsgAbort, api.MsgShutdown:
	default:
		return cmd, fmt.Errorf("type %q không được hỗ trợ", typ)
	}
	if err := strictUnmarshal(line, &cmd); err != nil {
		return api.WorkerCommand{}, fmt.Errorf("lệnh %s không hợp lệ: %s", typ, sanitizeDecodeError(err))
	}
	if cmd.ID == "" {
		return api.WorkerCommand{}, fmt.Errorf("lệnh %s thiếu id", typ)
	}
	switch typ {
	case api.MsgSteer:
		if strings.TrimSpace(cmd.Text) == "" {
			return api.WorkerCommand{}, fmt.Errorf("lệnh steer cần text")
		}
		if len([]rune(cmd.Text)) > maxTextRunes {
			return api.WorkerCommand{}, fmt.Errorf("text quá dài")
		}
	case api.MsgSetAdvanceMode:
		if cmd.Mode != api.AdvanceAuto && cmd.Mode != api.AdvanceReview {
			return api.WorkerCommand{}, fmt.Errorf("mode %q không hợp lệ", cmd.Mode)
		}
	}
	return cmd, nil
}
