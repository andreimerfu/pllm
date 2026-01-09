import { useState, useRef, useEffect } from 'react';
import { toast } from '@/components/ui/use-toast';

export interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string | MessageContent[];
  timestamp: Date;
  model?: string;
  attachments?: UploadedFile[];
}

export interface MessageContent {
  type: 'text' | 'image_url';
  text?: string;
  image_url?: {
    url: string;
    detail?: string;
  };
}

export interface UploadedFile {
  id: string;
  filename: string;
  size: number;
  type: string;
  url: string;
}

export function useChat(selectedModel: string, temperature: number, maxTokens: number) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const sendMessage = async (
    input: string,
    attachments: UploadedFile[],
    onClear: () => void
  ) => {
    if ((!input.trim() && attachments.length === 0) || isLoading || !selectedModel) return;

    // Create message content in vision format if attachments exist
    let messageContent: string | MessageContent[];
    if (attachments.length > 0) {
      const contentArray: MessageContent[] = [];
      if (input.trim()) {
        contentArray.push({
          type: 'text',
          text: input.trim(),
        });
      }
      // Add attachments (already in base64 format)
      attachments.forEach((attachment) => {
        contentArray.push({
          type: 'image_url',
          image_url: {
            url: attachment.url, // Already base64 data URL
          },
        });
      });
      messageContent = contentArray;
    } else {
      messageContent = input.trim();
    }

    const userMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content: messageContent,
      timestamp: new Date(),
      attachments: attachments.length > 0 ? [...attachments] : undefined,
    };

    setMessages((prev) => [...prev, userMessage]);
    onClear();
    setIsLoading(true);

    try {
      abortControllerRef.current = new AbortController();

      const token = localStorage.getItem('token') || localStorage.getItem('authToken');

      const requestPayload = {
        model: selectedModel,
        messages: [
          { role: 'system', content: 'You are a helpful assistant.' },
          ...messages.map((m) => ({
            role: m.role,
            content: m.content,
          })),
          { role: 'user', content: userMessage.content },
        ],
        temperature: temperature,
        max_tokens: maxTokens,
        stream: true,
      };

      const response = await fetch('/v1/chat/completions', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify(requestPayload),
        signal: abortControllerRef.current.signal,
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const reader = response.body?.getReader();
      if (!reader) throw new Error('No reader available');

      const assistantMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: '',
        timestamp: new Date(),
        model: selectedModel,
      };

      setMessages((prev) => [...prev, assistantMessage]);

      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const data = line.slice(6);
            if (data === '[DONE]') continue;

            try {
              const parsed = JSON.parse(data);
              const content = parsed.choices?.[0]?.delta?.content;
              if (content) {
                assistantMessage.content += content;
                setMessages((prev) => {
                  const newMessages = [...prev];
                  const lastMessage = newMessages[newMessages.length - 1];
                  if (lastMessage.id === assistantMessage.id) {
                    lastMessage.content = assistantMessage.content;
                  }
                  return newMessages;
                });
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
          title: 'Error',
          description: error.message || 'Failed to send message',
          variant: 'destructive',
        });
      }
    } finally {
      setIsLoading(false);
      abortControllerRef.current = null;
    }
  };

  const stopGeneration = () => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      setIsLoading(false);
    }
  };

  const clearMessages = () => {
    setMessages([]);
  };

  return {
    messages,
    isLoading,
    messagesEndRef,
    sendMessage,
    stopGeneration,
    clearMessages,
  };
}
