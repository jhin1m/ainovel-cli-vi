import { Link } from '@tanstack/react-router'
import { useEffect, useId, useRef, useState, type FormEvent } from 'react'
import { ApiError, errorMessage } from '@/api/client'
import { useBooks, useCreateBook, useKeys, useProviders } from '@/api/queries'
import type { APIKey, Book } from '@/api/types.gen'
import { FormError } from '@/components/ui/field'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { msg } from '@/lib/messages'
import { ConfigPanel } from './config-panel'
import {
  DEFAULT_BUDGET_USD,
  EMPTY_PROMPT_OPTIONS,
  composePrompt,
  modelsForKey,
  parseBudget,
  pickDefaultModel,
  validateNewBook,
  type NewBookErrors,
  type PromptOptions,
} from './new-book-logic'
import { PromptSection } from './prompt-section'

/**
 * Trang tạo truyện: cột trái là ý tưởng (PromptSection), cột phải là thẻ cấu hình
 * sticky (ConfigPanel). Chưa có key thì thêm ngay trong thẻ cấu hình, không rời trang.
 *
 * Thẻ cấu hình chứa `AddKeyForm` (một <form> riêng) nên nằm ngoài <form> tạo truyện:
 * <form> lồng nhau là HTML sai và submit của form con nổi bọt lên form cha. Các ô của
 * thẻ cấu hình nối vào form qua thuộc tính `form={formId}`.
 */
interface Props {
  onCreated: (b: Book) => void
  /** Ý tưởng điền sẵn (vd. từ truyện bị provider từ chối). */
  initialIdea?: string
  /** Chưa chọn tay thì ưu tiên key của provider khác provider này (provider vừa từ chối). */
  avoidProvider?: string
}

export function NewBookForm({ onCreated, initialIdea = '', avoidProvider }: Props) {
  const keys = useKeys()
  const providers = useProviders()
  // Chỉ dùng để ước tính số chương; đang tải hoặc lỗi thì không hiện ước tính.
  const books = useBooks()
  const create = useCreateBook()
  const formId = useId()
  const rootRef = useRef<HTMLDivElement>(null)

  const [idea, setIdea] = useState(initialIdea)
  const [options, setOptions] = useState<PromptOptions>(EMPTY_PROMPT_OPTIONS)
  const [keyId, setKeyId] = useState('')
  const [modelId, setModelId] = useState('')
  const [budget, setBudget] = useState(String(DEFAULT_BUDGET_USD))
  const [review, setReview] = useState(true)
  const [errors, setErrors] = useState<NewBookErrors>({})
  const [attempt, setAttempt] = useState(0)
  const [adding, setAdding] = useState(false)
  const [addRound, setAddRound] = useState(0)

  // Sau mỗi lần bấm gửi bị lỗi, đưa focus tới ô lỗi đầu tiên.
  useEffect(() => {
    if (attempt > 0) rootRef.current?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()
  }, [attempt])

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

  const key =
    keys.data.find((k) => k.id === keyId) ?? keys.data.find((k) => k.provider !== avoidProvider) ?? keys.data[0]
  const provider = providers.data.find((p) => p.name === key?.provider)
  const subscription = Boolean(provider?.subscription)
  const models = modelsForKey(key, providers.data)
  // Chưa chọn tay (hoặc vừa đổi key) thì dùng model đề xuất của provider.
  const model = models.find((m) => m.id === modelId) ?? pickDefaultModel(models)
  const composed = composePrompt(idea, options)

  function submit(e: FormEvent) {
    e.preventDefault()
    const input = { prompt: composed, keyId: key?.id ?? '', model: model?.id ?? '', budget }
    const errs = validateNewBook(input, { subscription })
    setErrors(errs)
    if (Object.keys(errs).length > 0) {
      setAttempt((n) => n + 1)
      return
    }
    create.mutate(
      {
        prompt: composed,
        api_key_id: input.keyId,
        model: input.model,
        // Gói thuê bao không có ngân sách USD; server cũng bỏ qua và lưu 0.
        budget_usd: subscription ? 0 : parseBudget(budget),
        advance_mode: review ? 'review' : 'auto',
      },
      { onSuccess: onCreated },
    )
  }

  function keySaved(k: APIKey) {
    setKeyId(k.id)
    setModelId('')
    // Giữ khối kết quả của AddKeyForm (kết quả tự kiểm) cạnh ô chọn key tới khi bấm "Xong".
    setAdding(true)
  }

  function addOpen(open: boolean) {
    setAdding(open)
    if (!open) setAddRound((n) => n + 1)
  }

  const errCode = create.error instanceof ApiError ? create.error.code : ''
  const serverError = create.isError ? (
    <FormError>
      {errCode === 'disk_full' ? msg.newBook.diskFull : errorMessage(create.error)}
      {errCode === 'book_limit' && (
        <>
          {' '}
          <Link to="/" className="font-medium">
            {msg.newBook.bookLimitLink}
          </Link>
        </>
      )}
    </FormError>
  ) : undefined

  return (
    <div
      ref={rootRef}
      className="flex flex-col gap-6 px-5 pt-2 pb-7 md:px-0 lg:grid lg:grid-cols-[minmax(0,1fr)_minmax(20rem,24rem)] lg:items-start lg:gap-8"
    >
      <form id={formId} onSubmit={submit} noValidate className="min-w-0">
        <PromptSection
          idea={idea}
          onIdea={setIdea}
          options={options}
          onOptions={setOptions}
          composed={composed}
          error={errors.prompt}
        />
      </form>
      <ConfigPanel
        formId={formId}
        keys={keys.data}
        providers={providers.data}
        books={books.data}
        apiKey={key}
        provider={provider}
        models={models}
        model={model}
        budget={budget}
        review={review}
        errors={errors}
        adding={adding}
        addRound={addRound}
        pending={create.isPending}
        serverError={serverError}
        onKey={(id) => {
          setKeyId(id)
          setModelId('')
        }}
        onModel={setModelId}
        onBudget={setBudget}
        onReview={setReview}
        onAddOpen={addOpen}
        onKeySaved={keySaved}
      />
    </div>
  )
}
