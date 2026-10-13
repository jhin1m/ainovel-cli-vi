import { ArrowRight, Plus } from 'lucide-react'
import { useId, type ReactNode } from 'react'
import type { APIKey, Book, ModelOption, ProviderInfo } from '@/api/types.gen'
import { ProviderIcon } from '@/components/provider-icon'
import { buttonClass } from '@/components/ui/button'
import { inputClass } from '@/components/ui/field'
import { AddKeyForm } from '@/features/keys/add-key-form'
import { formatPer1M } from '@/lib/format'
import { msg } from '@/lib/messages'
import {
  BUDGET_CHIPS,
  MAX_BUDGET_USD,
  estimateChapters,
  parseBudget,
  priceTiers,
  type NewBookErrors,
} from './new-book-logic'

interface Props {
  /** id của <form> tạo truyện; thẻ này nằm ngoài form nên các ô nối vào bằng `form`. */
  formId: string
  keys: APIKey[]
  providers: ProviderInfo[]
  /** undefined khi danh sách truyện chưa tải xong (chưa biết có số đo hay không). */
  books?: Book[]
  /** Key đang chọn (undefined khi chưa có key nào). */
  apiKey?: APIKey
  provider?: ProviderInfo
  models: ModelOption[]
  model?: ModelOption
  budget: string
  review: boolean
  errors: NewBookErrors
  /** Đang hiện `AddKeyForm` (luôn hiện khi chưa có key). */
  adding: boolean
  /** Đổi giá trị này để remount `AddKeyForm` thành form trống. */
  addRound: number
  pending: boolean
  serverError?: ReactNode
  onKey: (id: string) => void
  onModel: (id: string) => void
  onBudget: (v: string) => void
  onReview: (v: boolean) => void
  onAddOpen: (open: boolean) => void
  onKeySaved: (k: APIKey) => void
}

/** Thẻ "Cấu hình" của form tạo truyện; state nằm ở form cha. */
export function ConfigPanel(p: Props) {
  const ids = useId()
  const label = (name: string) => p.providers.find((x) => x.name === name)?.label ?? name
  const subscription = Boolean(p.provider?.subscription)
  const hasKeys = p.keys.length > 0
  const showAdd = p.adding || !hasKeys

  return (
    <section
      aria-labelledby={`${ids}-title`}
      // Thẻ có thể cao hơn màn (vd. 1366×768 khi form thêm key đang mở): giới hạn chiều cao (trừ cả header trang),
      // cuộn bên trong, và khối nút gửi dính đáy thẻ để luôn thấy được.
      className="flex flex-col gap-5 rounded-card border border-line-card bg-surface p-4 shadow-card lg:sticky lg:top-4 lg:max-h-[calc(100dvh-5.5rem)] lg:overflow-y-auto"
    >
      <h2 id={`${ids}-title`} className="m-0 font-serif text-xl font-semibold">
        {msg.newBook.config}
      </h2>

      <div className="flex flex-col gap-2">
        {hasKeys ? (
          <>
            <label htmlFor="key" className="text-sm font-medium">
              {msg.newBook.apiKey}
            </label>
            <div className="relative">
              {p.apiKey && (
                <ProviderIcon
                  name={p.apiKey.provider}
                  size={20}
                  className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2"
                />
              )}
              <select
                id="key"
                form={p.formId}
                className={`${inputClass} pl-11`}
                value={p.apiKey?.id ?? ''}
                onChange={(e) => p.onKey(e.target.value)}
              >
                {p.keys.map((k) => (
                  <option key={k.id} value={k.id}>
                    {[label(k.provider), k.label].filter(Boolean).join(' · ')} · ••••{k.last4}
                  </option>
                ))}
              </select>
            </div>
            {!showAdd && (
              <button
                type="button"
                onClick={() => p.onAddOpen(true)}
                className="inline-flex min-h-11 cursor-pointer items-center gap-1.5 self-start border-0 bg-transparent px-0 text-sm font-medium text-accent"
              >
                <Plus aria-hidden size={16} />
                {msg.newBook.addKey}
              </button>
            )}
          </>
        ) : (
          <div className="flex flex-col gap-1">
            <h3 className="m-0 text-base font-semibold">{msg.newBook.noKeyTitle}</h3>
            <p className="m-0 text-sm text-muted">{msg.newBook.noKeyBody}</p>
          </div>
        )}
        {p.errors.keyId && (
          <span role="alert" className="text-sm text-st-error">
            {p.errors.keyId}
          </span>
        )}
        {showAdd && (
          // Cùng vị trí trong cây khi số key đổi 0 → 1, nên form không bị remount và
          // kết quả tự kiểm key vẫn nằm cạnh ô chọn key cho tới khi bấm "Xong".
          // Thẻ cấu hình hẹp (≤24rem) nên lưới provider giữ 2 cột ở mọi cỡ màn.
          <div
            className={`[&_[role=radiogroup]]:grid-cols-2 ${hasKeys ? 'rounded-control border border-line p-3' : ''}`}
          >
            <AddKeyForm
              key={p.addRound}
              compact
              onSaved={p.onKeySaved}
              onClose={hasKeys ? () => p.onAddOpen(false) : undefined}
            />
          </div>
        )}
      </div>

      {hasKeys && <ModelField {...p} />}
      {hasKeys && (subscription ? <SubscriptionNote label={label(p.provider?.name ?? '')} /> : <BudgetField {...p} />)}

      <div className="flex items-start justify-between gap-4 border-t border-line pt-4">
        <div className="flex flex-col gap-1">
          <label htmlFor="review" className="text-[0.9375rem] font-semibold">
            {msg.newBook.review}
          </label>
          <span className="text-sm leading-normal text-muted">{msg.newBook.reviewHint}</span>
        </div>
        <button
          id="review"
          type="button"
          role="switch"
          aria-checked={p.review}
          onClick={() => p.onReview(!p.review)}
          className={`flex h-8 w-13 shrink-0 cursor-pointer items-center rounded-2xl border-0 p-0.75 ${
            p.review ? 'justify-end bg-accent' : 'justify-start bg-switch-off'
          }`}
        >
          <span className="size-6.5 rounded-full bg-white shadow-sm" />
        </button>
      </div>

      {p.serverError}

      <div className="flex flex-col gap-2 lg:sticky lg:-bottom-4 lg:-mx-4 lg:-mb-4 lg:border-t lg:border-line lg:bg-surface lg:px-4 lg:pt-3 lg:pb-4">
        <button
          type="submit"
          form={p.formId}
          disabled={p.pending}
          className={buttonClass('primary', 'min-h-13 !rounded-card text-base')}
        >
          {p.pending ? msg.newBook.submitting : msg.newBook.submit}
          <ArrowRight aria-hidden size={20} />
        </button>
        <p className="m-0 text-center text-sm text-muted">{msg.newBook.submitHint}</p>
      </div>
    </section>
  )
}

function ModelField({ formId, models, model, provider, errors, onModel }: Props) {
  const ids = useId()
  // Gói thuê bao không trừ USD nên bậc giá không có ý nghĩa.
  const tiers = provider?.subscription ? null : priceTiers(models)
  const tier = model && tiers?.get(model.id)
  const hasBadges = Boolean(model && (model.recommended || tier))
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor="model" className="text-sm font-medium">
        {msg.newBook.model}
      </label>
      <select
        id="model"
        form={formId}
        className={inputClass}
        value={model?.id ?? ''}
        onChange={(e) => onModel(e.target.value)}
        disabled={models.length === 0}
        aria-invalid={errors.model ? true : undefined}
        aria-describedby={hasBadges ? `${ids}-badges` : undefined}
      >
        {models.map((m) => {
          const t = tiers?.get(m.id)
          return (
            <option key={m.id} value={m.id}>
              {[m.name, m.recommended && msg.newBook.recommended, t && msg.newBook.tiers[t]]
                .filter(Boolean)
                .join(' · ')}
            </option>
          )
        })}
      </select>
      {models.length === 0 ? (
        <span role="alert" className="text-sm text-st-error">
          {msg.newBook.noModels}
        </span>
      ) : (
        model && (
          <>
            {hasBadges && (
              <span id={`${ids}-badges`} className="flex flex-wrap gap-1.5">
                {model.recommended && (
                  <span className="rounded-full bg-warm-bg px-2 py-0.5 text-xs font-semibold text-warm">
                    {msg.newBook.recommended}
                  </span>
                )}
                {tier && (
                  <span className="rounded-full bg-track px-2 py-0.5 text-xs font-medium text-muted">
                    {msg.newBook.tiers[tier]}
                  </span>
                )}
              </span>
            )}
            {!provider?.subscription && (
              <details className="text-sm">
                <summary className="flex min-h-11 cursor-pointer items-center text-muted">
                  {msg.newBook.pricingTitle}
                </summary>
                <p className="m-0 leading-relaxed text-muted">
                  {msg.newBook.pricing(formatPer1M(model.input_cost_per_1m), formatPer1M(model.output_cost_per_1m))}
                </p>
              </details>
            )}
          </>
        )
      )}
    </div>
  )
}

function SubscriptionNote({ label }: { label: string }) {
  return <p className="callout m-0 text-sm leading-relaxed">{msg.newBook.subscription(label)}</p>
}

function BudgetField({ formId, budget, books, model, errors, onBudget }: Props) {
  const ids = useId()
  const value = parseBudget(budget)
  const valid = budget.trim() !== '' && Number.isFinite(value) && value > 0 && value <= MAX_BUDGET_USD
  const est = model && valid && books ? estimateChapters(value, books, model.id) : null
  return (
    <div className="flex flex-col gap-2">
      <label htmlFor="budget" className="text-sm font-medium">
        {msg.newBook.budget}
      </label>
      <div role="group" aria-label={msg.newBook.budgetChips} className="flex flex-wrap gap-2">
        {BUDGET_CHIPS.map((c) => (
          <button
            key={c}
            type="button"
            aria-pressed={value === c}
            onClick={() => onBudget(String(c))}
            className={`min-h-11 min-w-14 cursor-pointer rounded-full border px-3 text-sm tabular-nums ${
              value === c ? 'border-accent bg-accent text-on-accent' : 'border-line bg-surface text-fg hover:bg-track'
            }`}
          >
            {c} US$
          </button>
        ))}
      </div>
      <div className="flex items-center gap-2.5">
        <input
          id="budget"
          form={formId}
          type="number"
          inputMode="decimal"
          min={1}
          max={MAX_BUDGET_USD}
          step="any"
          value={budget}
          onChange={(e) => onBudget(e.target.value)}
          aria-invalid={errors.budget ? true : undefined}
          aria-describedby={`${ids}-hint ${ids}-est${errors.budget ? ` ${ids}-err` : ''}`}
          className={`${inputClass} !w-30`}
        />
        <span className="text-[0.9375rem] text-muted">US$</span>
      </div>
      <span id={`${ids}-hint`} className="text-sm text-muted">
        {msg.newBook.budgetHint}
      </span>
      <span id={`${ids}-est`} className="text-sm text-fg">
        {model && valid && books
          ? est
            ? est.max < 1
              ? msg.newBook.estimateLow
              : msg.newBook.estimate(est.min, est.max)
            : msg.newBook.estimateNone
          : null}
      </span>
      {errors.budget && (
        <span id={`${ids}-err`} role="alert" className="text-sm text-st-error">
          {errors.budget}
        </span>
      )}
    </div>
  )
}
