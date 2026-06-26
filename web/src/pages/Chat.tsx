import React, { useState, useRef, useEffect } from 'react'
import { Icon } from '@iconify/react'
import { icons } from '@/lib/icons'
import { detectProvider } from '@/lib/providers'
import ReactMarkdown from 'react-markdown'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { vscDarkPlus, oneLight } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { Button } from '../components/ui/button'
import { Textarea } from '../components/ui/textarea'
import { Switch } from '../components/ui/switch'
import { Slider } from '../components/ui/slider'
import { StatusChip } from '@/components/common/status-chip'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '../components/ui/command'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '../components/ui/popover'
import { getModels, getRoutes } from '../lib/api'
import { cn } from '../lib/utils'
import { getModelIcon } from '../lib/chat-utils'
import { useChatMessages, type MessageContent } from '../hooks/useChatMessages'
import { useColorMode } from '@/contexts/ColorModeContext'

// Model Combobox Component
function ModelCombobox({
  models,
  selectedModel,
  onModelChange,
  compact = false,
}: {
  models: any[]
  selectedModel: string
  onModelChange: (model: string) => void
  compact?: boolean
}) {
  const [open, setOpen] = useState(false)

  const selectedModelData = models.find(model => model.id === selectedModel)

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className={cn("justify-between", compact ? "h-8 text-sm px-3" : "w-full")}
        >
          <div className="flex items-center gap-2">
            {selectedModelData && (
              <Icon
                icon={detectProvider(selectedModelData.id, selectedModelData.owned_by || "").icon}
                className="h-4 w-4"
              />
            )}
            <span className="truncate max-w-[200px]">
              {selectedModelData ? (selectedModelData.name || selectedModelData.id) : "Select model..."}
            </span>
          </div>
          <Icon icon={icons.chevronsUpDown} className="ml-2 h-3 w-3 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[320px] p-0">
        <Command>
          <CommandInput placeholder="Search models..." className="h-9" />
          <CommandList>
            <CommandEmpty>No model found.</CommandEmpty>
            <CommandGroup>
              {models
                .filter((model) => model.id && model.id.trim() !== '')
                .map((model) => (
                  <CommandItem
                    key={model.id}
                    value={model.id}
                    onSelect={() => {
                      onModelChange(model.id === selectedModel ? "" : model.id)
                      setOpen(false)
                    }}
                  >
                    <div className="flex items-center gap-2 flex-1">
                      <Icon
                        icon={getModelIcon(model.id)}
                        className="h-4 w-4"
                      />
                      <div className="flex flex-col">
                        <span className="text-sm font-medium">
                          {model.name || model.id}
                        </span>
                        {model.name && model.name !== model.id && (
                          <span className="text-xs text-muted-foreground">
                            {model.id}
                          </span>
                        )}
                      </div>
                    </div>
                    <Icon
                      icon={icons.check}
                      className={cn(
                        "ml-auto h-4 w-4",
                        selectedModel === model.id ? "opacity-100" : "opacity-0"
                      )}
                    />
                  </CommandItem>
                ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}


// Prose wrapper for rendered markdown. `prose-pre:*` resets neutralize the
// dark <pre> frame Typography adds around fenced code (the highlighter draws
// its own panel), and `prose-code:before/after:content-['']` strips the
// literal backtick quotes Typography injects around inline <code>.
const PROSE_CLASS =
  "prose prose-sm max-w-none dark:prose-invert break-words " +
  "prose-pre:m-0 prose-pre:bg-transparent prose-pre:p-0 " +
  "prose-code:before:content-[''] prose-code:after:content-['']"

function useMarkdownComponents() {
  const { resolvedMode } = useColorMode()
  const isDark = resolvedMode === 'dark'
  return {
    code({ className, children }: any) {
      const match = /language-(\w+)/.exec(className || '')
      const text = String(children ?? '')
      // Treat anything with a language hint OR a newline as a block; the rest
      // (short, single-line) renders as inline code.
      const isBlock = !!match || text.includes('\n')
      if (!isBlock) {
        return (
          <code className="rounded bg-surface-2 px-1.5 py-0.5 font-mono text-[0.85em] text-foreground">
            {children}
          </code>
        )
      }
      return (
        <div className="my-2 overflow-hidden rounded-lg border border-border">
          <SyntaxHighlighter
            style={isDark ? vscDarkPlus : oneLight}
            language={match ? match[1] : 'text'}
            PreTag="div"
            customStyle={{
              margin: 0,
              borderRadius: 0,
              padding: '0.875rem',
              fontSize: '0.8125rem',
              background: isDark ? 'hsl(218 26% 8%)' : 'hsl(216 22% 95%)',
              overflowX: 'auto',
            }}
            codeTagProps={{ style: { fontFamily: "'IBM Plex Mono', ui-monospace, monospace" } }}
          >
            {text.replace(/\n$/, '')}
          </SyntaxHighlighter>
        </div>
      )
    },
  }
}

function MessageContentRenderer({ content }: { content: string | MessageContent[] }) {
  const components = useMarkdownComponents()

  if (Array.isArray(content)) {
    return (
      <div className="space-y-3">
        {content.map((item, index) => (
          <div key={index}>
            {item.type === 'text' && (
              <div className={PROSE_CLASS}>
                <ReactMarkdown components={components}>{item.text || ''}</ReactMarkdown>
              </div>
            )}
            {item.type === 'image_url' && item.image_url && (
              <div className="rounded-lg overflow-hidden border bg-muted/20">
                <img
                  src={item.image_url.url}
                  alt="User uploaded image"
                  className="max-w-full h-auto max-h-96 object-contain"
                  loading="lazy"
                />
              </div>
            )}
          </div>
        ))}
      </div>
    )
  }

  return (
    <div className={PROSE_CLASS}>
      <ReactMarkdown components={components}>{content}</ReactMarkdown>
    </div>
  )
}


export default function Chat() {
  const [input, setInput] = useState('')
  const [selectedModel, setSelectedModel] = useState('gpt-4o')
  const [temperature, setTemperature] = useState(1)
  const [maxTokens, setMaxTokens] = useState(2048)
  const [availableModels, setAvailableModels] = useState<any[]>([])
  const [reasoningEffort] = useState('')
  const [cacheDisabled] = useState(false)
  const [systemPrompt, setSystemPrompt] = useState('You are a helpful assistant.')
  const [guardrailsEnabled, setGuardrailsEnabled] = useState(false)

  const messagesEndRef = useRef<HTMLDivElement>(null)

  const { messages, isLoading, sendMessage, stopGeneration } = useChatMessages({
    selectedModel,
    temperature,
    maxTokens,
    cacheDisabled,
    reasoningEffort: reasoningEffort || undefined,
    systemPrompt,
  })

  useEffect(() => {
    const fetchModels = async () => {
      try {
        const [modelsResponse, routesResponse] = await Promise.all([
          getModels(),
          getRoutes().catch(() => ({ routes: [] })),
        ])

        const validModels = (modelsResponse.data || []).filter((m: any) => m.id && m.id.trim() !== '')

        const routeEntries = (routesResponse.routes || [])
          .filter((r: any) => r.enabled)
          .map((r: any) => ({ id: r.slug, name: `${r.name} (route)`, owned_by: 'route' }))

        const allEntries = [...validModels, ...routeEntries]
        setAvailableModels(allEntries)
        if (allEntries.length > 0) {
          setSelectedModel(allEntries[0].id)
        }
      } catch (err) {
        console.error('Failed to fetch models:', err)
        const fallbackModels = [
          { id: 'gpt-4o', name: 'GPT-4o' },
          { id: 'claude-3.5-sonnet', name: 'Claude 3.5 Sonnet' },
          { id: 'gemini-pro', name: 'Gemini Pro' }
        ]
        setAvailableModels(fallbackModels)
        setSelectedModel(fallbackModels[0].id)
      }
    }

    fetchModels()
  }, [])

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const handleSubmit = async () => {
    if (!input.trim() || isLoading) return
    await sendMessage(input, [])
    setInput('')
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSubmit()
    }
  }

  return (
    <div className="flex h-[calc(100vh-14rem)] min-h-[480px] flex-col gap-4 lg:flex-row">
      {/* LEFT — chat card */}
      <div className="flex min-w-0 flex-1 flex-col rounded-[14px] border border-border bg-card shadow-sm">
        {/* Header: model selector + guardrails chip */}
        <div className="flex items-center justify-between border-b border-border p-4">
          <ModelCombobox
            models={availableModels}
            selectedModel={selectedModel}
            onModelChange={setSelectedModel}
            compact
          />
          {guardrailsEnabled && (
            <StatusChip
              tone="good"
              dot={false}
              label={
                <span className="inline-flex items-center gap-1.5">
                  <Icon icon={icons.shield} className="h-3.5 w-3.5" />
                  1 guardrail active
                </span>
              }
            />
          )}
        </div>

        {/* Messages */}
        <div className="flex-1 space-y-4 overflow-y-auto p-5">
          {messages.length === 0 ? (
            <div className="flex h-full items-center justify-center px-4">
              <div className="max-w-sm text-center">
                <div className="mb-3 inline-flex h-12 w-12 items-center justify-center rounded-lg bg-surface-2">
                  <Icon icon={icons.terminal} className="h-6 w-6 text-text-dim" />
                </div>
                <h2 className="mb-1 text-base font-semibold text-foreground">Test a prompt</h2>
                <p className="text-[13px] text-muted-foreground">
                  Send a message to test your model and configuration.
                </p>
              </div>
            </div>
          ) : (
            messages.map((message) =>
              message.role === 'user' ? (
                <div key={message.id} className="flex flex-row-reverse gap-3">
                  <div className="flex h-[30px] w-[30px] shrink-0 items-center justify-center rounded-lg bg-[var(--accent-soft)] text-primary">
                    <Icon icon={icons.user} className="h-4 w-4" />
                  </div>
                  <div className="max-w-[80%] whitespace-pre-wrap rounded-2xl rounded-tr-md bg-[var(--accent-soft)] px-4 py-3 text-[13.5px] leading-relaxed text-foreground">
                    {Array.isArray(message.content) ? (
                      <MessageContentRenderer content={message.content} />
                    ) : (
                      message.content
                    )}
                  </div>
                </div>
              ) : (
                <div key={message.id} className="flex gap-3">
                  <div className="flex h-[30px] w-[30px] shrink-0 items-center justify-center rounded-lg bg-kind-agent/12 text-kind-agent">
                    <Icon icon={icons.agent} className="h-4 w-4" />
                  </div>
                  <div className="max-w-[80%] rounded-2xl rounded-tl-md bg-surface-2 px-4 py-3 text-[13.5px] leading-relaxed text-foreground">
                    <MessageContentRenderer content={message.content} />
                  </div>
                </div>
              )
            )
          )}

          {isLoading && (
            <div className="flex gap-3">
              <div className="flex h-[30px] w-[30px] shrink-0 items-center justify-center rounded-lg bg-kind-agent/12 text-kind-agent">
                <Icon icon={icons.agent} className="h-4 w-4" />
              </div>
              <div className="flex items-center gap-2 rounded-2xl rounded-tl-md bg-surface-2 px-4 py-3 text-[13.5px] text-muted-foreground">
                <Icon icon={icons.refresh} className="h-3.5 w-3.5 animate-spin" />
                Thinking…
              </div>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>

        {/* Input bar */}
        <div className="border-t border-border p-3">
          <div className="flex items-end gap-2">
            <Textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Send a message to test your configuration…"
              className="max-h-[160px] min-h-[40px] flex-1 resize-none text-[13.5px]"
              disabled={isLoading}
            />
            {isLoading ? (
              <Button variant="destructive" onClick={stopGeneration} className="shrink-0 gap-1.5">
                <Icon icon={icons.stop} className="h-4 w-4" />
                Stop
              </Button>
            ) : (
              <Button
                onClick={handleSubmit}
                disabled={!input.trim()}
                className="shrink-0 gap-1.5 bg-primary text-primary-foreground hover:bg-primary/90"
              >
                Send
                <Icon icon={icons.arrowRight} className="h-4 w-4" />
              </Button>
            )}
          </div>
        </div>
      </div>

      {/* RIGHT — configuration panel */}
      <div className="w-full shrink-0 space-y-5 self-start rounded-[14px] border border-border bg-card p-5 shadow-sm lg:w-[360px] xl:w-[380px]">
        <h2 className="text-base font-semibold text-foreground">Configuration</h2>

        {/* Model */}
        <div className="space-y-2">
          <label className="text-[13px] font-medium text-foreground">Model</label>
          <ModelCombobox
            models={availableModels}
            selectedModel={selectedModel}
            onModelChange={setSelectedModel}
          />
        </div>

        {/* Temperature */}
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <label className="text-[13px] font-medium text-foreground">Temperature</label>
            <span className="font-mono text-[12px] text-text-dim">{temperature.toFixed(1)}</span>
          </div>
          <Slider
            value={[temperature]}
            onValueChange={(value) => setTemperature(value[0])}
            min={0}
            max={1}
            step={0.1}
          />
        </div>

        {/* Max tokens */}
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <label className="text-[13px] font-medium text-foreground">Max tokens</label>
            <span className="font-mono text-[12px] text-text-dim">{maxTokens}</span>
          </div>
          <Slider
            value={[maxTokens]}
            onValueChange={(value) => setMaxTokens(value[0])}
            min={100}
            max={4000}
            step={100}
          />
        </div>

        {/* System prompt */}
        <div className="space-y-2">
          <label className="text-[13px] font-medium text-foreground">System prompt</label>
          <Textarea
            value={systemPrompt}
            onChange={(e) => setSystemPrompt(e.target.value)}
            placeholder="You are a helpful assistant."
            className="min-h-[88px] resize-none text-[13px]"
          />
        </div>

        {/* Guardrails */}
        <div className="flex items-center justify-between">
          <label className="text-[13px] font-medium text-foreground">Guardrails</label>
          <Switch checked={guardrailsEnabled} onCheckedChange={setGuardrailsEnabled} />
        </div>

        {/* Tools */}
        <div className="space-y-2">
          <label className="text-[13px] font-medium text-foreground">Tools</label>
          <div className="flex flex-wrap gap-1.5">
            <span className="rounded-md bg-surface-2 px-2 py-1 text-[12px] text-text-dim">None</span>
          </div>
        </div>
      </div>
    </div>
  )
}
