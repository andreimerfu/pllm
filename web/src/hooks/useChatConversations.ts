import { useState, useMemo } from 'react'
import { mockConversations, Conversation } from '@/lib/chat-utils'

export function useChatConversations() {
  const [conversations] = useState<Conversation[]>(mockConversations)
  const [searchQuery, setSearchQuery] = useState('')

  const filteredConversations = useMemo(() => {
    if (!searchQuery) return conversations

    return conversations.filter(conv =>
      conv.title.toLowerCase().includes(searchQuery.toLowerCase())
    )
  }, [conversations, searchQuery])

  const getConversationById = (id: string) => {
    return conversations.find(conv => conv.id === id)
  }

  const createConversation = (title: string): Conversation => {
    const newConversation: Conversation = {
      id: Date.now().toString(),
      title,
      updatedAt: new Date()
    }
    return newConversation
  }

  const updateConversation = (id: string, updates: Partial<Conversation>) => {
    // In a real app, this would update the backend
    console.log('Update conversation:', id, updates)
  }

  const deleteConversation = (id: string) => {
    // In a real app, this would delete from backend
    console.log('Delete conversation:', id)
  }

  return {
    conversations: filteredConversations,
    allConversations: conversations,
    searchQuery,
    setSearchQuery,
    getConversationById,
    createConversation,
    updateConversation,
    deleteConversation,
  }
}
