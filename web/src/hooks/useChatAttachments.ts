import { useState } from 'react';
import { UploadedFile } from './useChat';

export function useChatAttachments() {
  const [currentAttachments, setCurrentAttachments] = useState<UploadedFile[]>([]);

  const addAttachment = (file: File) => {
    // Convert file to base64 immediately and store locally
    const reader = new FileReader();
    reader.onload = (e) => {
      const base64Data = e.target?.result as string;
      const uploadedFile: UploadedFile = {
        id: Date.now().toString(),
        filename: file.name,
        size: file.size,
        type: file.type,
        url: base64Data, // Store base64 directly instead of server URL
      };
      setCurrentAttachments((prev) => [...prev, uploadedFile]);
    };
    reader.readAsDataURL(file);
  };

  const removeAttachment = (fileId: string) => {
    setCurrentAttachments((prev) => prev.filter((f) => f.id !== fileId));
  };

  const clearAttachments = () => {
    setCurrentAttachments([]);
  };

  return {
    currentAttachments,
    addAttachment,
    removeAttachment,
    clearAttachments,
  };
}
