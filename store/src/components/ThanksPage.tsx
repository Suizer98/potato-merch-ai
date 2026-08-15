import { useEffect, useState } from 'react'
import { formatMoney } from '../api/products'
import { go } from '../nav'

type CheckoutSession = {
  id: string
  status: string
  payment_status: string
  amount_total: number
  customer_email: string
  client_reference_id: string
}

export function ThanksPage({
  sessionId,
  onPaid,
}: {
  sessionId: string
  onPaid: () => void
}) {
  const [session, setSession] = useState<CheckoutSession | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let alive = true
    let attempts = 0

    async function load() {
      try {
        const res = await fetch(`/api/checkout/sessions/${encodeURIComponent(sessionId)}`)
        const data = await res.json()
        if (!res.ok) throw new Error(data.error || 'Session not found')
        if (!alive) return
        setSession(data)
        if (data.payment_status === 'paid') {
          onPaid()
        } else if (attempts < 8) {
          attempts += 1
          window.setTimeout(() => {
            void load()
          }, 400)
        }
      } catch (err) {
        if (alive) setError(err instanceof Error ? err.message : String(err))
      }
    }

    void load()
    return () => {
      alive = false
    }
  }, [sessionId, onPaid])

  const paid = session?.payment_status === 'paid'

  return (
    <main className="mx-auto flex min-h-svh max-w-lg flex-col justify-center px-4 py-16">
      <p className="text-xs uppercase tracking-[0.28em] text-accent">
        {paid ? 'Order confirmed' : 'Checking payment'}
      </p>
      <h1 className="mt-2 font-display text-6xl tracking-[0.06em]">
        {paid ? 'Thanks, spud' : 'Waiting…'}
      </h1>
      <p className="mt-4 text-sm text-mute">
        {error
          ? error
          : paid
            ? `Webhook marked ${session.client_reference_id} Paid in Twenty. Amount ${formatMoney(session.amount_total / 100)}.`
            : 'This page only retrieves the checkout session. CRM is updated by the webhook, not by this URL.'}
      </p>
      {session ? (
        <p className="mt-3 break-all text-[11px] text-mute">{session.id}</p>
      ) : null}
      <button
        type="button"
        onClick={() => go('/')}
        className="mt-8 w-fit bg-paper px-6 py-3 text-xs font-semibold uppercase tracking-[0.22em] text-ink hover:bg-accent"
      >
        Back to shop
      </button>
    </main>
  )
}
