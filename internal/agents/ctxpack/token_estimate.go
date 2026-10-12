package ctxpack

import (
	"github.com/voocel/agentcore"
	corecontext "github.com/voocel/agentcore/context"

	"github.com/voocel/ainovel-cli/internal/lang"
)

// viBytesPerToken là số byte UTF-8 trung bình ứng với một token cho văn tiếng Việt.
// Chữ Latin có dấu tách token kém hơn tiếng Anh (dấu thanh chiếm 2–3 byte và hay bị
// cắt rời), nên byte/3 sát thực tế hơn mức byte/4 mà agentcore dùng cho văn ASCII.
const viBytesPerToken = 3

// estimateTextTokens ước lượng token của một đoạn văn bản theo ngôn ngữ sáng tác.
// zh giữ nguyên ước lượng của agentcore (rune × 1.5 cho văn CJK); vi lấy giá trị
// lớn hơn giữa ước lượng agentcore và byte/3 để không đánh giá thấp ngân sách.
func estimateTextTokens(text string) int {
	base := corecontext.EstimateTokens(agentcore.UserMsg(text))
	if !lang.IsVI() {
		return base
	}
	return max(base, (len(text)+viBytesPerToken-1)/viBytesPerToken)
}

// bytesPerToken là số byte quy đổi cho một token khi cắt JSON theo ngân sách.
func bytesPerToken() int {
	if lang.IsVI() {
		return viBytesPerToken
	}
	return 4
}
