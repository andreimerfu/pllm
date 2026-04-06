import { detectProvider } from './providers';

// Model icon mapping function — delegates to the central detectProvider logic
export function getModelIcon(modelId: string): string {
  return detectProvider(modelId, '').icon;
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
