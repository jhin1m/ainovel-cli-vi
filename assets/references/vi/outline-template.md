# Mẫu quy hoạch dàn ý

Mẫu này không nhằm ép mọi tác phẩm về một độ dài cố định, mà giúp xác định cấp độ tác phẩm trước, rồi mới chọn độ chi tiết của dàn ý.

## Bước một: xác định cấp độ độ dài tác phẩm

### Truyện ngắn / câu chuyện một quyển

- Phù hợp: một xung đột, một mục tiêu, ít nhân vật, kết cục tập trung
- Độ dài tham khảo: 8-25 chương
- Định dạng đề xuất: `outline` phẳng

### Truyện vừa / câu chuyện nhiều giai đoạn

- Phù hợp: có nâng cấp theo giai đoạn, vài tuyến phụ, quan hệ nhân vật có thay đổi
- Độ dài tham khảo: 25-60 chương
- Định dạng đề xuất: `outline` phẳng hoặc phân tầng nhẹ

### Truyện dài đăng nhiều kỳ / kiểu truyện mạng

- Phù hợp: đề tài vốn có dư địa nâng cấp liên tục, sức căng quan hệ dài hạn, nhiều mục tiêu giai đoạn, thế giới mở rộng được, bí ẩn dài hạn hoặc tuyến trưởng thành dài hạn
- Độ dài tham khảo: 80-200+ chương
- Định dạng đề xuất: `layered_outline` phân tầng

## Bước hai: xác định có bắt buộc dùng dàn ý phân tầng không

Chỉ cần thoả bất kỳ 2 điều dưới đây là ưu tiên dùng `layered_outline`:

- Thế giới quan cần mở ra dần, không kể hết một lần
- Nhân vật chính trưởng thành không phải một bước nhảy, mà là nâng cấp nhiều giai đoạn
- Quan hệ nhân vật liên tục thay đổi qua nhiều giai đoạn
- Giữa truyện và cuối truyện có các loại mâu thuẫn chính khác nhau
- Cần nhiều lần đổi bản đồ/thế lực/thân phận/mục tiêu
- Đề tài rõ ràng giống tiểu thuyết thương mại đăng nhiều kỳ hơn là câu chuyện một quyển

## Bước ba: với truyện dài, đừng làm ngay "sổ liệt kê chương của cả bộ"

Thứ tự quy hoạch truyện dài đề xuất:

1. Điểm hút và sự khác biệt của tác phẩm
2. Động cơ câu chuyện dài hạn
3. Chủ đề và nâng cấp cấp quyển
4. Mục tiêu và bước ngoặt giai đoạn cấp cung
5. Sự kiện và móc câu cấp chương

Cách làm sai:

- Viết tóm tắt 20 chương trước, rồi gượng kéo dài
- Quyển nào cũng lặp "gặp địch - mạnh lên - đổi bản đồ"
- Chỉ nâng cấp tuyến chính, không nâng cấp quan hệ
- Phần đầu tiêu hết các bí mật lớn, giữa và cuối truyện chỉ còn lặp công thức

## Mẫu dàn ý phẳng (truyện ngắn/vừa)

```json
[
  {
    "chapter": 1,
    "title": "Tên chương",
    "core_event": "Sự kiện cốt lõi của chương",
    "hook": "Móc câu cuối chương",
    "scenes": ["Cảnh 1", "Cảnh 2", "Cảnh 3"]
  }
]
```

## Mẫu dàn ý phân tầng (truyện dài - triển khai cuốn chiếu hai tầng quyển và cung)

Quy hoạch ban đầu dùng cuốn chiếu hai tầng: 2 quyển đầu có khung cung, các quyển còn lại là quyển khung; cung đầu tiên có chương chi tiết.

```json
[
  {
    "index": 1,
    "title": "Tên quyển một",
    "theme": "Mâu thuẫn/chủ đề cốt lõi mới của quyển này",
    "arcs": [
      {
        "index": 1,
        "title": "Cung thứ nhất (đã triển khai)",
        "goal": "Mục tiêu cục bộ, lực cản và bước ngoặt",
        "chapters": [
          {"chapter": 1, "title": "Tên chương", "core_event": "Sự kiện cốt lõi", "hook": "Móc câu cuối chương", "scenes": ["Cảnh 1", "Cảnh 2"]}
        ]
      },
      {
        "index": 2,
        "title": "Cung thứ hai (cung khung)",
        "goal": "Tóm tắt mục tiêu của cung này",
        "estimated_chapters": 12,
        "chapters": []
      }
    ]
  },
  {
    "index": 2,
    "title": "Tên quyển hai",
    "theme": "Chủ đề quyển hai",
    "arcs": [
      {"index": 1, "title": "Tên cung", "goal": "Mục tiêu cung", "estimated_chapters": 15, "chapters": []},
      {"index": 2, "title": "Tên cung", "goal": "Mục tiêu cung", "estimated_chapters": 10, "chapters": []}
    ]
  },
  {
    "index": 3,
    "title": "Tên quyển ba (quyển khung)",
    "theme": "Hướng chủ đề quyển ba",
    "estimated_chapters": 60,
    "arcs": []
  }
]
```

- Triển khai cấp cung: khi việc viết tiến tới cung khung, kiến trúc sư triển khai các chương chi tiết của cung đó
- Triển khai cấp quyển: khi việc viết tiến tới quyển khung, kiến trúc sư triển khai cấu trúc cung của quyển đó + các chương của cung đầu

## Danh sách kiểm tra cấp quyển cho truyện dài

Quyển nào cũng phải trả lời được:

- Quyển này bổ sung thông tin thế giới gì?
- Quyển này nâng cấp mâu thuẫn cốt lõi nào?
- Quyển này cho nhân vật chính được gì, và mất gì?
- Quyển này thay đổi các quan hệ nhân vật chính ra sao?
- Kết thúc quyển này, vì sao câu chuyện buộc phải sang quyển sau?

## Danh sách kiểm tra cấp cung cho truyện dài

Cung nào cũng phải trả lời được:

- Mục tiêu rõ ràng của cung này là gì?
- Lực cản đến từ ai, quy tắc nào, cái giá nào?
- Bước ngoặt là gì?
- Kết thúc cung này, những trạng thái nào đã thay đổi không thể đảo ngược?

## Danh sách kiểm tra cấp chương

- Mỗi chương phải phục vụ mục tiêu của cung chứa nó
- Mỗi chương phải có một bước đẩy sự kiện không thể xoá bỏ
- Móc câu phải đa dạng, đừng chỉ dựa vào một kiểu "phát hiện bí mật"
- Các chương đầu không được chỉ để "giới thiệu thế giới", phải đồng thời đẩy nhân vật và xung đột
