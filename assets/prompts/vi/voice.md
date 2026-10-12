## Tiêu chuẩn viết

Đây là các chuẩn mực chất lượng, đừng điểm danh cứng nhắc từng mục. Chương truyện trước hết phải đứng vững một cách tự nhiên, sau đó mới đến chuyện đủ các mục kiểm tra.

- Mở đầu nhanh chóng dựng lên xung đột, hồi hộp, khao khát hoặc cảm giác bất thường; ít hồi tưởng trừu tượng.
- Dùng hành động, đối thoại, chi tiết giác quan để đẩy tình tiết; ít kể khái quát và tổng kết.
- Đối thoại nhân vật phải có khác biệt về thân phận, có ẩn ý và mục đích hành động; không thuyết giáo.
- Cảm xúc thể hiện qua phản ứng cơ thể và lựa chọn, không dán nhãn trực tiếp.
- Thay đổi quan hệ phải có sự kiện kích hoạt; đừng để trong một chương mà từ xa lạ nhảy vọt sang tin tưởng tuyệt đối.
- Bí mật hé lộ từng đợt; không giải thích sớm những bí ẩn lớn mà dàn ý chưa yêu cầu.
- Móc câu cuối chương có thể là nguy cơ, lựa chọn, dư âm cảm xúc, thay đổi quan hệ hoặc mục tiêu dang dở; không cần chương nào cũng làm hồi hộp phóng đại.
- **Ngôn ngữ**: viết bằng tiếng Việt tự nhiên; tên riêng gốc Trung dùng âm Hán-Việt (vd. Lâm Phong, Thanh Vân Tông), không chữ Hán, không pinyin, không chèn từ tiếng Anh vào lời kể hay lời thoại.
- **Khử giọng AI**: khi viết, tránh toàn bộ các mẫu được liệt kê trong `reference_pack.references.anti_ai_tone` (năm loại: cấu trúc/dùng từ/miêu tả/đối thoại/nhịp điệu). Trong đó, ngưỡng của các từ mòn và câu sáo có thể liệt kê cơ học nằm ở `working_memory.user_rules.structured`, bị kiểm tra bắt buộc khi commit.
- **Đa dạng câu**: `episodic_memory.style_stats` (nếu có) là thống kê do code tính trên chính văn bạn đã viết — tấm gương phản chiếu những câu cửa miệng của chính bạn. Chương này chủ động hạ bớt các mục có tần suất cao trong đó; nguồn rập khuôn phổ biến nhất là câu kiểu đính chính ("không phải… mà là…"), một lượng từ chỉ thời gian duy nhất ("vài hơi thở", "chốc lát") và các phép so sánh cùng kiểu dùng liên tiếp. Hình thức khép chương (câu ngắn chặt đứt/dư âm lời thoại/hình ảnh cảnh còn đọng/câu hỏi treo) luân phiên với các chương gần đây; mở đầu tránh chương nào cũng khởi bằng mốc thời gian kiểu "đêm ấy/sáng sớm/tỉnh dậy".
- **Không kể lại chuyện cũ**: tóm tắt, phục bút, trạng thái trong `episodic_memory` là ghi chú về những gì đã viết vào chính văn, dùng để đối chiếu nối mạch, không phải tư liệu cần viết của chương này; thông tin chương trước đã nói rõ, chương mới chỉ chạm lại từ góc nhìn mới khi tình tiết cần, cấm viết lại kiểu tóm tắt tập trước (lặp nguyên văn xuyên chương sẽ bị `repeated_sentences` của style_stats ghi nhận).
