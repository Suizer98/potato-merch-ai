export type ChatChunk = {
  session_id: string
  delta: string
  done: boolean
  error?: string
  agent_id: string
  event: string
}

const SESSION_KEY = 'potato-chat-session'

export function chatSessionId() {
  const existing = sessionStorage.getItem(SESSION_KEY)
  if (existing) return existing
  const next = crypto.randomUUID()
  sessionStorage.setItem(SESSION_KEY, next)
  return next
}

export async function streamChat(
  message: string,
  onChunk: (chunk: ChatChunk) => void,
  signal?: AbortSignal,
) {
  const res = await fetch('/api/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ session_id: chatSessionId(), message }),
    signal,
  })
  if (!res.ok || !res.body) {
    const text = await res.text()
    throw new Error(text || `Chat failed (${res.status})`)
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { value, done } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const parts = buffer.split('\n\n')
    buffer = parts.pop() || ''
    for (const part of parts) {
      const line = part
        .split('\n')
        .map((row) => row.trim())
        .find((row) => row.startsWith('data:'))
      if (!line) continue
      const payload = line.slice(5).trim()
      if (!payload) continue
      onChunk(JSON.parse(payload) as ChatChunk)
    }
  }
}
