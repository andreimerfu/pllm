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
import { Card, CardContent } from '../components/ui/card'
import { Avatar, AvatarFallback } from '../components/ui/avatar'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '../components/ui/tooltip'
import { getModels } from '../lib/api'
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

const getProviderInfo = (modelId: string) => {
  const id = modelId?.toLowerCase() || ""
  
  if (id.includes('gpt') || id.includes('openai')) {
    return { icon: 'logos:openai-icon', name: 'OpenAI', color: 'bg-green-500' }
  }
  if (id.includes('claude') || id.includes('anthropic')) {
    return { icon: 'logos:anthropic-icon', name: 'Anthropic', color: 'bg-orange-500' }
  }
  if (id.includes('mixtral') || id.includes('mistral')) {
    return { icon: 'simple-icons:mistral', name: 'Mistral', color: 'bg-red-500' }
  }
  if (id.includes('llama') || id.includes('meta')) {
    return { icon: 'logos:meta', name: 'Meta', color: 'bg-blue-500' }
  }
  if (id.includes('gemini') || id.includes('google')) {
    return { icon: 'logos:google', name: 'Google', color: 'bg-yellow-500' }
  }
  if (id.includes('azure') || id.includes('microsoft')) {
    return { icon: 'logos:microsoft', name: 'Microsoft', color: 'bg-blue-600' }
  }
  if (id.includes('bedrock') || id.includes('aws')) {
    return { icon: 'logos:aws', name: 'AWS', color: 'bg-orange-600' }
  }
  return { icon: 'lucide:cpu', name: 'Local', color: 'bg-gray-500' }
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

  useEffect(() => {
    const fetchModels = async () => {
      try {
        const response = await getModels()
        if (response.data && response.data.length > 0) {
          const modelIds = response.data.map((m: any) => m.id)
          setModels(modelIds)
          if (!settings.model && modelIds.length > 0) {
            setSettings(prev => ({ ...prev, model: modelIds[0] }))
          }
        }
      } catch (err) {
        console.error('Failed to fetch models:', err)
        toast({
          title: "Failed to fetch models",
          description: "Could not load available models. Please check your permissions.",
          variant: "destructive"
        })
      }
    }
    
    fetchModels()
  }, [])

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

    const apiMessages = [
      { role: 'system', content: settings.systemPrompt },
      ...messages.map(m => ({ role: m.role, content: m.content })),
      { role: 'user', content: userMessage.content }
    ]

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

  const provider = getProviderInfo(settings.model)

  return (
    <TooltipProvider>
      <div className="flex h-full bg-background">
        {/* Main Chat Area */}
        <div className="flex-1 flex flex-col">
          {/* Modern Header */}
          <div className="border-b bg-card/95 backdrop-blur supports-[backdrop-filter]:bg-card/95">
            <div className="flex items-center justify-between p-4">
              <div className="flex items-center gap-3">
                <div className="relative">
                  <Avatar className="h-9 w-9">
                    <AvatarFallback className="bg-gradient-to-br from-blue-500 to-purple-600 text-white">
                      <Icon icon="lucide:sparkles" width="18" height="18" />
                    </AvatarFallback>
                  </Avatar>
                  {settings.model && (
                    <div className={cn(
                      "absolute -bottom-1 -right-1 h-4 w-4 rounded-full flex items-center justify-center",
                      provider.color
                    )}>
                      <Icon icon={provider.icon} width="10" height="10" className="text-white" />
                    </div>
                  )}
                </div>
                <div>
                  <h1 className="font-semibold text-lg">Chat Playground</h1>
                  <p className="text-sm text-muted-foreground">
                    {settings.model ? settings.model : 'No model selected'}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-2">
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={handleClear}
                      disabled={messages.length === 0}
                    >
                      <Icon icon="lucide:trash-2" width="16" height="16" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Clear conversation</TooltipContent>
                </Tooltip>
              </div>
            </div>
          </div>

          {/* Messages Area with Modern Layout */}
          <div className="flex-1 relative">
            <ScrollArea className="h-full">
              {messages.length === 0 ? (
                <div className="flex flex-col items-center justify-center h-full p-8">
                  <div className="text-center max-w-md">
                    <div className="h-16 w-16 rounded-full bg-gradient-to-br from-blue-500/10 to-purple-600/10 flex items-center justify-center mx-auto mb-6">
                      <Icon icon="lucide:sparkles" width="32" height="32" className="text-blue-500" />
                    </div>
                    <h2 className="text-xl font-semibold mb-2">Start a conversation</h2>
                    <p className="text-muted-foreground text-sm mb-6">
                      Type a message below to begin chatting with your selected model
                    </p>
                    {!settings.model && (
                      <Badge variant="outline" className="text-xs">
                        Please select a model first
                      </Badge>
                    )}
                  </div>
                </div>
              ) : (
                <div className="px-4 py-6">
                  <div className="max-w-3xl mx-auto space-y-6">
                    {messages.map((message) => (
                      <div
                        key={message.id}
                        className={cn(
                          "flex gap-3 group",
                          message.role === 'user' ? "flex-row-reverse" : "flex-row"
                        )}
                      >
                        {/* Avatar */}
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

                        {/* Message Content */}
                        <div className="flex flex-col min-w-0 max-w-[80%]">
                          {message.role === 'assistant' && message.model && (
                            <div className="flex items-center gap-2 mb-1">
                              <Badge variant="secondary" className="text-xs">
                                {message.model}
                              </Badge>
                            </div>
                          )}
                          <Card className={cn(
                            "shadow-sm",
                            message.role === 'user' 
                              ? "bg-primary text-primary-foreground border-primary/20" 
                              : "bg-card"
                          )}>
                            <CardContent className="p-3">
                              <p className="whitespace-pre-wrap break-words text-sm leading-relaxed">
                                {message.content}
                              </p>
                            </CardContent>
                          </Card>
                          <div className="text-xs text-muted-foreground mt-1 px-1">
                            {message.timestamp.toLocaleTimeString([], { 
                              hour: '2-digit', 
                              minute: '2-digit' 
                            })}
                          </div>
                        </div>
                      </div>
                    ))}
                    
                    {isLoading && (
                      <div className="flex gap-3">
                        <Avatar className="h-8 w-8 shrink-0">
                          <AvatarFallback className="bg-muted">
                            <Icon icon="lucide:bot" width="16" height="16" />
                          </AvatarFallback>
                        </Avatar>
                        <Card className="bg-card shadow-sm">
                          <CardContent className="p-3">
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
              )}
            </ScrollArea>
          </div>

          {/* Modern Input Area */}
          <div className="border-t bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/95 p-4">
            <form onSubmit={handleSubmit} className="max-w-3xl mx-auto">
              <div className="flex gap-3 items-end">
                <div className="flex-1 relative">
                  <Textarea
                    ref={textareaRef}
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder="Type your message..."
                    className="min-h-[60px] max-h-[200px] resize-none pr-12 rounded-xl border-2 border-border/50 focus:border-ring/50"
                    disabled={isLoading}
                  />
                </div>
                <div className="flex gap-2">
                  {isLoading ? (
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          type="button"
                          size="icon"
                          variant="destructive"
                          onClick={handleStop}
                          className="h-12 w-12 rounded-xl"
                        >
                          <Icon icon="lucide:square" width="18" height="18" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>Stop generation</TooltipContent>
                    </Tooltip>
                  ) : (
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          type="submit"
                          size="icon"
                          disabled={!input.trim() || !settings.model}
                          className="h-12 w-12 rounded-xl"
                        >
                          <Icon icon="lucide:send" width="18" height="18" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>Send message</TooltipContent>
                    </Tooltip>
                  )}
                </div>
              </div>
              <div className="mt-2 text-xs text-muted-foreground text-center">
                Press <kbd className="px-1.5 py-0.5 text-xs bg-muted rounded">Enter</kbd> to send, 
                <kbd className="px-1.5 py-0.5 text-xs bg-muted rounded ml-1">Shift+Enter</kbd> for new line
              </div>
            </form>
          </div>
        </div>

        {/* Redesigned Settings Sidebar */}
        <div className="w-80 border-l bg-background">
          <div className="h-full flex flex-col">
            <div className="border-b p-4">
              <h2 className="font-semibold flex items-center gap-2">
                <Icon icon="lucide:settings" width="18" height="18" />
                Configuration
              </h2>
            </div>
            
            <ScrollArea className="flex-1">
              <div className="p-4 space-y-6">
                {/* Model Selection */}
                <div className="space-y-3">
                  <Label className="text-sm font-medium">Model</Label>
                  <Select
                    value={settings.model}
                    onValueChange={(value: string) => setSettings(prev => ({ ...prev, model: value }))}
                  >
                    <SelectTrigger className="h-10">
                      <SelectValue placeholder="Select a model">
                        {settings.model && (
                          <div className="flex items-center gap-2">
                            <Icon icon={getProviderInfo(settings.model).icon} width="16" height="16" />
                            <span className="truncate">{settings.model}</span>
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

                <Separator />

                {/* Parameters */}
                <div className="space-y-4">
                  <h3 className="text-sm font-medium">Parameters</h3>
                  
                  {/* Temperature */}
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <Label className="text-sm">Temperature</Label>
                      <Badge variant="outline" className="text-xs tabular-nums">
                        {settings.temperature}
                      </Badge>
                    </div>
                    <Slider
                      value={[settings.temperature]}
                      onValueChange={([value]) => setSettings(prev => ({ ...prev, temperature: value }))}
                      min={0}
                      max={2}
                      step={0.1}
                      className="w-full"
                    />
                    <p className="text-xs text-muted-foreground">
                      Controls randomness. Lower = focused, higher = creative.
                    </p>
                  </div>

                  {/* Max Tokens */}
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <Label className="text-sm">Max Tokens</Label>
                      <Badge variant="outline" className="text-xs tabular-nums">
                        {settings.maxTokens}
                      </Badge>
                    </div>
                    <Slider
                      value={[settings.maxTokens]}
                      onValueChange={([value]) => setSettings(prev => ({ ...prev, maxTokens: value }))}
                      min={1}
                      max={4096}
                      step={1}
                      className="w-full"
                    />
                  </div>

                  {/* Top P */}
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <Label className="text-sm">Top P</Label>
                      <Badge variant="outline" className="text-xs tabular-nums">
                        {settings.topP}
                      </Badge>
                    </div>
                    <Slider
                      value={[settings.topP]}
                      onValueChange={([value]) => setSettings(prev => ({ ...prev, topP: value }))}
                      min={0}
                      max={1}
                      step={0.01}
                      className="w-full"
                    />
                  </div>

                  {/* Frequency Penalty */}
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <Label className="text-sm">Frequency Penalty</Label>
                      <Badge variant="outline" className="text-xs tabular-nums">
                        {settings.frequencyPenalty}
                      </Badge>
                    </div>
                    <Slider
                      value={[settings.frequencyPenalty]}
                      onValueChange={([value]) => setSettings(prev => ({ ...prev, frequencyPenalty: value }))}
                      min={-2}
                      max={2}
                      step={0.1}
                      className="w-full"
                    />
                  </div>

                  {/* Presence Penalty */}
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <Label className="text-sm">Presence Penalty</Label>
                      <Badge variant="outline" className="text-xs tabular-nums">
                        {settings.presencePenalty}
                      </Badge>
                    </div>
                    <Slider
                      value={[settings.presencePenalty]}
                      onValueChange={([value]) => setSettings(prev => ({ ...prev, presencePenalty: value }))}
                      min={-2}
                      max={2}
                      step={0.1}
                      className="w-full"
                    />
                  </div>
                </div>

                <Separator />

                {/* System Prompt */}
                <div className="space-y-3">
                  <Label className="text-sm font-medium">System Prompt</Label>
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
            </ScrollArea>
          </div>
        </div>
      </div>
    </TooltipProvider>
  )
}