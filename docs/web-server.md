# Bản web (`ainovel-cli serve`)

`ainovel-cli serve` chạy engine viết truyện như một dịch vụ web nhiều người dùng: đăng nhập, mỗi người dán API key LLM
của mình (BYOK), gõ một câu ý tưởng, xem tiến độ realtime, đọc chương, can thiệp, duyệt từng chương và xuất TXT/EPUB.
Giao diện là một SPA React nhúng sẵn trong binary, cài được như PWA. Binary lúc chạy không cần Node.

## Kiến trúc ngắn

```
Trình duyệt (SPA/PWA)
   │  REST JSON + SSE, cookie session
   ▼
Cloudflare ─► Caddy (443) ─► ainovel-cli serve (127.0.0.1:8080) ── Postgres
                                  │  spawn `ainovel-cli worker` (stdin/stdout JSON Lines)
                                  ├── worker truyện A   cwd = /var/lib/ainovel/users/<uid>/books/<id>
                                  └── worker truyện B   HOME = /var/lib/ainovel/users/<uid>/home
```

- **Server** lo xác thực, lưu API key đã mã hoá (AES bằng master key), hàng đợi, giới hạn tài nguyên, SSE và đọc/xuất truyện.
- **Worker** là một process con cho mỗi lượt chạy của một truyện (chạy engine nguyên bản). Worker thoát khi engine dừng:
  chờ duyệt, hết ngân sách, người dùng dừng, xong truyện hoặc lỗi. "Tiếp tục", "Chương tiếp" hay steer khi đang dừng sẽ
  spawn worker mới. API key chỉ đi qua stdin của worker, không nằm trong env hay log.
- **Postgres** giữ user, session, API key mã hoá, danh sách truyện và trạng thái job. Nội dung truyện nằm ở
  `AINOVEL_DATA_DIR` dưới dạng tệp.
- **SSE** (`GET /api/books/{id}/events`) đẩy event realtime, ping mỗi 20 giây để Cloudflare không cắt kết nối im lặng.
  Khi mất kết nối trình duyệt tự nối lại và dựng lại timeline từ snapshot.

## Yêu cầu

- Linux có systemd >= 250 (Debian 12, Ubuntu 24.04 trở lên).
- Postgres 13 trở lên (bản `apt` mặc định của Debian 12 và Ubuntu 24.04 dùng được; CI chạy trên 17), chỉ bind localhost, với extension `citext` (có sẵn trong gói `postgresql-contrib`; từ PG13 chủ DB tự tạo được).
- Caddy 2.7+ và một tên miền trên Cloudflare.
- Để build từ mã nguồn: Go 1.25+, Node 24, pnpm 12 (xem bên dưới). Có thể build ở máy khác rồi chép binary lên VPS.

## Build

```bash
git clone https://github.com/jhin1m/ainovel-cli-vi.git && cd ainovel-cli-vi
make build            # pnpm -C web install --frozen-lockfile && pnpm -C web build && go build -o ainovel-cli ./cmd/ainovel-cli
```

Kết quả là một file `./ainovel-cli` đã nhúng SPA. `go build` thuần (không `make build`) vẫn chạy được nhưng SPA sẽ là
trang "chưa build". `ainovel-cli serve --help` in cách dùng; cấu hình hoàn toàn qua biến môi trường.

## Cài đặt trên VPS

### 1. Postgres

```bash
sudo apt install postgresql postgresql-contrib
sudo -u postgres psql <<'SQL'
CREATE ROLE ainovel LOGIN PASSWORD 'doi-mat-khau-manh';
CREATE DATABASE ainovel OWNER ainovel;
SQL
```

Giữ `listen_addresses = 'localhost'` (mặc định). Server tự chạy migration khi khởi động.

### 2. User, thư mục, binary

```bash
sudo useradd --system --home-dir /var/lib/ainovel --shell /usr/sbin/nologin ainovel
sudo install -d -o ainovel -g ainovel -m 0700 /var/lib/ainovel
sudo install -d -m 0700 /etc/ainovel
sudo install -m 0755 ainovel-cli /usr/local/bin/ainovel-cli
```

### 3. File secret

Secret chỉ đọc từ file (env của server đọc được qua `/proc` bởi cùng UID). Hai file thuộc root, systemd chuyển vào
service qua `LoadCredential`:

```bash
# Chuỗi kết nối Postgres
printf '%s' 'postgres://ainovel:doi-mat-khau-manh@127.0.0.1:5432/ainovel?sslmode=disable' | sudo tee /etc/ainovel/database-url >/dev/null
sudo chmod 0400 /etc/ainovel/database-url

# Master key 32 byte (base64 hoặc hex)
openssl rand -base64 32 | sudo tee /etc/ainovel/master-key >/dev/null
sudo chmod 0400 /etc/ainovel/master-key
```

**Sao lưu master key riêng** (password manager hoặc kho bí mật khác), không để chung với bản dump DB. Mất master key là
mất toàn bộ API key đã lưu (người dùng phải dán lại); lộ master key cùng bản dump là lộ toàn bộ API key.

### 4. systemd

```bash
sudo install -m 0644 deploy/ainovel-serve.service /etc/systemd/system/ainovel-serve.service
sudoedit /etc/systemd/system/ainovel-serve.service   # đặt AINOVEL_PUBLIC_URL đúng tên miền, chỉnh MemoryMax theo máy
sudo systemctl daemon-reload
sudo systemctl enable --now ainovel-serve
journalctl -u ainovel-serve -f
curl -s http://127.0.0.1:8080/healthz
```

Unit chạy bằng user `ainovel`, `KillMode=mixed`, `TimeoutStopSec=90`, `OOMPolicy=continue` (một worker bị OOM không kéo sập
cả service), giới hạn khởi động lại 5 lần/60 giây khi cấu hình sai, hardening (`NoNewPrivileges`, `ProtectSystem=strict`,
chỉ ghi được `/var/lib/ainovel`, `PrivateTmp`...). Fork/exec không bị chặn vì server cần spawn worker.

### 5. Caddy và Cloudflare

1. Trỏ DNS về VPS (bật proxy cam của Cloudflare). Cloudflare đứng trước nên Caddy không tự xin chứng chỉ Let's Encrypt
   được (challenge không qua proxy, và tường lửa chỉ mở 443); dùng **Cloudflare Origin Certificate**.
2. Tạo Origin Certificate: Cloudflare dashboard > SSL/TLS > Origin Server > Create Certificate (RSA hoặc ECDSA, hostname là
   tên miền của bạn, hạn tuỳ chọn). Lưu certificate vào `cf-origin.pem`, private key vào `cf-origin.key` (key chỉ hiện một lần), rồi:
   ```bash
   sudo install -m 0644 -o root -g caddy cf-origin.pem /etc/caddy/cf-origin.pem
   sudo install -m 0640 -o root -g caddy cf-origin.key /etc/caddy/cf-origin.key
   shred -u cf-origin.key 2>/dev/null || rm -f cf-origin.key   # xoá bản tạm
   ```
   Đặt SSL/TLS mode **Full (strict)**. Origin Certificate chỉ được Cloudflare tin, truy cập thẳng vào VPS sẽ báo chứng chỉ
   không hợp lệ (đúng ý muốn). Hết hạn thì tạo cái mới và `systemctl reload caddy`.
3. Cài Caddy, chép `deploy/Caddyfile` vào `/etc/caddy/Caddyfile`, đổi tên miền (dòng `tls` đã trỏ tới hai file trên),
   `sudo caddy validate --config /etc/caddy/Caddyfile`, rồi `sudo systemctl reload caddy`.
4. Tường lửa: mở 443 chỉ cho dải IP Cloudflare (https://www.cloudflare.com/ips), đóng mọi cổng khác trừ SSH của bạn (8080 và 5432
   không mở ra ngoài). Ví dụ với ufw: `sudo ufw default deny incoming`, rồi với từng dải
   `sudo ufw allow from <dải> to any port 443 proto tcp`. Cập nhật dải IP trong `Caddyfile` và tường lửa khi Cloudflare đổi;
   lấy danh sách bằng `curl https://www.cloudflare.com/ips-v4` và `.../ips-v6`.
5. **Bắt buộc:** thêm rate limiting rule của Cloudflare trước khi mở đăng ký. Đăng ký tự do, nên đây là lớp chặn chính; limiter trong server chỉ là lớp phụ (nằm trong bộ nhớ, mất khi restart). Các path cần che: `/api/auth/` (đăng nhập, đăng ký), `/api/auth/signup` (chặt hơn) và `/api/keys` (tạo, xoá, kiểm key). Số rule và chu kỳ đếm tuỳ gói:
   - **Free** (1 rule, chu kỳ 10 giây, lọc theo path): một rule gộp, khớp khi URI path bắt đầu bằng `/api/auth/` hoặc `/api/keys`, đếm theo IP.
   - **Pro trở lên** (từ 2 rule): rule gộp như trên, cộng một rule riêng chặt hơn cho `/api/auth/signup`.

   Chọn ngưỡng theo lưu lượng thật, quy về chu kỳ của gói (vd. trên Free, vài request mỗi 10 giây cho mỗi IP). Xem
   https://developers.cloudflare.com/waf/rate-limiting-rules/ để biết giới hạn hiện tại của từng gói.
6. Kiểm tra SSE qua Cloudflare: mở một truyện đang chạy và để tab quá 5 phút, không được gặp lỗi 524.

Xuất file bị server cắt sau 80 giây, Caddy chờ header tối đa 90 giây, còn Cloudflare cắt ở 100 giây; ba mốc này cần giữ đúng thứ tự.

Caddy đặt `X-Forwarded-For` bằng IP client thật (lấy từ `CF-Connecting-IP` chỉ khi kết nối đến từ dải Cloudflare) và
tắt đệm cho SSE (`flush_interval -1`). Server đặt `AINOVEL_TRUSTED_PROXIES=127.0.0.1/32` nên chỉ tin header đó khi
đến từ Caddy; nếu quên biến này, mọi client dùng chung một IP cho rate limit và server ghi cảnh báo.

## Biến môi trường

| Biến | Mặc định | Ý nghĩa |
|---|---|---|
| `AINOVEL_LISTEN` | `127.0.0.1:8080` | Địa chỉ lắng nghe. Đừng mở ra ngoài, để Caddy đứng trước. |
| `AINOVEL_DATA_DIR` | `/var/lib/ainovel` | Nơi lưu truyện, HOME từng user, cache bảng giá. |
| `AINOVEL_PUBLIC_URL` | rỗng | URL công khai dạng `https://ten-mien` (không đường dẫn). Origin tin cậy cho request ghi (chống CSRF); nên đặt ở production, trùng tên miền người dùng truy cập. |
| `AINOVEL_TRUSTED_PROXIES` | rỗng | Danh sách IP/CIDR cách nhau dấu phẩy của proxy được tin để đọc IP client. Với Caddy cùng máy: `127.0.0.1/32`. |
| `AINOVEL_CLIENT_IP_HEADER` | `X-Forwarded-For` | Header chứa IP client: `X-Forwarded-For` hoặc `CF-Connecting-IP`. |
| `AINOVEL_DATABASE_URL_FILE` | bắt buộc | File chứa chuỗi kết nối Postgres. |
| `AINOVEL_MASTER_KEY_FILE` | bắt buộc | File chứa master key 32 byte, base64 hoặc hex. |
| `AINOVEL_MAX_JOBS` | `4` | Số worker chạy đồng thời toàn server. |
| `AINOVEL_MAX_JOBS_PER_USER` | `1` | Số worker đồng thời mỗi user. |
| `AINOVEL_MAX_BOOKS_PER_USER` | `20` | Số truyện tối đa mỗi user. |
| `AINOVEL_MAX_BOOK_BUDGET_USD` | `50` | Trần ngân sách cho một truyện (số > 0). Truyện thuê bao (Z.ai Coding Plan) không có ngân sách USD. |
| `AINOVEL_MAX_RUN_MINUTES` | `60` | Một lượt chạy dài hơn mức này thì nhường slot (chia lượt). `0` = tắt chia lượt. Giá trị 0..10080. |
| `AINOVEL_SLICE_GRACE_MINUTES` | `20` | Thời gian ân hạn cho lượt bị cắt viết nốt chương đang dở. `0` = cắt ngay, bỏ chương đang viết dở. Giá trị 0..10080. |
| `AINOVEL_MAX_SSE_PER_USER` | `4` | Kết nối SSE mỗi user. |
| `AINOVEL_MAX_SSE_TOTAL` | `200` | Kết nối SSE toàn server. |
| `AINOVEL_MAX_EXPORTS` | `2` | Số lượt xuất file chạy đồng thời. |

Chỉ dùng khi phát triển (bị từ chối nếu không có `AINOVEL_DEV=true`): `AINOVEL_DEV` (tắt cookie Secure, cho phép HTTP),
`AINOVEL_DATABASE_URL` (chuỗi kết nối trực tiếp trong env), `AINOVEL_DEV_WORKER_BIN` (chạy worker giả thay cho chính binary).
Các biến số nguyên phải > 0 (riêng hai biến phút ở trên nhận 0). File secret có quyền rộng hơn 0600 chỉ bị cảnh báo; `LoadCredential` đã tự đặt quyền chặt.

## Giới hạn khi mở đăng ký

Đăng ký tự do nên mọi tài nguyên tốn tiền đều có trần. Rule rate limit của Cloudflare (bước 5 mục Caddy và Cloudflare) là lớp chặn
chính; các giới hạn dưới đây là lớp phụ trong server.

| Việc | Giới hạn | Lỗi khi vượt |
|---|---|---|
| Đăng ký | 30/phút theo IP (dùng chung với đăng nhập) và 10/ngày theo IP; IPv6 tính theo dải /64 | 429 `rate_limited` |
| Tạo API key | 30/ngày/user; tối đa 20 key/user | 429 `rate_limited`; 409 `key_limit` |
| Kiểm key (`POST /api/keys/test`) | 3/phút và 20/ngày/user; 60/ngày/IP; tối đa 4 lượt chạy đồng thời toàn server | 429 `rate_limited`; 429 `busy` |
| Worker đồng thời | `AINOVEL_MAX_JOBS` (4) toàn server, `AINOVEL_MAX_JOBS_PER_USER` (1) mỗi user | truyện xếp hàng |
| Số truyện | `AINOVEL_MAX_BOOKS_PER_USER` (20) mỗi user, không tính truyện đang xoá | 409 `book_limit` |
| Đĩa | tạo truyện bị chặn khi ổ chứa data còn dưới 1 GiB | 507 `disk_full` |

- **Chia lượt.** Khi server hết slot và có truyện của user khác đang chờ chạy được ngay, lượt chạy đã quá `AINOVEL_MAX_RUN_MINUTES`
  (mặc định 60) bị cắt để nhường slot, mỗi lúc một worker. Lượt bị cắt dừng sau khi chương đang viết được commit; nếu quá
  `AINOVEL_SLICE_GRACE_MINUTES` (mặc định 20) mà chưa tới ranh giới chương thì dừng hẳn. Đặt grace `0` để cắt ngay (bỏ chương
  đang viết dở). Vì vậy một slot có thể bị giữ tối đa `MAX_RUN + SLICE_GRACE` (mặc định 80 phút). Đặt `AINOVEL_MAX_RUN_MINUTES=0`
  để tắt chia lượt.
- **Retry của model.** Trong worker của bản web, `llmretry` retry tối đa 10 lần hoặc 10 phút kể từ lần gọi đầu, hết trần thì
  trả lỗi cuối, để key bị 429 kéo dài không giữ slot mãi. Trần này chỉ được đặt trong
  `ainovel-cli worker`; CLI/TUI chạy local không đặt nên vẫn retry không giới hạn.
- **Tài khoản bỏ hoang.** Tài khoản (không phải admin) tạo quá 30 ngày mà không có key, truyện lẫn session nào thì bị dọn tự động.
- **Limiter nằm trong bộ nhớ.** Mất khi restart server. Mỗi limiter có trần số khoá theo dõi (100.000): limiter ngắn đầy thì từ
  chối khoá mới (đóng an toàn); limiter theo ngày đầy thì bỏ khoá cũ nhất để không khoá mọi IP mới suốt cả ngày.
- **Tạo key không cần mật khẩu.** `POST /api/keys` chỉ cần `provider`, `label`, `api_key`. Đổi mật khẩu vẫn đòi mật khẩu cũ.

## Provider

Allowlist cố định trong binary (`internal/entry/worker/providers.go`), mỗi provider có base URL biên dịch sẵn. Server không nhận
base URL hay loại giao thức từ người dùng, nên không mở đường SSRF; Ollama, Bedrock và custom proxy vì thế chỉ có ở CLI/TUI.

| `name` | Nhãn | Base URL | Tính phí |
|---|---|---|---|
| `openrouter` | OpenRouter | `https://openrouter.ai/api/v1` | theo token |
| `anthropic` | Anthropic | mặc định của SDK | theo token |
| `gemini` | Gemini | mặc định của SDK | theo token |
| `openai` | OpenAI | mặc định của SDK | theo token |
| `deepseek` | DeepSeek | mặc định của SDK | theo token |
| `qwen` | Qwen | mặc định của SDK | theo token |
| `zai-coding` | Z.ai Coding Plan | `https://api.z.ai/api/coding/paas/v4` | thuê bao: không có ngân sách USD, giao diện hiện số token |
| `zai` | Z.ai | `https://api.z.ai/api/paas/v4` | theo token |
| `glm` | GLM (bigmodel.cn) | mặc định của SDK | theo token |
| `grok` | Grok | mặc định của SDK | theo token |

Truyện thuê bao lưu `budget_usd = 0` và báo chi phí USD bằng 0. Mỗi truyện gắn với một provider; đổi key chỉ được sang key
cùng provider (400 `provider_mismatch`), không đổi giữa `zai` và `zai-coding`.

## Kiểm key và lỗi provider

`POST /api/keys/test` chỉ nhận `key_id` của key đã lưu của chính user (không nhận key thô), theo giới hạn ở bảng trên. Server gọi
provider đúng một lần (không retry) với `max_tokens` nhỏ. Response chỉ có `ok`, `model` (model dùng để kiểm), nhóm kết quả (`kind`: `ok`, `auth_failed`,
`plan_mismatch`, `rate_limited`, `quota`, `network`, `unknown`), mã gốc nằm trong danh sách cho phép và câu thông báo do server
viết, không bao giờ chứa văn bản lỗi của provider. Kết quả là `ok` cả khi provider từ chối tham số (400/422 không nói về key hay
số dư) hoặc trả 200 nhưng hết `max_tokens` trước khi có chữ, vì khi đó provider đã nhận key.

Khi provider từ chối hẳn một lượt chạy, truyện dừng với `stop_reason = provider_rejected` (mã lỗi `provider_rejected`), không
retry ở tầng nào. Ví dụ mã 1113 của Z.ai: key không thuộc gói Coding Plan, gói hết quota hoặc model không có trong gói.
Cách xử lý: thêm key dưới đúng provider (`zai-coding` cho key gói Coding Plan, `zai` cho key trả theo token) rồi tạo truyện mới.
Truyện cũ không đổi provider được.

## Chạy trên máy bạn

Bản web là dịch vụ hosted: server lo việc chạy, người dùng chỉ cần API key. Muốn chạy trên máy riêng (mọi provider, kể cả
base URL riêng, không cần server) hãy dùng CLI/TUI; cách cài và chạy ở [README.md](../README.md#cài-đặt).

## Backup và restore

`deploy/backup.sh` dump Postgres (`pg_dump -Fc`) và nén `users/` trong thư mục data, giữ `BACKUP_KEEP` bản mới nhất (mặc định 14):

```bash
sudo install -m 0750 deploy/backup.sh /usr/local/sbin/ainovel-backup
sudo /usr/local/sbin/ainovel-backup        # ghi vào /var/backups/ainovel
```

Bản sao lưu ghi vào cùng VPS (file quyền 0600, thư mục 0700) nên **mất VPS là mất cả bản sao lưu**: phải đẩy `BACKUP_DIR` ra
ngoài máy. Ví dụ: `rsync -a /var/backups/ainovel/ user@may-khac:ainovel-backup/`, hoặc `rclone sync /var/backups/ainovel remote:ainovel`
(S3, B2...), hoặc `restic backup /var/backups/ainovel` nếu muốn mã hoá và khử trùng lặp. Đặt lịch bằng cron hoặc systemd timer
chạy backup rồi đẩy đi. Bản dump chứa băm mật khẩu và API key đã mã hoá, nên chọn nơi lưu có kiểm soát truy cập.
Master key sao lưu riêng như ở trên.

Restore (trên máy mới đã cài đặt xong bước 1-3 với **cùng master key**, service đang tắt):

```bash
sudo systemctl stop ainovel-serve
sudo -u postgres dropdb ainovel && sudo -u postgres createdb -O ainovel ainovel
sudo -u postgres pg_restore --no-owner --role=ainovel -d ainovel db-<stamp>.dump   # chạy dưới user postgres, không đưa mật khẩu lên dòng lệnh
sudo tar -C /var/lib/ainovel -xzf users-<stamp>.tar.gz
sudo chown -R ainovel:ainovel /var/lib/ainovel
sudo systemctl start ainovel-serve
```

Chưa diễn tập restore toàn phần; hãy thử trên máy phụ trước khi cần dùng thật.

## Nâng cấp

Server spawn worker bằng chính binary của mình, nên không thay binary khi đang chạy. Thứ tự đúng là stop → thay → start:

```bash
sudo systemctl stop ainovel-serve       # drain: đánh dấu truyện đang chạy là interrupted, dừng worker, tối đa ~60s
sudo install -m 0755 ainovel-cli /usr/local/bin/ainovel-cli
sudo systemctl start ainovel-serve      # truyện đang chạy và đang xếp hàng tự tiếp tục, không thành failed
```

Truyện bị ngắt lúc stop được tiếp tục tự động khi server khởi động lại. Migration DB chạy tự động lúc khởi động. Nên chạy
backup (`sudo /usr/local/sbin/ainovel-backup`, xem mục trên) trước mỗi lần nâng cấp.

### Nâng cấp lên bản này (đăng ký tự do, Z.ai)

1. Chạy `deploy/backup.sh` (đã cài thành `ainovel-backup`) trước khi deploy. Đây là đường lùi duy nhất không mất dữ liệu.
2. Thêm rate limiting rule Cloudflare như bước 5 mục Caddy và Cloudflare, trước khi mở đăng ký.
3. Deploy như trên. Hai migration tự chạy khi khởi động:
   - `0004_drop_invite_codes`: bỏ bảng `invite_codes`. Mã mời cũ không còn dùng được và không lấy lại được từ DB.
   - `0005_books_budget_subscription`: nới `CHECK` của `books.budget_usd` từ `> 0` thành `>= 0`, để truyện thuê bao (Z.ai Coding Plan) lưu ngân sách 0.
4. Lệnh `serve invite` đã bị gỡ. Đăng ký không còn mã mời.
5. Người đang mở tab cũ cần lấy bản mới: bấm "Cập nhật" trên thông báo có phiên bản mới, hoặc đóng hết tab của trang rồi mở lại.
   F5 thôi chưa đủ, vì service worker vẫn phục vụ bản cũ cho tới khi được cập nhật. Form cũ còn field đã bỏ (mã mời, mật khẩu
   khi thêm key) nên bị từ chối 400.

### Rollback

Chạy binary cũ trên DB đã lên 0004/0005 sẽ hỏng: đăng ký trả 500 (binary cũ ghi vào bảng `invite_codes` đã mất) và truyện thuê bao
không chạy được (binary cũ không biết provider `zai-coding`). Chọn một trong hai cách:

- **Restore backup** trước nâng cấp (mục Backup và restore). Mất mọi dữ liệu sau thời điểm backup.
- **Goose Down 0005 → 0004 rồi 0004 → 0003**, sau đó cài lại binary cũ. Cần có `goose` CLI (repo không kèm lệnh này; migration
  dùng goose v3.27.0 và bảng mặc định `goose_db_version`). Service phải đang tắt:

  ```bash
  sudo systemctl stop ainovel-serve
  go install github.com/pressly/goose/v3/cmd/goose@v3.27.0
  # chạy trong thư mục mã nguồn của bản mới; mỗi lần `down` lùi một migration.
  # Chuỗi kết nối đi qua biến môi trường, không nằm trên dòng lệnh (`ps` không thấy mật khẩu).
  export GOOSE_DRIVER=postgres GOOSE_DBSTRING="$(sudo cat /etc/ainovel/database-url)"
  goose -dir internal/server/db/migrations down   # 0005 → 0004
  goose -dir internal/server/db/migrations down   # 0004 → 0003
  unset GOOSE_DBSTRING
  ```

  Down 0005 đổi `budget_usd = 0` thành `0.01` rồi siết lại `CHECK (budget_usd > 0)` (không dùng `NOT VALID`, vì Postgres vẫn kiểm
  CHECK ở mọi UPDATE). Truyện thuê bao mất ngân sách 0 và chỉ dừng với lỗi init trên binary cũ. Down 0004 dựng lại bảng
  `invite_codes` rỗng. Chưa diễn tập trên DB thật; thử trên máy phụ trước.

## Giới hạn hiện tại

- Worker cùng UID `ainovel` với server (chưa có UID riêng). Giảm thiểu: secret không nằm trong env của server hay worker,
  worker chỉ nhận bốn biến môi trường, `notify` bị tắt, API key chỉ đi qua stdin. UID riêng để sau.
- Timeline realtime không giữ qua lần restart server; sau restart trình duyệt dựng lại từ snapshot.
- Đăng ký tự do, chưa có xác minh email hay quên mật khẩu. Rủi ro thật: ai cũng đăng ký được bằng email của người khác (chiếm email
  trước khi chủ thật tới), và có thể dò xem một email đã đăng ký chưa qua lỗi 409 `email_taken`. Chưa có captcha; nếu log cho thấy
  đăng ký hàng loạt (limiter 10/ngày/IP bị chạm thường xuyên) thì thêm Cloudflare Turnstile cho `/api/auth/signup`.
- Chưa xoay vòng master key; một master key duy nhất.
- Chưa đưa lên web: đồng sáng tác, import truyện có sẵn, đổi model theo vai trò hoặc giữa chừng, sửa chương bằng tay.
- Một VPS một server (không chạy nhiều bản server dùng chung DB và thư mục data).
- iOS PWA ngắt SSE khi khoá màn hình; trình duyệt tự nối lại khi mở lại.

## Xử lý sự cố

| Triệu chứng | Nguyên nhân thường gặp |
|---|---|
| `serve: thiếu AINOVEL_DATABASE_URL_FILE` / `AINOVEL_MASTER_KEY_FILE` | Chưa đặt biến, hoặc `LoadCredential` trỏ sai file. `systemctl status` và `journalctl -u ainovel-serve` cho thấy lý do. |
| `master key phải là 32 byte...` | File master key sai định dạng. Tạo lại bằng `openssl rand -base64 32` (và chấp nhận mất các API key đã lưu nếu đổi key). |
| `AINOVEL_DATABASE_URL chỉ dùng được khi AINOVEL_DEV=true` | Đang đặt chuỗi kết nối trong env. Dùng `AINOVEL_DATABASE_URL_FILE`. |
| Migration lỗi `citext` | Chủ DB không tạo được extension: cài `postgresql-contrib` hoặc `CREATE EXTENSION citext` bằng superuser trong DB `ainovel`. |
| Request ghi trả 403 `cross_origin` | `AINOVEL_PUBLIC_URL` không khớp URL trên trình duyệt (scheme, tên miền), hoặc proxy làm đổi header `Host`. |
| Cảnh báo "chưa đặt AINOVEL_TRUSTED_PROXIES" | Đặt `AINOVEL_TRUSTED_PROXIES=127.0.0.1/32` khi chạy sau Caddy. |
| Nhiều người bị giới hạn đăng nhập cùng lúc | Rate limit đang dùng chung một IP vì proxy chưa được tin; xem dòng trên. |
| 524 hoặc SSE đứt sau ~100 giây | Caddy đang đệm (thiếu `flush_interval -1`) hoặc có lớp nén ở giữa; kiểm tra `Caddyfile`. |
| Truyện không bắt đầu / đứng ở hàng đợi | Đạt `AINOVEL_MAX_JOBS` hoặc `AINOVEL_MAX_JOBS_PER_USER`; xem `journalctl`. |
| Worker bị giết khi thiếu RAM | Kernel giết worker trước server (`oom_score_adj=500`); `OOMPolicy=continue` giữ server và các truyện khác chạy tiếp. Tăng `MemoryMax` hoặc giảm `AINOVEL_MAX_JOBS`; mở truyện để xem trạng thái và bấm tiếp tục nếu cần. Nếu cả service bị restart khi OOM, kiểm tra unit đã có `OOMPolicy=continue` và đã `daemon-reload`. |
| Cloudflare trả 525/526 | Caddy chưa có Origin Certificate: kiểm tra hai file `/etc/caddy/cf-origin.*` đọc được bởi nhóm `caddy` và `journalctl -u caddy`. |
| `/healthz` trả `db: error` | Postgres không kết nối được. `disk_low: true` nghĩa là ổ chứa data còn dưới 1 GiB. |
| `Permission denied` ghi vào data dir | `/var/lib/ainovel` phải thuộc user `ainovel`; unit chỉ cho phép ghi ở đó. |
