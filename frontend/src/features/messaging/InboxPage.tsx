import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useAuth } from '../auth/AuthProvider';
import { api, type ConversationResponse, type MessageResponse, type PublicUserResponse } from '../../lib/api/client';

interface ConversationRow {
  conversation: ConversationResponse;
  otherUser: PublicUserResponse | null;
}

export function InboxPage() {
  const { user } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedId = Number(searchParams.get('conversation'));
  const [conversations, setConversations] = useState<ConversationRow[]>([]);
  const [messages, setMessages] = useState<MessageResponse[]>([]);
  const [draft, setDraft] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadConversations = useCallback(async () => {
    if (user === null) { setConversations([]); setLoading(false); return; }
    setLoading(true); setError(null);
    try {
      const result = await api.listConversations();
      const rows = await Promise.all(result.conversations.map(async (conversation): Promise<ConversationRow> => {
        const otherId = conversation.buyer_id === user.id ? conversation.seller_id : conversation.buyer_id;
        const otherUser = await api.getPublicUser(otherId).catch(() => null);
        return { conversation, otherUser };
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
    try { setMessages((await api.listMessages(selectedId)).messages); }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Could not load messages.'); }
  }, [selectedId]);

  useEffect(() => { void loadConversations(); }, [loadConversations]);
  useEffect(() => { void loadMessages(); }, [loadMessages]);

  async function sendMessage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (draft.trim() === '' || selectedId < 1) return;
    try {
      const message = await api.sendMessage(selectedId, draft);
      setMessages((current) => [...current, message]);
      setDraft(''); setError(null);
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'Could not send message.'); }
  }

  if (user === null) return <section className="seller-placeholder page-section"><span className="large-symbol">✉</span><p className="eyebrow">MESSAGES</p><h1>Good deals start with a conversation.</h1><p>Sign in with your wallet to contact sellers and keep your conversations together.</p><Link className="button button-dark" to="/listings">Explore listings</Link></section>;

  const activeConversation = conversations.find(({ conversation }) => conversation.id === selectedId)?.conversation;
  return <section className="page-section inbox-page">
    <div className="section-heading"><div><p className="eyebrow">BUYER & SELLER MESSAGES</p><h1>Inbox</h1></div><button className="button button-outline" type="button" onClick={() => { void loadConversations(); void loadMessages(); }}>Refresh</button></div>
    {error !== null && <p className="form-error" role="alert">{error}</p>}
    {loading ? <p className="loading-state">Loading conversations…</p> : <div className="inbox-layout">
      <aside className="conversation-list" aria-label="Conversations">
        {conversations.length === 0 ? <p className="empty-state">No conversations yet. Start by contacting a seller from a listing.</p> : conversations.map(({ conversation, otherUser }) => {
          const otherId = conversation.buyer_id === user.id ? conversation.seller_id : conversation.buyer_id;
          return <button className={`conversation-row${selectedId === conversation.id ? ' selected' : ''}`} type="button" key={conversation.id} onClick={() => setSearchParams({ conversation: String(conversation.id) })}>
            <span className="seller-avatar">{(otherUser?.name ?? `#${otherId}`).charAt(0)}</span><span><strong>{otherUser?.name ?? `Account #${otherId}`}</strong><small>{conversation.order_id === null ? 'Marketplace conversation' : `Order #${conversation.order_id}`}</small></span>
          </button>;
        })}
      </aside>
      <div className="message-panel">
        {activeConversation ? <>
          <div className="message-panel-heading"><strong>Conversation #{activeConversation.id}</strong>{activeConversation.order_id !== null && <Link to={`/orders?order=${activeConversation.order_id}`}>Order #{activeConversation.order_id}</Link>}</div>
          <div className="message-list">{messages.length === 0 ? <p className="empty-state">No messages yet. Say hello to get the conversation started.</p> : messages.map((message) => <article className={`message-bubble${message.sender_id === user.id ? ' own-message' : ''}`} key={message.id}><p>{message.content}</p><time>{new Date(message.created_at).toLocaleString()}</time></article>)}</div>
          <form className="message-compose" onSubmit={(event) => { void sendMessage(event); }}><textarea value={draft} onChange={(event) => setDraft(event.target.value)} rows={2} maxLength={4000} placeholder="Write a message…" required /><button className="button button-dark" type="submit" disabled={draft.trim() === ''}>Send</button></form>
        </> : <p className="empty-state">Choose a conversation to read and reply.</p>}
      </div>
    </div>}
  </section>;
}
