import { describe, expect, it } from 'vitest';
import type { MessageResponse } from '../../lib/api/client';
import { shouldSendMessageOnEnter, sortMessagesChronologically } from './messageUtils';

const message = (id: number): MessageResponse => ({
  id,
  conversation_id: 1,
  sender_id: 1,
  content: `Message ${id}`,
  created_at: `2026-11-08T12:00:0${id}Z`,
});

describe('message utilities', () => {
  it('orders messages oldest to newest without mutating the input', () => {
    const messages = [message(3), message(1), message(2)];

    expect(sortMessagesChronologically(messages).map(({ id }) => id)).toEqual([1, 2, 3]);
    expect(messages.map(({ id }) => id)).toEqual([3, 1, 2]);
  });

  it('sends on Enter but preserves Shift+Enter and IME composition', () => {
    expect(shouldSendMessageOnEnter('Enter', false, false)).toBe(true);
    expect(shouldSendMessageOnEnter('Enter', true, false)).toBe(false);
    expect(shouldSendMessageOnEnter('Enter', false, true)).toBe(false);
    expect(shouldSendMessageOnEnter('a', false, false)).toBe(false);
  });
});
