---
name: chao-ngay-moi
description: Tạo lỗi chào ngày mới hài hước, dí dỏm bằng tiếng Việt. Lấy thông tin ngày hiện tại từ internet (ngày tháng, thứ, sự kiện đặc biệt) và tạo nội dung chào buổi sáng phong cách vui vẻ. Sử dụng khi user muốn "chào ngày mới", "good morning", "chào buổi sáng", hoặc cần lỗi chào bắt đầu ngày mới.
---

# Skill Chào Ngày Mới

Tạo lỗi chào ngày mới phong cách hài hước, dí dỏm bằng tiếng Việt.

## Cách sử dụng

1. **Tìm kiếm thông tin ngày hiện tại từ internet**:
   - Dùng `web_search` để tìm: "hôm nay ngày mấy" hoặc "today date" hoặc "sự kiện hôm nay"
   - Có thể tìm thêm: "ngày đặc biệt hôm nay", "holiday today", sự kiện văn hóa, lễ hội...

2. **Thông tin cần thu thập**:
   - Ngày tháng năm hiện tại
   - Thứ trong tuần
   - Sự kiện đặc biệt hôm nay (nếu có) - lễ, ngày kỷ niệm, sự kiện văn hóa...
   - Thông tin thú vị về ngày này

3. **Dựa trên thông tin đó, tạo lỗi chào hài hước**:
   - Chào + đùa 1 câu
   - Nhắc thứ/ngày 1 câu
   - Chúc 1 câu

## Phong cách

- **CỰC KỲ NGẮN** - Chỉ 2-3 câu, đi thẳng vào vấn đề
- Hài hước, dí dỏm
- Không lan man, không dài dòng
- Không hardcode sự kiện - luôn tìm kiếm real-time từ internet

## Format đầu ra (NGẮN GỌN)

```
☀️ [Chào 1 câu + đùa 1 câu]
📅 [Thứ ngày]
🧧 [Chúc 1 câu]
```

## Ví dụ đầu ra

```
☀️ Chào buổi sáng! Thứ Sáu rồi, mai được nghỉ rồi nhá! 🎉
📅 Thứ Sáu, 20/2
🧧 Cuối tuần vui vẻ, deadline tự trôi! 💪
```

## Lưu ý

- KHÔNG dùng script hay hardcode
- LUÔN tìm kiếm thông tin thực tế từ internet
- Sự kiện phải được tìm thấy online, không tự bịa
