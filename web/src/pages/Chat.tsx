import React, { useState, useRef, useEffect } from 'react'
import { Icon } from '@iconify/react'
import { icons } from '@/lib/icons'
import { detectProvider } from '@/lib/providers'
import ReactMarkdown from 'react-markdown'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { vscDarkPlus, oneLight } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { Button } from '../components/ui/button'
import { Textarea } from '../components/ui/textarea'
import { ScrollArea } from '../components/ui/scroll-area'
import { Avatar, AvatarFallback } from '../components/ui/avatar'
import { Switch } from '../components/ui/switch'
import { Slider } from '../components/ui/slider'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select'
import { Separator } from '../components/ui/separator'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '../components/ui/tooltip'
import { toast } from '../components/ui/use-toast'
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
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '../components/ui/sheet'
import { getModels, getRoutes } from '../lib/api'
import { cn } from '../lib/utils'
import { getModelIcon } from '../lib/chat-utils'
import { useChatMessages, type MessageContent, type UploadedFile } from '../hooks/useChatMessages'
import { useChatConversations } from '../hooks/useChatConversations'

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


// Settings Popover Content
function SettingsPopover({
  temperature,
  setTemperature,
  maxTokens,
  setMaxTokens,
  reasoningEffort,
  setReasoningEffort,
  cacheDisabled,
  setCacheDisabled,
}: {
  temperature: number
  setTemperature: (temp: number) => void
  maxTokens: number
  setMaxTokens: (tokens: number) => void
  reasoningEffort: string
  setReasoningEffort: (effort: string) => void
  cacheDisabled: boolean
  setCacheDisabled: (disabled: boolean) => void
}) {
  return (
    <div className="w-72 space-y-4">
      {/* Temperature */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <label className="text-sm font-medium">Temperature</label>
          <span className="text-xs font-mono text-muted-foreground">{temperature}</span>
        </div>
        <Slider
          value={[temperature]}
          onValueChange={(value) => setTemperature(value[0])}
          max={2}
          min={0}
          step={0.1}
          className="w-full"
        />
        <div className="flex justify-between text-xs text-muted-foreground mt-1">
          <span>Precise</span>
          <span>Creative</span>
        </div>
      </div>

      {/* Max Tokens */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <label className="text-sm font-medium">Max Tokens</label>
          <span className="text-xs font-mono text-muted-foreground">{maxTokens}</span>
        </div>
        <Slider
          value={[maxTokens]}
          onValueChange={(value) => setMaxTokens(value[0])}
          max={4000}
          min={100}
          step={100}
          className="w-full"
        />
      </div>

      {/* Reasoning Effort */}
      <div>
        <label className="text-sm font-medium mb-2 block">Reasoning Effort</label>
        <Select value={reasoningEffort} onValueChange={setReasoningEffort}>
          <SelectTrigger className="w-full h-8 text-sm">
            <SelectValue placeholder="Default" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="none">None</SelectItem>
            <SelectItem value="low">Low</SelectItem>
            <SelectItem value="medium">Medium</SelectItem>
            <SelectItem value="high">High</SelectItem>
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-foreground mt-1">For reasoning models (o1, o3, etc.)</p>
      </div>

      <Separator />

      {/* Disable Cache */}
      <div className="flex items-center justify-between">
        <div>
          <label className="text-sm font-medium">Disable Cache</label>
          <p className="text-xs text-muted-foreground">Fresh responses every time</p>
        </div>
        <Switch
          checked={cacheDisabled}
          onCheckedChange={setCacheDisabled}
        />
      </div>
    </div>
  )
}


function MessageContentRenderer({ content }: { content: string | MessageContent[] }) {
  const isDark = document.documentElement.classList.contains('dark')

  if (Array.isArray(content)) {
    return (
      <div className="space-y-3">
        {content.map((item, index) => (
          <div key={index}>
            {item.type === 'text' && (
              <div className="prose prose-sm max-w-none dark:prose-invert">
                <ReactMarkdown
                  components={{
                    code({ className, children }) {
                      const match = /language-(\w+)/.exec(className || '')
                      const language = match ? match[1] : ''
                      const isInline = !match

                      return !isInline ? (
                        <SyntaxHighlighter
                          style={isDark ? vscDarkPlus : oneLight}
                          language={language}
                          PreTag="div"
                          customStyle={{
                            borderRadius: '0.375rem',
                            fontSize: '0.875rem'
                          }}
                        >
                          {String(children).replace(/\n$/, '')}
                        </SyntaxHighlighter>
                      ) : (
                        <code className="bg-muted px-1 py-0.5 rounded text-sm font-mono">
                          {children}
                        </code>
                      )
                    }
                  }}
                >
                  {item.text || ''}
                </ReactMarkdown>
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
    <div className="prose prose-sm max-w-none dark:prose-invert">
      <ReactMarkdown
        components={{
          code({ className, children }) {
            const match = /language-(\w+)/.exec(className || '')
            const language = match ? match[1] : ''
            const isInline = !match

            return !isInline ? (
              <SyntaxHighlighter
                style={isDark ? vscDarkPlus : oneLight}
                language={language}
                PreTag="div"
                customStyle={{
                  borderRadius: '0.375rem',
                  fontSize: '0.875rem'
                }}
              >
                {String(children).replace(/\n$/, '')}
              </SyntaxHighlighter>
            ) : (
              <code className="bg-muted px-1 py-0.5 rounded text-sm font-mono">
                {children}
              </code>
            )
          }
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
}


function EmptyState() {
  return (
    <div className="flex-1 flex items-center justify-center min-h-0 px-4">
      <div className="text-center max-w-sm">
        <div className="mb-3 inline-flex items-center justify-center w-12 h-12 rounded-lg bg-muted">
          <Icon icon={icons.terminal} className="h-6 w-6 text-muted-foreground" />
        </div>
        <h2 className="text-lg font-semibold mb-1">Test a model</h2>
        <p className="text-sm text-muted-foreground">
          Select a model above and send a prompt to test your gateway configuration.
        </p>
      </div>
    </div>
  )
}

function MessageInput({
  value,
  onChange,
  onSubmit,
  disabled,
  onStop,
  attachments,
  onAttachmentAdd,
  onAttachmentRemove
}: {
  value: string
  onChange: (value: string) => void
  onSubmit: () => void
  disabled: boolean
  onStop: () => void
  attachments?: UploadedFile[]
  onAttachmentAdd?: (file: File) => void
  onAttachmentRemove?: (fileId: string) => void
}) {
  const fileInputRef = useRef<HTMLInputElement>(null)

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      onSubmit()
    }
  }

  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files
    if (!files || files.length === 0 || !onAttachmentAdd) return

    const file = files[0]

    if (!file.type.startsWith('image/')) {
      toast({
        title: "Invalid file type",
        description: "Only image files are supported",
        variant: "destructive"
      })
      return
    }

    if (file.size > 10 * 1024 * 1024) {
      toast({
        title: "File too large",
        description: "File size must be less than 10MB",
        variant: "destructive"
      })
      return
    }

    onAttachmentAdd(file)

    toast({
      title: "Image added",
      description: `${file.name} ready to send`
    })

    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
  }

  useEffect(() => {
    const handleSetPrompt = (e: any) => {
      onChange(e.detail)
    }
    window.addEventListener('setPrompt', handleSetPrompt)
    return () => window.removeEventListener('setPrompt', handleSetPrompt)
  }, [onChange])

  return (
    <div className="px-4 py-3">
      <div className="max-w-4xl mx-auto">
        {/* Attachments preview */}
        {attachments && attachments.length > 0 && (
          <div className="flex flex-wrap gap-2 mb-2">
            {attachments.map((file) => (
              <div key={file.id} className="relative group">
                <div className="flex items-center gap-2 bg-muted rounded-md p-1.5 pr-7 text-xs">
                  <Icon icon={icons.image} className="h-3.5 w-3.5" />
                  <span className="truncate max-w-32">{file.filename}</span>
                </div>
                {onAttachmentRemove && (
                  <button
                    onClick={() => onAttachmentRemove(file.id)}
                    className="absolute -top-1 -right-1 bg-destructive text-destructive-foreground rounded-full w-4 h-4 flex items-center justify-center text-[10px] hover:bg-destructive/80"
                    aria-label="Remove attachment"
                  >
                    ×
                  </button>
                )}
              </div>
            ))}
          </div>
        )}

        <div className="relative flex items-end gap-2">
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*"
            onChange={handleFileUpload}
            style={{ display: 'none' }}
          />

          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="shrink-0 h-9 w-9 text-muted-foreground hover:text-foreground"
                  onClick={() => fileInputRef.current?.click()}
                  disabled={disabled}
                >
                  <Icon icon="solar:paperclip-linear" className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                <p>Attach image</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>

          <Textarea
            value={value}
            onChange={(e) => onChange(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Test a prompt..."
            className="min-h-[40px] max-h-[200px] resize-none flex-1 text-sm"
            disabled={disabled}
          />

          <TooltipProvider>
            {disabled ? (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button size="icon" variant="destructive" onClick={onStop} className="shrink-0 h-9 w-9">
                    <Icon icon={icons.stop} className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  <p>Stop generation</p>
                </TooltipContent>
              </Tooltip>
            ) : (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    size="icon"
                    onClick={onSubmit}
                    disabled={!value.trim() && (!attachments || attachments.length === 0)}
                    className="shrink-0 h-9 w-9 bg-teal-600 hover:bg-teal-700 text-white"
                  >
                    <Icon icon={icons.send} className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  <p>Send (Enter)</p>
                </TooltipContent>
              </Tooltip>
            )}
          </TooltipProvider>
        </div>
      </div>
    </div>
  )
}


// Conversations sidebar as a Sheet (slide-over panel)
function ConversationsSheet({
  conversations,
  searchQuery,
  setSearchQuery,
  children,
}: {
  conversations: any[]
  searchQuery: string
  setSearchQuery: (query: string) => void
  children: React.ReactNode
}) {
  return (
    <Sheet>
      <SheetTrigger asChild>
        {children}
      </SheetTrigger>
      <SheetContent side="left" className="w-72 p-0">
        <SheetHeader className="px-4 pt-4 pb-2">
          <SheetTitle className="text-sm font-semibold">Conversations</SheetTitle>
        </SheetHeader>

        <div className="px-4 pb-3">
          <div className="relative">
            <Icon icon={icons.search} className="absolute left-2.5 top-1/2 transform -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
            <input
              type="text"
              placeholder="Search..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-8 pr-3 py-1.5 border border-input bg-background rounded-md text-sm focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
        </div>

        <ScrollArea className="flex-1 h-[calc(100vh-120px)]">
          <div className="px-2 space-y-0.5">
            {conversations.length === 0 ? (
              <div className="text-center text-muted-foreground py-8">
                <p className="text-sm">No conversations</p>
              </div>
            ) : (
              conversations.map((conversation) => (
                <Button
                  key={conversation.id}
                  variant="ghost"
                  className="w-full justify-start text-left h-auto py-2 px-3 hover:bg-accent"
                >
                  <div className="flex flex-col items-start w-full">
                    <span className="font-medium truncate w-full text-sm">{conversation.title}</span>
                    <span className="text-xs text-muted-foreground">
                      {conversation.updatedAt.toLocaleDateString()}
                    </span>
                  </div>
                </Button>
              ))
            )}
          </div>
        </ScrollArea>
      </SheetContent>
    </Sheet>
  )
}


export default function Chat() {
  const [input, setInput] = useState('')
  const [selectedModel, setSelectedModel] = useState('gpt-4o')
  const [temperature, setTemperature] = useState(1)
  const [maxTokens, setMaxTokens] = useState(2048)
  const [availableModels, setAvailableModels] = useState<any[]>([])
  const [currentAttachments, setCurrentAttachments] = useState<UploadedFile[]>([])
  const [reasoningEffort, setReasoningEffort] = useState('')
  const [cacheDisabled, setCacheDisabled] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)

  const messagesEndRef = useRef<HTMLDivElement>(null)

  const { messages, isLoading, sendMessage, stopGeneration, clearMessages } = useChatMessages({
    selectedModel,
    temperature,
    maxTokens,
    cacheDisabled,
    reasoningEffort: reasoningEffort || undefined,
  })

  const { conversations, searchQuery, setSearchQuery } = useChatConversations()

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

  const handleAttachmentAdd = (file: File) => {
    const reader = new FileReader()
    reader.onload = (e) => {
      const base64Data = e.target?.result as string
      const uploadedFile: UploadedFile = {
        id: Date.now().toString(),
        filename: file.name,
        size: file.size,
        type: file.type,
        url: base64Data
      }
      setCurrentAttachments(prev => [...prev, uploadedFile])
    }
    reader.readAsDataURL(file)
  }

  const handleAttachmentRemove = (fileId: string) => {
    setCurrentAttachments(prev => prev.filter(f => f.id !== fileId))
  }

  const handleSubmit = async () => {
    await sendMessage(input, currentAttachments)
    setInput('')
    setCurrentAttachments([])
  }

  const handleStop = () => {
    stopGeneration()
  }

  const handleNewChat = () => {
    clearMessages()
    setInput('')
    setCurrentAttachments([])
  }

  const handleCopyMessage = (content: string | MessageContent[]) => {
    const text = Array.isArray(content)
      ? content.filter(c => c.type === 'text').map(c => c.text).join('\n')
      : content
    navigator.clipboard.writeText(text)
    toast({ title: "Copied to clipboard" })
  }

  return (
    <div className="flex flex-col h-[calc(100vh-8rem)] overflow-hidden">
      {/* Compact Header Bar */}
      <div className="flex items-center justify-between px-4 py-2 border-b bg-background shrink-0 gap-3">
        {/* Left: Conversations + Title */}
        <div className="flex items-center gap-2 shrink-0">
          <ConversationsSheet
            conversations={conversations}
            searchQuery={searchQuery}
            setSearchQuery={setSearchQuery}
          >
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <Icon icon={icons.clock} className="h-4 w-4" />
            </Button>
          </ConversationsSheet>
          <span className="text-sm font-semibold text-muted-foreground hidden sm:block">Chat</span>
        </div>

        {/* Center: Model Selector */}
        <div className="flex-1 flex justify-center max-w-md">
          <ModelCombobox
            models={availableModels}
            selectedModel={selectedModel}
            onModelChange={setSelectedModel}
            compact
          />
        </div>

        {/* Right: Settings, Clear, New */}
        <div className="flex items-center gap-1 shrink-0">
          <Popover open={settingsOpen} onOpenChange={setSettingsOpen}>
            <PopoverTrigger asChild>
              <Button variant="ghost" size="icon" className="h-8 w-8">
                <Icon icon={icons.settings} className="h-4 w-4" />
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" className="w-auto">
              <SettingsPopover
                temperature={temperature}
                setTemperature={setTemperature}
                maxTokens={maxTokens}
                setMaxTokens={setMaxTokens}
                reasoningEffort={reasoningEffort}
                setReasoningEffort={setReasoningEffort}
                cacheDisabled={cacheDisabled}
                setCacheDisabled={setCacheDisabled}
              />
            </PopoverContent>
          </Popover>

          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  onClick={handleNewChat}
                  disabled={messages.length === 0}
                >
                  <Icon icon={icons.delete} className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent><p>Clear chat</p></TooltipContent>
            </Tooltip>
          </TooltipProvider>

          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  onClick={handleNewChat}
                >
                  <Icon icon={icons.plus} className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent><p>New chat</p></TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>
      </div>

      {/* Message Area */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {messages.length === 0 ? (
          <>
            <EmptyState />
            <div className="shrink-0 border-t bg-background">
              <MessageInput
                value={input}
                onChange={setInput}
                onSubmit={handleSubmit}
                disabled={isLoading}
                onStop={handleStop}
                attachments={currentAttachments}
                onAttachmentAdd={handleAttachmentAdd}
                onAttachmentRemove={handleAttachmentRemove}
              />
            </div>
          </>
        ) : (
          <>
            <ScrollArea className="h-full">
              <div className="max-w-4xl mx-auto py-4 px-4 pb-8">
                <div className="space-y-4">
                  {messages.map((message) => (
                    <div
                      key={message.id}
                      className={cn(
                        "flex gap-3",
                        message.role === 'user' ? "flex-row-reverse" : "flex-row"
                      )}
                    >
                      <Avatar className="h-7 w-7 shrink-0">
                        <AvatarFallback className={cn(
                          "text-xs",
                          message.role === 'user'
                            ? "bg-teal-600 text-white"
                            : "bg-muted"
                        )}>
                          <Icon
                            icon={message.role === 'user'
                              ? icons.user
                              : (message.model ? getModelIcon(message.model) : icons.chat)
                            }
                            width="14"
                            height="14"
                          />
                        </AvatarFallback>
                      </Avatar>

                      <div className={cn(
                        "flex flex-col min-w-0 max-w-[80%]",
                        message.role === 'user' ? "items-end" : "items-start"
                      )}>
                        {message.role === 'assistant' && message.model && (
                          <div className="mb-1">
                            <span className="text-xs font-mono text-muted-foreground">{message.model}</span>
                          </div>
                        )}

                        <div className={cn(
                          "rounded-lg px-3.5 py-2.5",
                          message.role === 'user'
                            ? "bg-teal-600 text-white"
                            : "bg-card border shadow-sm"
                        )}>
                          {message.role === 'user' ? (
                            Array.isArray(message.content) ? (
                              <MessageContentRenderer content={message.content} />
                            ) : (
                              <p className="whitespace-pre-wrap break-words text-sm leading-relaxed">
                                {message.content}
                              </p>
                            )
                          ) : (
                            <MessageContentRenderer content={message.content} />
                          )}
                        </div>

                        {/* Assistant message footer with metadata */}
                        {message.role === 'assistant' && (
                          <div className="mt-1 flex items-center gap-3 text-[10px] text-muted-foreground px-1">
                            <span className="font-mono">
                              {message.timestamp.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                            </span>
                            <TooltipProvider>
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <button
                                    onClick={() => handleCopyMessage(message.content)}
                                    className="hover:text-foreground transition-colors"
                                  >
                                    <Icon icon={icons.copy} className="h-3 w-3" />
                                  </button>
                                </TooltipTrigger>
                                <TooltipContent><p>Copy response</p></TooltipContent>
                              </Tooltip>
                            </TooltipProvider>
                          </div>
                        )}
                      </div>
                    </div>
                  ))}

                  {isLoading && (
                    <div className="flex gap-3">
                      <Avatar className="h-7 w-7 shrink-0">
                        <AvatarFallback className="bg-muted text-xs">
                          <Icon icon={selectedModel ? getModelIcon(selectedModel) : icons.chat} width="14" height="14" />
                        </AvatarFallback>
                      </Avatar>
                      <div className="rounded-lg px-3.5 py-2.5 bg-card border shadow-sm">
                        <div className="flex items-center gap-2">
                          <Icon icon={icons.refresh} width="14" height="14" className="animate-spin" />
                          <span className="text-sm text-muted-foreground">Thinking...</span>
                        </div>
                      </div>
                    </div>
                  )}
                  <div ref={messagesEndRef} />
                </div>
              </div>
            </ScrollArea>

            {/* Input at bottom */}
            <div className="shrink-0 border-t bg-background">
              <MessageInput
                value={input}
                onChange={setInput}
                onSubmit={handleSubmit}
                disabled={isLoading}
                onStop={handleStop}
                attachments={currentAttachments}
                onAttachmentAdd={handleAttachmentAdd}
                onAttachmentRemove={handleAttachmentRemove}
              />
            </div>
          </>
        )}
      </div>
    </div>
  )
}
