/** Mẫu ý tưởng theo thể loại cho chip "Gợi ý" ở trang tạo truyện. */
export interface PromptPreset {
  label: string
  text: string
}

export const PROMPT_PRESETS: readonly PromptPreset[] = [
  {
    label: 'Tu tiên quật khởi',
    text: 'Một thiếu niên bị phế linh căn ở tông môn hạng bét, tình cờ nhặt được cuốn sổ ghi chép của một vị tiên nhân đã chết, từ đó âm thầm leo lên đỉnh tu tiên giới.',
  },
  {
    label: 'Ngôn tình học đường',
    text: 'Cô lớp trưởng nghiêm khắc và cậu học sinh chuyển trường hay đi muộn bị xếp ngồi chung bàn suốt năm cuối cấp ở một trường cấp ba Hải Phòng.',
  },
  {
    label: 'Ngôn tình cổ đại',
    text: 'Con gái một thầy thuốc trong kinh thành bị ép thế thân tiểu thư nhà quan để gả vào phủ tướng quân, nơi vị phu quân lạnh lùng đang âm thầm điều tra cái chết của mẹ mình.',
  },
  {
    label: 'Đô thị dị năng',
    text: 'Sau một đêm mất điện toàn Sài Gòn, một shipper phát hiện mình nghe được suy nghĩ của bất kỳ ai đang cầm điện thoại.',
  },
  {
    label: 'Trinh thám phố cổ',
    text: 'Một thợ sửa đồng hồ ở phố Hàng Bạc phát hiện mọi chiếc đồng hồ được mang đến tiệm trong tuần đều dừng đúng 3 giờ 17 phút.',
  },
  {
    label: 'Kinh dị làng quê',
    text: 'Một nhóm sinh viên về một ngôi làng ven sông để quay phim tài liệu về lễ cúng cô hồn, rồi nhận ra mỗi đêm dân làng lại đếm thiếu một người mà không ai nhớ người đó là ai.',
  },
  {
    label: 'Khoa huyễn thuộc địa',
    text: 'Năm 2187, kỹ sư trẻ trên trạm khai mỏ ở vành đai tiểu hành tinh nhận được tín hiệu cầu cứu từ chính con tàu mà cô sẽ lên đường ba ngày nữa.',
  },
  {
    label: 'Võ hiệp giang hồ',
    text: 'Đứa con duy nhất còn sống của một tiêu cục bị diệt môn được một gánh hát rong nuôi lớn, mười năm sau trở lại giang hồ trong vai một đào hát để tìm kẻ thù.',
  },
]
