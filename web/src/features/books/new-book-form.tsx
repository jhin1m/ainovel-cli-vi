import { Link } from '@tanstack/react-router'
import { ArrowRight } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { errorMessage } from '@/api/client'
import { useCreateBook, useKeys, useProviders } from '@/api/queries'
import type { Book } from '@/api/types.gen'
import { buttonClass } from '@/components/ui/button'
import { FormError, inputClass } from '@/components/ui/field'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { formatPer1M } from '@/lib/format'
import { msg } from '@/lib/messages'
import {
  DEFAULT_BUDGET_USD,
  MAX_PROMPT_RUNES,
  clampPrompt,
  modelsForKey,
  parseBudget,
  runeLength,
  validateNewBook,
  type NewBookErrors,
} from './new-book-logic'

export function NewBookForm({ onCreated }: { onCreated: (b: Book) => void }) {
  const keys = useKeys()
  const providers = useProviders()
  const create = useCreateBook()

  const [prompt, setPrompt] = useState('')
  const [keyId, setKeyId] = useState('')
  const [modelId, setModelId] = useState('')
  const [budget, setBudget] = useState(String(DEFAULT_BUDGET_USD))
  const [review, setReview] = useState(true)
  const [errors, setErrors] = useState<NewBookErrors>({})

  if (keys.isPending || providers.isPending) return <LoadingState />
  if (keys.isError) {
    return <ErrorState title={msg.keys.loadError} detail={errorMessage(keys.error)} onRetry={() => void keys.refetch()} />
  }
  if (providers.isError) {
    return (
      <ErrorState
        title={msg.newBook.providersError}
        detail={errorMessage(providers.error)}
        onRetry={() => void providers.refetch()}
      />
    )
  }
  if (keys.data.length === 0) {
    return (
      <EmptyState
        title={msg.newBook.noKeyTitle}
        hint={msg.newBook.noKeyBody}
        action={
          <Link to="/settings/keys" className={buttonClass('primary', 'text-on-accent no-underline hover:text-on-accent')}>
            {msg.newBook.noKeyAction}
          </Link>
        }
      />
    )
  }

  const providerLabel = (name: string) => providers.data.find((p) => p.name === name)?.label ?? name
  const key = keys.data.find((k) => k.id === keyId) ?? keys.data[0]
  const models = modelsForKey(key, providers.data)
  const model = models.find((m) => m.id === modelId) ?? models[0]

  function submit(e: FormEvent) {
    e.preventDefault()
    const input = { prompt, keyId: key?.id ?? '', model: model?.id ?? '', budget }
    const errs = validateNewBook(input)
    setErrors(errs)
    if (Object.keys(errs).length > 0) return
    create.mutate(
      {
        prompt: prompt.trim(),
        api_key_id: input.keyId,
        model: input.model,
        budget_usd: parseBudget(budget),
        advance_mode: review ? 'review' : 'auto',
      },
      { onSuccess: onCreated },
    )
  }

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-5 px-5 pt-2 pb-7">
      <div className="flex flex-col gap-2.5">
        <label htmlFor="prompt" className="font-serif text-[28px] leading-tight font-semibold tracking-tight">
          {msg.newBook.heading}
        </label>
        <textarea
          id="prompt"
          rows={5}
          value={prompt}
          onChange={(e) => setPrompt(clampPrompt(e.target.value))}
          placeholder={msg.newBook.placeholder}
          aria-invalid={errors.prompt ? true : undefined}
          aria-describedby="prompt-hint"
          className="box-border w-full rounded-[14px] border border-line-input bg-surface px-4 py-3.5 font-serif text-[17px] leading-relaxed text-fg"
        />
        <div id="prompt-hint" className="flex justify-between gap-3 text-[13px] text-muted">
          <span>{msg.newBook.hint}</span>
          <span className="shrink-0">
            {runeLength(prompt)}/{MAX_PROMPT_RUNES}
          </span>
        </div>
        {errors.prompt && (
          <span role="alert" className="text-[13px] text-st-error">
            {errors.prompt}
          </span>
        )}
      </div>

      <div className="flex flex-col gap-2.5">
        <span className="text-[13px] font-semibold tracking-wider text-muted uppercase">
          {msg.newBook.ideasTitle}
        </span>
        <div className="flex flex-wrap gap-2">
          {msg.newBook.ideas.map((i) => (
            <button
              key={i.label}
              type="button"
              onClick={() => setPrompt(i.text)}
              className="min-h-11 cursor-pointer rounded-full border border-line bg-surface px-3.5 text-sm text-fg"
            >
              {i.label}
            </button>
          ))}
        </div>
      </div>

      <fieldset className="m-0 flex flex-col gap-3.5 rounded-[14px] border border-line bg-surface p-4">
        <legend className="px-1.5 text-[13px] font-semibold tracking-wider text-muted uppercase">
          {msg.newBook.modelSection}
        </legend>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="key" className="text-sm font-medium">
            {msg.newBook.apiKey}
          </label>
          <select
            id="key"
            className={inputClass}
            value={key?.id ?? ''}
            onChange={(e) => {
              setKeyId(e.target.value)
              setModelId('')
            }}
          >
            {keys.data.map((k) => (
              <option key={k.id} value={k.id}>
                {[providerLabel(k.provider), k.label].filter(Boolean).join(' · ')} · ••••{k.last4}
              </option>
            ))}
          </select>
        </div>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="model" className="text-sm font-medium">
            {msg.newBook.model}
          </label>
          <select
            id="model"
            className={inputClass}
            value={model?.id ?? ''}
            onChange={(e) => setModelId(e.target.value)}
            disabled={models.length === 0}
            aria-invalid={errors.model ? true : undefined}
          >
            {models.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
              </option>
            ))}
          </select>
          {models.length === 0 ? (
            <span role="alert" className="text-[13px] text-st-error">
              {msg.newBook.noModels}
            </span>
          ) : model ? (
            <span className="text-[13px] text-muted">
              {msg.newBook.pricing(formatPer1M(model.input_cost_per_1m), formatPer1M(model.output_cost_per_1m))}
            </span>
          ) : null}
        </div>
      </fieldset>

      <div className="flex flex-col gap-1.5">
        <label htmlFor="budget" className="text-sm font-medium">
          {msg.newBook.budget}
        </label>
        <div className="flex items-center gap-2.5">
          <input
            id="budget"
            type="number"
            inputMode="decimal"
            min={1}
            max={50}
            step="any"
            value={budget}
            onChange={(e) => setBudget(e.target.value)}
            aria-invalid={errors.budget ? true : undefined}
            className={`${inputClass} !w-30`}
          />
          <span className="text-[15px] text-muted">US$</span>
        </div>
        <span className="text-[13px] text-muted">{msg.newBook.budgetHint}</span>
        {errors.budget && (
          <span role="alert" className="text-[13px] text-st-error">
            {errors.budget}
          </span>
        )}
      </div>

      <div className="flex items-start justify-between gap-4 rounded-[14px] border border-line bg-surface p-4">
        <div className="flex flex-col gap-1">
          <label htmlFor="review" className="text-[15px] font-semibold">
            {msg.newBook.review}
          </label>
          <span className="text-[13px] leading-normal text-muted">{msg.newBook.reviewHint}</span>
        </div>
        <button
          id="review"
          type="button"
          role="switch"
          aria-checked={review}
          onClick={() => setReview((v) => !v)}
          className={`flex h-8 w-[52px] shrink-0 cursor-pointer items-center rounded-2xl border-0 p-[3px] ${
            review ? 'justify-end bg-accent' : 'justify-start bg-switch-off'
          }`}
        >
          <span className="size-[26px] rounded-full bg-white shadow-sm" />
        </button>
      </div>

      {create.isError && <FormError>{errorMessage(create.error)}</FormError>}

      <button
        type="submit"
        disabled={create.isPending}
        className={buttonClass('primary', 'min-h-[52px] !rounded-[14px] text-base')}
      >
        {create.isPending ? msg.newBook.submitting : msg.newBook.submit}
        <ArrowRight aria-hidden size={20} />
      </button>
      <p className="-mt-2 mb-0 text-center text-[13px] text-muted">{msg.newBook.submitHint}</p>
    </form>
  )
}
