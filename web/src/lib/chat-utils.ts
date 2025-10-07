// Model icon mapping function
export function getModelIcon(modelId: string): string {
  const id = modelId.toLowerCase();

  // OpenAI models
  if (id.includes('gpt-4') || id.includes('gpt-3.5') || id.includes('gpt-o1')) {
    return 'simple-icons:openai';
  }

  // Anthropic models
  if (id.includes('claude')) {
    return 'simple-icons:anthropic';
  }

  // Google models
  if (id.includes('gemini') || id.includes('bard') || id.includes('palm')) {
    return 'simple-icons:google';
  }

  // Meta models
  if (id.includes('llama') || id.includes('meta')) {
    return 'simple-icons:meta';
  }

  // Mistral models
  if (id.includes('mistral') || id.includes('mixtral')) {
    return 'simple-icons:mistral';
  }

  // Cohere models
  if (id.includes('cohere') || id.includes('command')) {
    return 'simple-icons:cohere';
  }

  // Default icon for unknown models
  return 'lucide:bot';
}

// Mock conversations data
export interface Conversation {
  id: string;
  title: string;
  updatedAt: Date;
}

export const mockConversations: Conversation[] = [
  { id: '1', title: 'React Component Design', updatedAt: new Date('2025-01-10') },
  { id: '2', title: 'API Integration Help', updatedAt: new Date('2025-01-09') },
  { id: '3', title: 'Database Schema Questions', updatedAt: new Date('2025-01-08') },
  { id: '4', title: 'UI/UX Best Practices', updatedAt: new Date('2025-01-07') },
  { id: '5', title: 'Performance Optimization', updatedAt: new Date('2025-01-06') },
];
