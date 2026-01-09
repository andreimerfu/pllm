import { useState, useEffect } from 'react';
import { getModels } from '@/lib/api';

export function useChatModels() {
  const [availableModels, setAvailableModels] = useState<any[]>([]);
  const [selectedModel, setSelectedModel] = useState('gpt-4o');
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const fetchModels = async () => {
      try {
        setIsLoading(true);
        const response = await getModels();
        if (response.data && response.data.length > 0) {
          // Filter out models with empty or invalid IDs
          const validModels = response.data.filter((m: any) => m.id && m.id.trim() !== '');
          setAvailableModels(validModels);
          if (validModels.length > 0) {
            setSelectedModel(validModels[0].id);
          }
        }
      } catch (err) {
        console.error('Failed to fetch models:', err);
        // Fallback to some default models if API fails
        const fallbackModels = [
          { id: 'gpt-4o', name: 'GPT-4o' },
          { id: 'claude-3.5-sonnet', name: 'Claude 3.5 Sonnet' },
          { id: 'gemini-pro', name: 'Gemini Pro' },
        ];
        setAvailableModels(fallbackModels);
        setSelectedModel(fallbackModels[0].id);
      } finally {
        setIsLoading(false);
      }
    };

    fetchModels();
  }, []);

  return {
    availableModels,
    selectedModel,
    setSelectedModel,
    isLoading,
  };
}
