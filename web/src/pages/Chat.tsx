import React, { useState, useRef, useEffect } from 'react'
import { Icon } from '@iconify/react'
import { Check, ChevronsUpDown } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Badge } from '../components/ui/badge'
import { Textarea } from '../components/ui/textarea'
import { ScrollArea } from '../components/ui/scroll-area'
import { Card, CardContent } from '../components/ui/card'
import { Avatar, AvatarFallback } from '../components/ui/avatar'
import { Switch } from '../components/ui/switch'
import { Slider } from '../components/ui/slider'
import { Separator } from '../components/ui/separator'
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
import { getModels } from '../lib/api'
import { cn } from '../lib/utils'

interface Message {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  timestamp: Date
  model?: string
}

interface Conversation {
  id: string
  title: string
  updatedAt: Date
}

// Mock conversations data
const mockConversations: Conversation[] = [
  { id: '1', title: 'React Component Design', updatedAt: new Date('2025-01-10') },
  { id: '2', title: 'API Integration Help', updatedAt: new Date('2025-01-09') },
  { id: '3', title: 'Database Schema Questions', updatedAt: new Date('2025-01-08') },
  { id: '4', title: 'UI/UX Best Practices', updatedAt: new Date('2025-01-07') },
  { id: '5', title: 'Performance Optimization', updatedAt: new Date('2025-01-06') },
]

// Model icon mapping function
function getModelIcon(modelId: string): string {
  const id = modelId.toLowerCase()
  
  // OpenAI models
  if (id.includes('gpt-4') || id.includes('gpt-3.5') || id.includes('gpt-o1')) {
    return 'simple-icons:openai'
  }
  
  // Anthropic models
  if (id.includes('claude')) {
    return 'simple-icons:anthropic'
  }
  
  // Google models
  if (id.includes('gemini') || id.includes('bard') || id.includes('palm')) {
    return 'simple-icons:google'
  }
  
  // Meta models
  if (id.includes('llama') || id.includes('meta')) {
    return 'simple-icons:meta'
  }
  
  // Mistral models
  if (id.includes('mistral') || id.includes('mixtral')) {
    return 'simple-icons:mistral'
  }
  
  // Cohere models
  if (id.includes('cohere') || id.includes('command')) {
    return 'simple-icons:cohere'
  }
  
  // Perplexity models
  if (id.includes('perplexity') || id.includes('pplx')) {
    return 'simple-icons:perplexity'
  }
  
  // xAI models (Grok)
  if (id.includes('grok')) {
    return 'simple-icons:x'
  }
  
  // Default AI icon
  return 'lucide:bot'
}

// Model Combobox Component
function ModelCombobox({ 
  models, 
  selectedModel, 
  onModelChange 
}: { 
  models: any[]
  selectedModel: string
  onModelChange: (model: string) => void 
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
          className="w-full justify-between"
        >
          <div className="flex items-center gap-2">
            {selectedModelData && (
              <Icon 
                icon={getModelIcon(selectedModelData.id)} 
                className="h-4 w-4" 
              />
            )}
            <span className="truncate">
              {selectedModelData ? (selectedModelData.name || selectedModelData.id) : "Select a model..."}
            </span>
          </div>
          <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[300px] p-0">
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
                    onSelect={(currentValue) => {
                      onModelChange(currentValue === selectedModel ? "" : currentValue)
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
                    <Check
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


function RightSidebar({ 
  selectedModel, 
  setSelectedModel, 
  searchQuery, 
  setSearchQuery,
  temperature,
  setTemperature,
  maxTokens,
  setMaxTokens,
  isCollapsed,
  onToggle,
  availableModels
}: { 
  selectedModel: string
  setSelectedModel: (model: string) => void
  searchQuery: string
  setSearchQuery: (query: string) => void
  temperature: number
  setTemperature: (temp: number) => void
  maxTokens: number
  setMaxTokens: (tokens: number) => void
  isCollapsed: boolean
  onToggle: () => void
  availableModels: any[]
}) {
  const filteredConversations = mockConversations.filter(conv =>
    conv.title.toLowerCase().includes(searchQuery.toLowerCase())
  )

  return (
    <div className={cn(
      "bg-background border-l transition-all duration-200 ease-in-out overflow-hidden",
      "fixed top-16 right-0 bottom-0 z-30",
      isCollapsed ? "w-0 translate-x-full" : "w-80 translate-x-0"
    )}>
      <div className="flex flex-col h-full">
        {/* Header */}
        <div className="p-4 border-b shrink-0">
          <div className="flex items-center justify-between">
            <h2 className="font-semibold">Settings</h2>
            <Button variant="ghost" size="sm" onClick={onToggle} className="md:block lg:hidden">
              <Icon icon="lucide:x" className="h-4 w-4" />
            </Button>
          </div>
        </div>

        <ScrollArea className="flex-1 overflow-hidden">
          <div className="p-4 space-y-6">
            {/* Model Selection */}
            <div>
              <label className="text-sm font-medium mb-3 block">Model</label>
              <ModelCombobox 
                models={availableModels}
                selectedModel={selectedModel}
                onModelChange={setSelectedModel}
              />
            </div>

            {/* Temperature */}
            <div>
              <div className="flex items-center justify-between mb-3">
                <label className="text-sm font-medium">Temperature</label>
                <span className="text-sm text-muted-foreground">{temperature}</span>
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
              <div className="flex items-center justify-between mb-3">
                <label className="text-sm font-medium">Max Tokens</label>
                <span className="text-sm text-muted-foreground">{maxTokens}</span>
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

            <Separator />

            {/* Conversations */}
            <div>
              <div className="flex items-center justify-between mb-3">
                <h3 className="font-medium">Conversations</h3>
                <Button size="sm" variant="ghost">
                  <Icon icon="lucide:plus" className="h-4 w-4" />
                </Button>
              </div>
              
              <div className="relative mb-3">
                <Icon icon="lucide:search" className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <input
                  type="text"
                  placeholder="Search conversations..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="w-full pl-10 pr-4 py-2 border border-input bg-background rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                />
              </div>

              <div className="space-y-1">
                {filteredConversations.length === 0 ? (
                  <div className="text-center text-muted-foreground py-4">
                    <p className="text-sm">No conversations</p>
                  </div>
                ) : (
                  filteredConversations.map((conversation) => (
                    <Button
                      key={conversation.id}
                      variant="ghost"
                      className="w-full justify-start text-left h-auto p-3 hover:bg-accent"
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
            </div>
          </div>
        </ScrollArea>
        
        <div className="p-4 border-t">
          <Button className="w-full gap-2">
            <Icon icon="lucide:plus" className="h-4 w-4" />
            New Chat
          </Button>
        </div>
      </div>
    </div>
  )
}

function EmptyState() {
  return (
    <div className="flex-1 flex items-center justify-center min-h-0 px-4">
      <div className="text-center max-w-md">
        <div className="mb-4">
          <Icon icon="lucide:message-circle" className="h-16 w-16 text-muted-foreground mx-auto" />
        </div>
        <h2 className="text-xl font-semibold mb-2">Start a conversation</h2>
        <p className="text-muted-foreground">
          Type your message below to begin chatting with AI models
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
  onStop 
}: {
  value: string
  onChange: (value: string) => void
  onSubmit: () => void
  disabled: boolean
  onStop: () => void
}) {
  const [webSearchEnabled, setWebSearchEnabled] = useState(true)
  
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      onSubmit()
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
    <div className="border-t bg-background px-4 py-2">
      <div className="max-w-4xl mx-auto">
        <div className="flex gap-2 mb-3">
          <Button variant="outline" size="sm" className="gap-2">
            <Icon icon="lucide:image" className="h-4 w-4" />
            Image
          </Button>
          <Button variant="outline" size="sm" className="gap-2">
            <Icon icon="lucide:code" className="h-4 w-4" />
            Interactive App
          </Button>
          <Button variant="outline" size="sm" className="gap-2">
            <Icon icon="lucide:layout" className="h-4 w-4" />
            Landing Page
          </Button>
          <Button variant="outline" size="sm" className="gap-2">
            <Icon icon="lucide:gamepad-2" className="h-4 w-4" />
            2D Game
          </Button>
          <Button variant="outline" size="sm" className="gap-2">
            <Icon icon="lucide:box" className="h-4 w-4" />
            3D Game
          </Button>
        </div>

        <div className="relative">
          <Textarea
            value={value}
            onChange={(e) => onChange(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Start a new message..."
            className="min-h-[60px] max-h-[200px] resize-none pr-16 border-2 rounded-xl"
            disabled={disabled}
          />
          
          <div className="absolute bottom-3 right-3">
            {disabled ? (
              <Button size="icon" variant="destructive" onClick={onStop}>
                <Icon icon="lucide:square" className="h-4 w-4" />
              </Button>
            ) : (
              <Button 
                size="icon" 
                onClick={onSubmit}
                disabled={!value.trim()}
              >
                <Icon icon="lucide:send" className="h-4 w-4" />
              </Button>
            )}
          </div>
        </div>

        <div className="flex items-center justify-between mt-3">
          <div className="flex items-center gap-4">
            <Button variant="ghost" size="sm" disabled>
              <Icon icon="lucide:paperclip" className="h-4 w-4" />
            </Button>
            <Button variant="ghost" size="sm" disabled>
              <Icon icon="lucide:mic" className="h-4 w-4" />
            </Button>
          </div>
          
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Icon icon="lucide:globe" className="h-4 w-4" />
            <span>Web search</span>
            <Switch 
              checked={webSearchEnabled} 
              onCheckedChange={setWebSearchEnabled}
            />
          </div>
        </div>
      </div>
    </div>
  )
}

export default function Chat() {
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [selectedModel, setSelectedModel] = useState('gpt-4o')
  const [searchQuery, setSearchQuery] = useState('')
  const [temperature, setTemperature] = useState(0.7)
  const [maxTokens, setMaxTokens] = useState(2048)
  const [isSidebarCollapsed, setIsSidebarCollapsed] = useState(true) // Start collapsed on mobile
  const [availableModels, setAvailableModels] = useState<any[]>([])
  
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const abortControllerRef = useRef<AbortController | null>(null)

  useEffect(() => {
    const fetchModels = async () => {
      try {
        const response = await getModels()
        if (response.data && response.data.length > 0) {
          // Filter out models with empty or invalid IDs
          const validModels = response.data.filter((m: any) => m.id && m.id.trim() !== '')
          setAvailableModels(validModels)
          if (validModels.length > 0) {
            setSelectedModel(validModels[0].id)
          }
        }
      } catch (err) {
        console.error('Failed to fetch models:', err)
        // Fallback to some default models if API fails
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

  // Auto-collapse sidebar on desktop on load
  useEffect(() => {
    const handleResize = () => {
      if (window.innerWidth >= 1024) { // lg breakpoint
        setIsSidebarCollapsed(false)
      }
    }
    
    handleResize() // Check on mount
    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [])

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const handleSubmit = async () => {
    if (!input.trim() || isLoading || !selectedModel) return

    const userMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content: input.trim(),
      timestamp: new Date()
    }

    setMessages(prev => [...prev, userMessage])
    setInput('')
    setIsLoading(true)

    try {
      abortControllerRef.current = new AbortController()
      
      const token = localStorage.getItem('token') || localStorage.getItem('authToken');
      
      const response = await fetch('/v1/chat/completions', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
        },
        signal: abortControllerRef.current.signal,
        body: JSON.stringify({
          model: selectedModel,
          messages: [
            { role: 'system', content: 'You are a helpful assistant.' },
            ...messages.map(m => ({ role: m.role, content: m.content })),
            { role: 'user', content: userMessage.content }
          ],
          temperature: temperature,
          max_tokens: maxTokens,
          stream: true
        })
      })

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`)
      }

      const reader = response.body?.getReader()
      const decoder = new TextDecoder()
      
      let assistantMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: '',
        timestamp: new Date(),
        model: selectedModel
      }
      
      setMessages(prev => [...prev, assistantMessage])
      
      while (reader) {
        const { value, done } = await reader.read()
        if (done) break
        
        const chunk = decoder.decode(value)
        const lines = chunk.split('\n')
        
        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const data = line.slice(6)
            if (data === '[DONE]') continue
            
            try {
              const parsed = JSON.parse(data)
              const content = parsed.choices?.[0]?.delta?.content
              if (content) {
                assistantMessage.content += content
                setMessages(prev => {
                  const newMessages = [...prev]
                  const lastMessage = newMessages[newMessages.length - 1]
                  if (lastMessage.id === assistantMessage.id) {
                    lastMessage.content = assistantMessage.content
                  }
                  return newMessages
                })
              }
            } catch (e) {
              // Ignore parsing errors
            }
          }
        }
      }
    } catch (error: any) {
      if (error.name !== 'AbortError') {
        toast({
          title: "Error",
          description: error.message || "Failed to send message",
          variant: "destructive"
        })
      }
    } finally {
      setIsLoading(false)
      abortControllerRef.current = null
    }
  }

  const handleStop = () => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
      setIsLoading(false)
    }
  }

  return (
    <div className="flex h-[calc(100vh-8rem)] overflow-hidden relative">
      {/* Main Chat Area */}
      <div className={cn(
        "flex flex-col flex-1 min-w-0 h-full",
        // Account for sidebar width on larger screens when not collapsed
        !isSidebarCollapsed && "lg:pr-80"
      )}>
        {/* Mobile Sidebar Toggle */}
        <div className="lg:hidden flex items-center justify-between p-4 border-b bg-background shrink-0">
          <h1 className="text-lg font-semibold">Chat</h1>
          <Button variant="ghost" size="sm" onClick={() => setIsSidebarCollapsed(false)}>
            <Icon icon="lucide:settings" className="h-4 w-4 mr-2" />
            Settings
          </Button>
        </div>

        {/* Chat Messages Area - Scrollable */}
        <div className="flex-1 flex flex-col overflow-hidden">
          {messages.length === 0 ? (
            <>
              <EmptyState />
              {/* Message Input - Always at Bottom */}
              <div className="shrink-0 bg-background border-t">
                <MessageInput
                  value={input}
                  onChange={setInput}
                  onSubmit={handleSubmit}
                  disabled={isLoading}
                  onStop={handleStop}
                />
              </div>
            </>
          ) : (
            <>
              <ScrollArea className="h-full pb-40">
                <div className="max-w-4xl mx-auto py-6 px-4 pb-32">
                  <div className="space-y-6">
                    {messages.map((message) => (
                    <div
                      key={message.id}
                      className={cn(
                        "flex gap-4",
                        message.role === 'user' ? "flex-row-reverse" : "flex-row"
                      )}
                    >
                      <Avatar className="h-8 w-8 shrink-0">
                        <AvatarFallback className={cn(
                          message.role === 'user' 
                            ? "bg-primary text-primary-foreground" 
                            : "bg-muted"
                        )}>
                          <Icon 
                            icon={message.role === 'user' ? "lucide:user" : "lucide:bot"} 
                            width="16" 
                            height="16" 
                          />
                        </AvatarFallback>
                      </Avatar>

                      <div className="flex flex-col min-w-0 max-w-[80%]">
                        {message.role === 'assistant' && message.model && (
                          <div className="mb-1">
                            <Badge variant="secondary" className="text-xs">
                              {message.model}
                            </Badge>
                          </div>
                        )}
                        <Card className={cn(
                          "shadow-sm",
                          message.role === 'user' 
                            ? "bg-primary text-primary-foreground" 
                            : "bg-card"
                        )}>
                          <CardContent className="p-4">
                            <p className="whitespace-pre-wrap break-words text-sm leading-relaxed">
                              {message.content}
                            </p>
                          </CardContent>
                        </Card>
                      </div>
                    </div>
                  ))}
                  
                  {isLoading && (
                    <div className="flex gap-4">
                      <Avatar className="h-8 w-8 shrink-0">
                        <AvatarFallback className="bg-muted">
                          <Icon icon="lucide:bot" width="16" height="16" />
                        </AvatarFallback>
                      </Avatar>
                      <Card className="bg-card shadow-sm">
                        <CardContent className="p-4">
                          <div className="flex items-center gap-2">
                            <Icon icon="lucide:loader-2" width="16" height="16" className="animate-spin" />
                            <span className="text-sm text-muted-foreground">Thinking...</span>
                          </div>
                        </CardContent>
                      </Card>
                    </div>
                  )}
                  <div ref={messagesEndRef} />
                </div>
              </div>
            </ScrollArea>
            
            {/* Message Input - Fixed at Bottom when messages exist */}
            <div className="shrink-0 bg-background border-t">
              <MessageInput
                value={input}
                onChange={setInput}
                onSubmit={handleSubmit}
                disabled={isLoading}
                onStop={handleStop}
              />
            </div>
            </>
          )}
        </div>
      </div>


      {/* Mobile Overlay */}
      {!isSidebarCollapsed && (
        <div 
          className="fixed inset-0 bg-black/20 z-20 lg:hidden" 
          onClick={() => setIsSidebarCollapsed(true)}
        />
      )}

      {/* Right Sidebar */}
      <RightSidebar 
        selectedModel={selectedModel}
        setSelectedModel={setSelectedModel}
        searchQuery={searchQuery}
        setSearchQuery={setSearchQuery}
        temperature={temperature}
        setTemperature={setTemperature}
        maxTokens={maxTokens}
        setMaxTokens={setMaxTokens}
        isCollapsed={isSidebarCollapsed}
        onToggle={() => setIsSidebarCollapsed(!isSidebarCollapsed)}
        availableModels={availableModels}
      />
    </div>
  )
}