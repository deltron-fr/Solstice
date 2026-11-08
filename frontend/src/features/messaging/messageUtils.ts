import type { MessageResponse } from '../../lib/api/client';

export function sortMessagesChronologically(messages: MessageResponse[]): MessageResponse[] {
  return [...messages].sort((left, right) => left.id - right.id);
}

export function shouldSendMessageOnEnter(key: string, shiftKey: boolean, isComposing: boolean): boolean {
  return key === 'Enter' && !shiftKey && !isComposing;
}
