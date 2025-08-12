import React, { useState, useRef, useEffect } from 'react'
import { Icon } from '@iconify/react'
import { Button } from '../components/ui/button'
import { Label } from '../components/ui/label'
import { Slider } from '../components/ui/slider'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select'
import { Textarea } from '../components/ui/textarea'
import { ScrollArea } from '../components/ui/scroll-area'
import { Separator } from '../components/ui/separator'
import { Badge } from '../components/ui/badge'
import { toast } from '../components/ui/use-toast'
import { cn } from '../lib/utils'

interface Message {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  timestamp: Date
  model?: string
}

interface ChatSettings {
  model: string
  temperature: number
  maxTokens: number
  topP: number
  frequencyPenalty: number
  presencePenalty: number
  systemPrompt: string
}

// Helper function to get provider info from model ID
const getProviderInfo = (modelId: string) => {
  const id = modelId.toLowerCase()
  
  if (id.includes('gpt') || id.includes('openai')) {
    return { icon: 'logos:openai-icon', name: 'OpenAI' }
  }
  if (id.includes('claude') || id.includes('anthropic')) {
    return { icon: 'logos:anthropic-icon', name: 'Anthropic' }
  }
  if (id.includes('mixtral') || id.includes('mistral')) {
    return { icon: 'simple-icons:mistral', name: 'Mistral' }
  }
  if (id.includes('llama') || id.includes('meta')) {
    return { icon: 'logos:meta', name: 'Meta' }
  }
  if (id.includes('gemini') || id.includes('google')) {
    return { icon: 'logos:google', name: 'Google' }
  }
  if (id.includes('azure') || id.includes('microsoft')) {
    return { icon: 'logos:microsoft', name: 'Microsoft' }
  }
  if (id.includes('bedrock') || id.includes('aws')) {
    return { icon: 'logos:aws', name: 'AWS' }
  }
  return { icon: 'lucide:cpu', name: 'Local' }
}

export default function Chat() {
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [models, setModels] = useState<string[]>([])
  const [settings, setSettings] = useState<ChatSettings>({
    model: '',
    temperature: 0.7,
    maxTokens: 2048,
    topP: 1,
    frequencyPenalty: 0,
    presencePenalty: 0,
    systemPrompt: 'You are a helpful assistant.'
  })
  
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const abortControllerRef = useRef<AbortController | null>(null)

  // Fetch available models
  useEffect(() => {
    fetch('/v1/models')
      .then(res => res.json())
      .then(data => {
        if (data.data && data.data.length > 0) {
          const modelIds = data.data.map((m: any) => m.id)
          setModels(modelIds)
          if (!settings.model && modelIds.length > 0) {
            setSettings(prev => ({ ...prev, model: modelIds[0] }))
          }
        }
      })
      .catch(err => console.error('Failed to fetch models:', err))
  }, [])

  // Auto-scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const handleSubmit = async (e?: React.FormEvent) => {
    e?.preventDefault()
    
    if (!input.trim() || isLoading) return
    if (!settings.model) {
      toast({
        title: "No model selected",
        description: "Please select a model from the settings panel",
        variant: "destructive"
      })
      return
    }

    const userMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content: input.trim(),
      timestamp: new Date()
    }

    setMessages(prev => [...prev, userMessage])
    setInput('')
    setIsLoading(true)

    // Prepare messages for API
    const apiMessages = [
      { role: 'system', content: settings.systemPrompt },
      ...messages.map(m => ({ role: m.role, content: m.content })),
      { role: 'user', content: userMessage.content }
    ]

    try {
      abortControllerRef.current = new AbortController()
      
      const response = await fetch('/v1/chat/completions', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        signal: abortControllerRef.current.signal,
        body: JSON.stringify({
          model: settings.model,
          messages: apiMessages,
          temperature: settings.temperature,
          max_tokens: settings.maxTokens,
          top_p: settings.topP,
          frequency_penalty: settings.frequencyPenalty,
          presence_penalty: settings.presencePenalty,
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
        model: settings.model
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
        console.error('Chat error:', error)
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

  const handleClear = () => {
    setMessages([])
    setInput('')
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSubmit()
    }
  }

  return (
    <div className="flex h-full">
      {/* Main Chat Area */}
      <div className="flex-1 flex flex-col">
        {/* Header */}
        <div className="border-b px-6 py-4 bg-card">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="h-10 w-10 rounded-lg bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center">
                <Icon icon="lucide:messages-square" width="20" height="20" className="text-white" />
              </div>
              <div>
                <h1 className="text-xl font-bold">Chat Playground</h1>
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  {settings.model ? (
                    <>
                      <Icon icon={getProviderInfo(settings.model).icon} width="14" height="14" />
                      <span>{settings.model}</span>
                    </>
                  ) : (
                    'No model selected'
                  )}
                </div>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={handleClear}
                disabled={messages.length === 0}
              >
                <Icon icon="lucide:trash-2" width="16" height="16" className="mr-2" />
                Clear
              </Button>
            </div>
          </div>
        </div>

        {/* Messages Area */}
        <ScrollArea className="flex-1 p-6">
          {messages.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-center">
              <div className="h-16 w-16 rounded-2xl bg-gradient-to-br from-blue-500/10 to-purple-600/10 flex items-center justify-center mb-4">
                <Icon icon="lucide:sparkles" width="32" height="32" className="text-blue-500" />
              </div>
              <h2 className="text-lg font-semibold mb-2">Start a conversation</h2>
              <p className="text-sm text-muted-foreground max-w-md">
                Type a message below to begin chatting with the selected model
              </p>
            </div>
          ) : (
            <div className="space-y-4 max-w-4xl mx-auto">
              {messages.map((message) => (
                <div
                  key={message.id}
                  className={cn(
                    "flex gap-3",
                    message.role === 'user' ? "justify-end" : "justify-start"
                  )}
                >
                  {message.role === 'assistant' && (
                    <div className="flex-shrink-0">
                      <div className="h-8 w-8 rounded-lg bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center">
                        <Icon icon="lucide:bot" width="16" height="16" className="text-white" />
                      </div>
                    </div>
                  )}
                  
                  <div
                    className={cn(
                      "group relative max-w-[70%] rounded-lg px-4 py-3",
                      message.role === 'user' 
                        ? "bg-primary text-primary-foreground" 
                        : "bg-muted"
                    )}
                  >
                    {message.role === 'assistant' && message.model && (
                      <div className="flex items-center gap-2 mb-1">
                        <Badge variant="secondary" className="text-xs">
                          {message.model}
                        </Badge>
                      </div>
                    )}
                    <div className="text-sm whitespace-pre-wrap break-words">
                      {message.content}
                    </div>
                    <div className="text-xs mt-1 opacity-50">
                      {message.timestamp.toLocaleTimeString()}
                    </div>
                  </div>
                  
                  {message.role === 'user' && (
                    <div className="flex-shrink-0">
                      <div className="h-8 w-8 rounded-lg bg-primary flex items-center justify-center">
                        <Icon icon="lucide:user" width="16" height="16" className="text-primary-foreground" />
                      </div>
                    </div>
                  )}
                </div>
              ))}
              {isLoading && (
                <div className="flex gap-3">
                  <div className="flex-shrink-0">
                    <div className="h-8 w-8 rounded-lg bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center">
                      <Icon icon="lucide:bot" width="16" height="16" className="text-white" />
                    </div>
                  </div>
                  <div className="bg-muted rounded-lg px-4 py-3">
                    <div className="flex items-center gap-2">
                      <Icon icon="lucide:loader-2" width="16" height="16" className="animate-spin" />
                      <span className="text-sm">Thinking...</span>
                    </div>
                  </div>
                </div>
              )}
              <div ref={messagesEndRef} />
            </div>
          )}
        </ScrollArea>

        {/* Input Area */}
        <div className="border-t p-4 bg-card">
          <form onSubmit={handleSubmit} className="max-w-4xl mx-auto">
            <div className="flex gap-2">
              <Textarea
                ref={textareaRef}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="Type your message..."
                className="min-h-[60px] max-h-[200px] resize-none"
                disabled={isLoading}
              />
              <div className="flex flex-col gap-2">
                {isLoading ? (
                  <Button
                    type="button"
                    size="icon"
                    variant="destructive"
                    onClick={handleStop}
                  >
                    <Icon icon="lucide:square" width="16" height="16" />
                  </Button>
                ) : (
                  <Button
                    type="submit"
                    size="icon"
                    disabled={!input.trim() || !settings.model}
                  >
                    <Icon icon="lucide:send" width="16" height="16" />
                  </Button>
                )}
              </div>
            </div>
            <div className="mt-2 text-xs text-muted-foreground">
              Press Enter to send, Shift+Enter for new line
            </div>
          </form>
        </div>
      </div>

      {/* Settings Sidebar */}
      <div className="w-80 border-l bg-card overflow-y-auto">
        <div className="p-6 space-y-6">
          <div>
            <h2 className="text-lg font-semibold mb-4">Chat Settings</h2>
          </div>

          <Separator />

          {/* Model Selection */}
          <div className="space-y-2">
            <Label>Model</Label>
            <Select
              value={settings.model}
              onValueChange={(value: string) => setSettings(prev => ({ ...prev, model: value }))}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select a model">
                  {settings.model && (
                    <div className="flex items-center gap-2">
                      <Icon icon={getProviderInfo(settings.model).icon} width="16" height="16" />
                      <span>{settings.model}</span>
                    </div>
                  )}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                {models.map(model => {
                  const provider = getProviderInfo(model)
                  return (
                    <SelectItem key={model} value={model}>
                      <div className="flex items-center gap-2">
                        <Icon icon={provider.icon} width="16" height="16" />
                        <span>{model}</span>
                      </div>
                    </SelectItem>
                  )
                })}
              </SelectContent>
            </Select>
          </div>

          {/* Temperature */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label>Temperature</Label>
              <span className="text-sm text-muted-foreground">{settings.temperature}</span>
            </div>
            <Slider
              value={[settings.temperature]}
              onValueChange={([value]) => setSettings(prev => ({ ...prev, temperature: value }))}
              min={0}
              max={2}
              step={0.1}
            />
            <p className="text-xs text-muted-foreground">
              Controls randomness. Lower is more focused, higher is more creative.
            </p>
          </div>

          {/* Max Tokens */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label>Max Tokens</Label>
              <span className="text-sm text-muted-foreground">{settings.maxTokens}</span>
            </div>
            <Slider
              value={[settings.maxTokens]}
              onValueChange={([value]) => setSettings(prev => ({ ...prev, maxTokens: value }))}
              min={1}
              max={4096}
              step={1}
            />
            <p className="text-xs text-muted-foreground">
              Maximum length of the response.
            </p>
          </div>

          {/* Top P */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label>Top P</Label>
              <span className="text-sm text-muted-foreground">{settings.topP}</span>
            </div>
            <Slider
              value={[settings.topP]}
              onValueChange={([value]) => setSettings(prev => ({ ...prev, topP: value }))}
              min={0}
              max={1}
              step={0.01}
            />
            <p className="text-xs text-muted-foreground">
              Nucleus sampling threshold.
            </p>
          </div>

          {/* Frequency Penalty */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label>Frequency Penalty</Label>
              <span className="text-sm text-muted-foreground">{settings.frequencyPenalty}</span>
            </div>
            <Slider
              value={[settings.frequencyPenalty]}
              onValueChange={([value]) => setSettings(prev => ({ ...prev, frequencyPenalty: value }))}
              min={-2}
              max={2}
              step={0.1}
            />
            <p className="text-xs text-muted-foreground">
              Reduces repetition of token sequences.
            </p>
          </div>

          {/* Presence Penalty */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label>Presence Penalty</Label>
              <span className="text-sm text-muted-foreground">{settings.presencePenalty}</span>
            </div>
            <Slider
              value={[settings.presencePenalty]}
              onValueChange={([value]) => setSettings(prev => ({ ...prev, presencePenalty: value }))}
              min={-2}
              max={2}
              step={0.1}
            />
            <p className="text-xs text-muted-foreground">
              Increases likelihood of new topics.
            </p>
          </div>

          <Separator />

          {/* System Prompt */}
          <div className="space-y-2">
            <Label>System Prompt</Label>
            <Textarea
              value={settings.systemPrompt}
              onChange={(e) => setSettings(prev => ({ ...prev, systemPrompt: e.target.value }))}
              placeholder="You are a helpful assistant..."
              className="min-h-[100px] resize-none text-sm"
            />
            <p className="text-xs text-muted-foreground">
              Sets the behavior and context for the assistant.
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}