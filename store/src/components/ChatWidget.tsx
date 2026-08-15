import { useEffect, useRef, useState } from 'react'
import { streamChat } from '../api/chat'

type ChatMessage = {
  id: string
  role: 'user' | 'assistant' | 'status'
  agentId?: string
  text: string
}

const AGENT_LABEL: Record<string, string> = {
  potato: 'Concierge',
  shop: 'Shop',
  billing: 'Billing',
  support: 'Support',
}

function friendlyError(text: string) {
  if (/failed to find agent/i.test(text)) {
    return 'I can help with tees, orders, or shipping. What do you need?'
  }
  return text
}

export function ChatWidget() {
  const [open, setOpen] = useState(false)
  const [input, setInput] = useState('')
  const [busy, setBusy] = useState(false)
  const [messages, setMessages] = useState<ChatMessage[]>([
    {
      id: 'welcome',
      role: 'assistant',
      agentId: 'potato',
      text: 'Hey — I can help with tees, orders, or shipping. What do you need?',
    },
  ])
  const bottom = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottom.current?.scrollIntoView({ block: 'end' })
  }, [messages, open])

  async function send() {
    const text = input.trim()
    if (!text || busy) return
    setInput('')
    setBusy(true)
    const userId = crypto.randomUUID()
    const assistantId = crypto.randomUUID()
    setMessages((current) => [
      ...current,
      { id: userId, role: 'user', text },
      { id: assistantId, role: 'assistant', agentId: 'potato', text: '' },
    ])

    try {
      await streamChat(text, (chunk) => {
        if (chunk.session_id) {
          sessionStorage.setItem('potato-chat-session', chunk.session_id)
        }
        if (chunk.error) {
          const errorText = friendlyError(chunk.error)
          setMessages((current) =>
            current.map((item) =>
              item.id === assistantId ? { ...item, text: errorText || item.text } : item,
            ),
          )
          return
        }
        if (chunk.event === 'handoff' || chunk.event === 'tool' || chunk.event === 'hitl') {
          const statusText = chunk.delta?.trim()
          if (!statusText || /transfer_to_agent|routing you/i.test(statusText)) return
          const statusMsg: ChatMessage = {
            id: crypto.randomUUID(),
            role: 'status',
            text: statusText,
          }
          setMessages((current) => {
            const assistantIndex = current.findIndex((item) => item.id === assistantId)
            if (assistantIndex === -1) return [...current, statusMsg]
            return [
              ...current.slice(0, assistantIndex),
              statusMsg,
              ...current.slice(assistantIndex),
            ]
          })
          return
        }
        if (!chunk.delta) return
        if (/^(shop|billing|support|potato|transfer_to_agent)$/i.test(chunk.delta.trim())) return
        setMessages((current) =>
          current.map((item) =>
            item.id === assistantId
              ? {
                  ...item,
                  agentId: AGENT_LABEL[chunk.agent_id] ? chunk.agent_id : item.agentId,
                  text: item.text + chunk.delta,
                }
              : item,
          ),
        )
      })
    } catch (err) {
      setMessages((current) =>
        current.map((item) =>
          item.id === assistantId
            ? { ...item, text: err instanceof Error ? err.message : String(err) }
            : item,
        ),
      )
    } finally {
      setBusy(false)
      setMessages((current) => current.filter((item) => item.role !== 'assistant' || item.text.trim()))
    }
  }

  return (
    <div className="fixed bottom-5 right-5 z-40 flex flex-col items-end gap-3">
      {open ? (
        <div className="flex h-[min(32rem,70svh)] w-[min(22rem,calc(100vw-2rem))] flex-col border border-line bg-ink shadow-2xl">
          <div className="flex items-center justify-between border-b border-line px-4 py-3">
            <div>
              <p className="font-display text-2xl tracking-[0.08em]">Ask Potato</p>
              <p className="text-[10px] uppercase tracking-[0.18em] text-mute">
                Shop · Billing · Support
              </p>
            </div>
            <button
              type="button"
              className="text-xs uppercase tracking-[0.16em] text-mute hover:text-accent"
              onClick={() => setOpen(false)}
            >
              Close
            </button>
          </div>
          <div className="flex-1 space-y-3 overflow-y-auto px-4 py-3 text-sm">
            {messages.map((item) => (
              <div key={item.id} className={item.role === 'user' ? 'text-right' : 'text-left'}>
                {item.role === 'assistant' && item.agentId && AGENT_LABEL[item.agentId] ? (
                  <p className="mb-1 text-[10px] uppercase tracking-[0.16em] text-accent">
                    {AGENT_LABEL[item.agentId]}
                  </p>
                ) : null}
                <p
                  className={
                    item.role === 'user'
                      ? 'inline-block whitespace-pre-wrap bg-paper px-3 py-2 text-ink'
                      : item.role === 'status'
                        ? 'text-xs uppercase tracking-[0.14em] text-mute'
                        : 'inline-block max-w-[90%] whitespace-pre-wrap border border-line px-3 py-2 text-paper/90'
                  }
                >
                  {item.text || (busy ? '…' : '')}
                </p>
              </div>
            ))}
            <div ref={bottom} />
          </div>
          <form
            className="flex items-center border-t border-line"
            onSubmit={(event) => {
              event.preventDefault()
              void send()
            }}
          >
            <textarea
              value={input}
              rows={1}
              onChange={(event) => setInput(event.target.value)}
              onKeyDown={(event) => {
                if (event.key !== 'Enter' || event.shiftKey || event.nativeEvent.isComposing) return
                event.preventDefault()
                void send()
              }}
              placeholder="Ask about a tee or order…"
              className="max-h-28 min-h-12 min-w-0 flex-1 resize-none bg-transparent px-4 py-3 text-sm outline-none placeholder:text-mute"
            />
            <button
              type="submit"
              disabled={busy}
              className="px-4 text-xs uppercase tracking-[0.16em] text-accent disabled:text-mute"
            >
              Send
            </button>
          </form>
        </div>
      ) : null}
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        className="border border-accent bg-ink px-7 py-4 text-sm font-semibold uppercase tracking-[0.18em] text-accent shadow-lg transition hover:bg-accent hover:text-ink"
      >
        AI Chat
      </button>
    </div>
  )
}
