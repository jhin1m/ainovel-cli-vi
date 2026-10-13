/** Thông tin phụ của provider mà server không trả: trang lấy key và một dòng mô tả. */
export interface ProviderMeta {
  /** Trang tạo API key chính thức của provider. */
  keyUrl: string
  /** Một dòng mô tả cách tính tiền / điểm khác biệt. */
  blurb: string
}

// URL đã mở để kiểm ngày 2026-10-13. OpenAI, DeepSeek và x.ai trả 403 cho curl
// (trang chặn bot), các trang còn lại trả 200 hoặc chuyển tới trang đăng nhập.
const meta: Record<string, ProviderMeta> = {
  openrouter: {
    keyUrl: 'https://openrouter.ai/settings/keys',
    blurb: 'Một key dùng được model của nhiều hãng, tính theo token.',
  },
  anthropic: {
    keyUrl: 'https://platform.claude.com/settings/keys',
    blurb: 'Model Claude của Anthropic, tính theo token.',
  },
  gemini: {
    keyUrl: 'https://aistudio.google.com/apikey',
    blurb: 'Model Gemini của Google, tính theo token.',
  },
  openai: {
    keyUrl: 'https://platform.openai.com/api-keys',
    blurb: 'Model GPT của OpenAI, tính theo token.',
  },
  deepseek: {
    keyUrl: 'https://platform.deepseek.com/api_keys',
    blurb: 'Model DeepSeek, giá rẻ, tính theo token.',
  },
  qwen: {
    keyUrl: 'https://bailian.console.aliyun.com/?tab=model#/api-key',
    blurb: 'Model Qwen qua Alibaba Cloud Bailian (Trung Quốc), tính theo token.',
  },
  'zai-coding': {
    keyUrl: 'https://z.ai/manage-apikey/apikey-list',
    blurb: 'Gói thuê bao, tính bằng token, không trừ USD.',
  },
  zai: {
    keyUrl: 'https://z.ai/manage-apikey/apikey-list',
    blurb: 'Model GLM qua api.z.ai, trả tiền theo token.',
  },
  glm: {
    keyUrl: 'https://bigmodel.cn/usercenter/proj-mgmt/apikeys',
    blurb: 'Model GLM qua bigmodel.cn (Trung Quốc), tính theo token.',
  },
  grok: {
    keyUrl: 'https://console.x.ai/',
    blurb: 'Model Grok của xAI, tính theo token.',
  },
}

/** Meta của provider; provider lạ trả `undefined`. */
export function providerMeta(name: string): ProviderMeta | undefined {
  return Object.hasOwn(meta, name) ? meta[name] : undefined
}
