// Vite xoá sạch web/dist (emptyOutDir); go:embed cần thư mục luôn có .gitkeep.
import { writeFileSync } from 'node:fs'
writeFileSync(new URL('../dist/.gitkeep', import.meta.url), '')
