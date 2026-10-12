# ainovel-cli (bản tiếng Việt)

Công cụ viết tiểu thuyết dài hoàn toàn tự động bằng AI, Việt hoá từ
[voocel/ainovel-cli](https://github.com/voocel/ainovel-cli). Một engine xác định điều phối ba
agent sáng tác (Architect lập dàn ý, Writer viết chương, Editor duyệt) và gọi Arbiter khi cần
phán quyết. Từ một câu yêu cầu đến cả bộ truyện, không cần người can thiệp.

Bản này khác upstream ở các điểm:

- `language` trong config: `"vi"` (mặc định) viết truyện tiếng Việt; `"zh"` dùng prompt tiếng Trung gốc.
- Prompt, tài liệu tham khảo, phong cách tiếng Việt ở `assets/*/vi/`.
- Mô tả tool, chỉ dẫn ngữ cảnh, thông báo lỗi gửi cho model, giao diện TUI đều bằng tiếng Việt.
- Độ dài chương tính theo "chữ" (từ/âm tiết tách bởi khoảng trắng) khi `language=vi`.
- Luật kiểm tra văn bản tiếng Việt: chữ Hán lẫn vào truyện, từ tiếng Anh lọt vào lời kể, đoạn lặp nguyên văn.

Tài liệu chi tiết về kiến trúc vẫn ở [README.md](README.md) (tiếng Trung). Cách đồng bộ upstream: [SYNC.md](SYNC.md).

## Cài đặt

Cần Go 1.25 trở lên.

```bash
git clone git@github.com:jhin1m/ainovel-cli-vi.git
cd ainovel-cli-vi
go build -o ainovel-cli ./cmd/ainovel-cli
./ainovel-cli --version
```

Có thể chép `ainovel-cli` vào thư mục trong `PATH` (vd. `~/.local/bin`).
Lưu ý: `ainovel-cli update` tải bản phát hành của upstream (tiếng Trung), không dùng lệnh này cho bản Việt.

## Cấu hình

Config toàn cục ở `~/.ainovel/config.json`, có thể ghi đè theo dự án bằng `./.ainovel/config.json`
(cùng cấu trúc, trường nào khai báo thì đè trường đó). Lần chạy TUI đầu tiên sẽ hỏi provider, API key,
base URL, model; chế độ headless thì không hỏi, cần tạo file trước.

Ví dụ dùng GLM qua z.ai (gói coding plan, endpoint tương thích OpenAI). Key dưới đây là key giả:

```json
{
  "provider": "glm",
  "model": "glm-5.3",
  "reasoning_effort": "medium",
  "language": "vi",
  "style": "fantasy",
  "providers": {
    "glm": {
      "api_key": "sk-glm-xxxxxxxxxxxxxxxxxxxxxxxx",
      "base_url": "https://api.z.ai/api/coding/paas/v4",
      "models": [
        { "name": "glm-5.3" },
        { "name": "glm-5.3-flash" }
      ]
    }
  }
}
```

- `language`: `"vi"` hoặc `"zh"`. Bỏ trống = `"vi"`.
- `style`: `default` / `fantasy` (huyền huyễn, tiên hiệp) / `romance` / `suspense`.
- `roles.<architect|writer|editor|arbiter>` cho phép mỗi vai một model riêng, xem `config.example.jsonc`.
- File config chứa API key: không commit. `.ainovel/` đã nằm trong `.gitignore`.

## Chạy

Mỗi bộ truyện gắn với thư mục khởi động; dữ liệu nằm ở `./output/novel/`. Chạy lại trong cùng thư mục
sẽ tiếp tục từ checkpoint gần nhất. Đổi thư mục = bộ truyện mới.

### Giao diện TUI

```bash
mkdir -p ~/novels/tien-hiep-1 && cd ~/novels/tien-hiep-1
ainovel-cli
```

Gõ yêu cầu vào ô nhập để bắt đầu. Trong lúc viết có thể gõ ý kiến chỉnh sửa bất cứ lúc nào
(vd. "đẩy tuyến tình cảm lên chương 4"), hệ thống tự đánh giá phạm vi ảnh hưởng.
Lệnh hay dùng: `/review on` (duyệt từng chương), `/next` (cho viết chương kế), `/review off`, `/model`, `/config`.

### Headless (máy chủ, treo máy)

```bash
mkdir -p ~/novels/tien-hiep-1 && cd ~/novels/tien-hiep-1

# Yêu cầu một câu
ainovel-cli --headless --prompt "Viết truyện tiên hiệp dài khoảng 100 chương, nhân vật chính xuất thân tạp dịch ngoại môn"

# Yêu cầu từ file
ainovel-cli --headless --prompt-file yeu-cau.txt

# Tiếp tục bộ truyện dang dở trong thư mục này
ainovel-cli --headless
```

Đầu ra của model ghi ra stdout, sự kiện ghi ra stderr, log đầy đủ ở `output/novel/logs/headless.log`.
Treo máy lâu nên chạy trong `tmux`/`screen` hoặc `nohup`:

```bash
nohup ainovel-cli --headless --prompt-file yeu-cau.txt > run.out 2> run.err &
```

Mẫu `yeu-cau.txt` cho bộ dài:

```text
Viết một bộ tiểu thuyết tiên hiệp tiếng Việt khoảng 100 chương, mỗi chương khoảng 3000 chữ.
Nhân vật chính: Lâm Phong, đệ tử tạp dịch của Thanh Vân Tông, linh căn bị cho là phế.
Giọng văn: kể ngôi thứ ba, nhịp nhanh, thoại tự nhiên, tên riêng dùng âm Hán-Việt.
Tuyến chính: từ ngoại môn lên nội môn, vạch trần âm mưu của trưởng lão, cuối cùng phi thăng.
```

### Kết quả

```
output/novel/
├── book.md          # tên truyện, giới thiệu
├── chapters/        # các chương hoàn chỉnh (01.md, 02.md, ...)
├── outline.json     # dàn ý
├── characters.json  # hồ sơ nhân vật
├── reviews/         # nhận xét của Editor
└── meta/            # tiến độ, phục bút, checkpoint
```

## Kiểm tra bản Việt hoá

```bash
go build ./... && go test ./...
go run ./scripts/check-han       # 0 = không còn chữ Hán trong chuỗi gửi cho model
```

## Giấy phép

Apache License 2.0, giống upstream. Credit xem [NOTICE](NOTICE).
