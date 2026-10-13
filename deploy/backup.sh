#!/usr/bin/env bash
# Sao lưu AI Novel: dump Postgres + nén thư mục dữ liệu truyện, xoay vòng bản cũ.
#
# Chạy bằng root hoặc user có quyền đọc DB URL và thư mục data (vd. qua cron/systemd timer).
# Master key KHÔNG nằm trong bản sao lưu này: lưu riêng (password manager).
#
# Biến môi trường (đều tuỳ chọn):
#   AINOVEL_DATABASE_URL_FILE  file chứa chuỗi kết nối   (mặc định /etc/ainovel/database-url)
#   AINOVEL_DATA_DIR           thư mục dữ liệu            (mặc định /var/lib/ainovel)
#   BACKUP_DIR                 nơi lưu bản sao lưu        (mặc định /var/backups/ainovel)
#   BACKUP_KEEP                số bản giữ lại             (mặc định 14)
set -euo pipefail
umask 077

DB_URL_FILE="${AINOVEL_DATABASE_URL_FILE:-/etc/ainovel/database-url}"
DATA_DIR="${AINOVEL_DATA_DIR:-/var/lib/ainovel}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/ainovel}"
KEEP="${BACKUP_KEEP:-14}"

case "$KEEP" in
  '' | *[!0-9]* | 0) echo "BACKUP_KEEP phải là số nguyên > 0" >&2; exit 2 ;;
esac
[ -r "$DB_URL_FILE" ] || { echo "không đọc được $DB_URL_FILE" >&2; exit 1; }
[ -d "$DATA_DIR/users" ] || { echo "không thấy $DATA_DIR/users" >&2; exit 1; }
command -v pg_dump >/dev/null || { echo "thiếu pg_dump" >&2; exit 1; }

mkdir -p "$BACKUP_DIR"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
tmp="$(mktemp -d "$BACKUP_DIR/.tmp.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT

# Chuỗi kết nối chỉ nằm trong biến cục bộ, không bao giờ được in ra. pg_dump nhận nó qua
# --dbname nên tham số này thấy được trong `ps` với user khác: chạy script bằng root
# hoặc để /proc gắn hidepid=2 trên máy nhiều user.
db_url="$(tr -d '\r\n' < "$DB_URL_FILE")"
pg_dump -Fc --no-owner --dbname "$db_url" -f "$tmp/db-$stamp.dump"
unset db_url

# Thư mục data đang được worker ghi: tar có thể báo "file changed" (mã 1), bản sao
# bản sao vẫn dùng được nhưng có thể lệch vài tệp đang ghi (truyện đang chạy). Chỉ lỗi nặng mới thoát.
rc=0
tar -C "$DATA_DIR" -czf "$tmp/users-$stamp.tar.gz" users || rc=$?
if [ "$rc" -gt 1 ]; then
  echo "tar thất bại (mã $rc)" >&2
  exit 1
fi

mv "$tmp/db-$stamp.dump" "$tmp/users-$stamp.tar.gz" "$BACKUP_DIR/"

# Xoay vòng: giữ KEEP bản mới nhất của mỗi loại.
for prefix in db users; do
  # shellcheck disable=SC2012
  ls -1t "$BACKUP_DIR"/"$prefix"-*.* 2>/dev/null | tail -n +"$((KEEP + 1))" | while IFS= read -r old; do
    rm -f -- "$old"
  done
done

echo "đã sao lưu vào $BACKUP_DIR (stamp $stamp)"
