import { useCallback, useEffect, useState, type FormEvent, type KeyboardEvent } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useAuth } from '../auth/AuthProvider';
import { api, type ConversationResponse, type MessageResponse, type PublicUserResponse } from '../../lib/api/client';
import { shouldSendMessageOnEnter, sortMessagesChronologically } from './messageUtils';

interface ConversationRow {
  conversation: ConversationResponse;
  otherUser: PublicUserResponse | null;
  orderTitle: string | null;
}

export function InboxPage() {
  const { user } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedId = Number(searchParams.get('conversation'));
  const [conversations, setConversations] = useState<ConversationRow[]>([]);
  const [messages, setMessages] = useState<MessageResponse[]>([]);
  const [draft, setDraft] = useState('');
  const [sending, setSending] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadConversations = useCallback(async () => {
    if (user === null) { setConversations([]); setLoading(false); return; }
    setLoading(true); setError(null);
    try {
      const result = await api.listConversations();
      const rows = await Promise.all(result.conversations.map(async (conversation): Promise<ConversationRow> => {
        const otherId = conversation.buyer_id === user.id ? conversation.seller_id : conversation.buyer_id;
        const [otherUser, orderTitle] = await Promise.all([
          api.getPublicUser(otherId).catch(() => null),
          conversation.order_id === null
            ? Promise.resolve(null)
            : api.getOrder(conversation.order_id)
              .then((order) => api.getListing(order.listing_id))
              .then((listing) => listing.title)
              .catch(() => null),
        ]);
        return { conversation, otherUser, orderTitle };
      }));
      setConversations(rows);
      if (!Number.isSafeInteger(selectedId) || selectedId < 1) {
        const first = rows[0]?.conversation.id;
        if (first !== undefined) setSearchParams({ conversation: String(first) }, { replace: true });
      }
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'Could not load conversations.'); }
    finally { setLoading(false); }
  }, [selectedId, setSearchParams, user]);

  const loadMessages = useCallback(async () => {
    if (!Number.isSafeInteger(selectedId) || selectedId < 1) { setMessages([]); return; }
    try { setMessages(sortMessagesChronologically((await api.listMessages(selectedId)).messages)); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Could not load messages.'); }
  }, [selectedId]);

  useEffect(() => { void loadConversations(); }, [loadConversations]);
  useEffect(() => { void loadMessages(); }, [loadMessages]);

  async function sendMessage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (sending || draft.trim() === '' || !Number.isSafeInteger(selectedId) || selectedId < 1) return;
    setSending(true);
    try {
      const message = await api.sendMessage(selectedId, draft);
      setMessages((current) => sortMessagesChronologically([...current, message]));
      setDraft(''); setError(null);
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'Could not send message.'); }
    finally { setSending(false); }
  }

  function handleComposerKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (!shouldSendMessageOnEnter(event.key, event.shiftKey, event.nativeEvent.isComposing)) return;
    event.preventDefault();
    event.currentTarget.form?.requestSubmit();
  }

  if (user === null) return <section className="seller-placeholder page-section"><span className="large-symbol">✉</span><p className="eyebrow">MESSAGES</p><h1>Good deals start with a conversation.</h1><p>Sign in with your wallet to contact sellers and keep your conversations together.</p><Link className="button button-dark" to="/listings">Explore listings</Link></section>;

  const activeRow = conversations.find(({ conversation }) => conversation.id === selectedId);
  return <section className="page-section inbox-page">
    <div className="section-heading"><div><p className="eyebrow">BUYER & SELLER MESSAGES</p><h1>Inbox</h1></div><button className="button button-outline" type="button" onClick={() => { void loadConversations(); void loadMessages(); }}>Refresh</button></div>
    {error !== null && <p className="form-error" role="alert">{error}</p>}
    {loading ? <p className="loading-state">Loading conversations…</p> : <div className="inbox-layout">
      <aside className="conversation-list" aria-label="Conversations">
        {conversations.length === 0 ? <p className="empty-state">No conversations yet. Start by contacting a seller from a listing.</p> : conversations.map(({ conversation, otherUser, orderTitle }) => {
          const partnerName = otherUser?.name ?? (conversation.buyer_id === user.id ? 'Seller' : 'Buyer');
          return <button className={`conversation-row${selectedId === conversation.id ? ' selected' : ''}`} type="button" key={conversation.id} onClick={() => setSearchParams({ conversation: String(conversation.id) })}>
            <span className="seller-avatar">{partnerName.charAt(0)}</span><span><strong>{partnerName}</strong><small>{conversation.order_id === null ? 'Marketplace chat' : orderTitle ?? 'Order conversation'}</small></span>
          </button>;
        })}
      </aside>
      <div className="message-panel">
        {activeRow ? <>
          <div className="message-panel-heading"><strong>Chat with {activeRow.otherUser?.name ?? (activeRow.conversation.buyer_id === user.id ? 'seller' : 'buyer')}</strong>{activeRow.conversation.order_id !== null && <Link to={`/orders?order=${activeRow.conversation.order_id}`}>View order{activeRow.orderTitle ? ` · ${activeRow.orderTitle}` : ''}</Link>}</div>
          <div className="message-list">{messages.length === 0 ? <p className="empty-state">No messages yet. Say hello to get the conversation started.</p> : messages.map((message) => <article className={`message-bubble${message.sender_id === user.id ? ' own-message' : ''}`} key={message.id}><p>{message.content}</p><time>{new Date(message.created_at).toLocaleString()}</time></article>)}</div>
          <form className="message-compose" onSubmit={(event) => { void sendMessage(event); }}><textarea value={draft} onChange={(event) => setDraft(event.target.value)} onKeyDown={handleComposerKeyDown} rows={2} maxLength={4000} placeholder="Write a message… (Enter to send, Shift+Enter for a new line)" required /><button className="button button-dark" type="submit" disabled={sending || draft.trim() === ''}>{sending ? 'Sending…' : 'Send'}</button></form>
        </> : <p className="empty-state">Choose a conversation to read and reply.</p>}
      </div>
    </div>}
  </section>;
}
