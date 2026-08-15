import { useEffect, useState } from 'react'
import { formatMoney } from '../api/products'
import { go } from '../nav'

type CheckoutSession = {
  id: string
  payment_status: string
  amount_total: number
  currency?: string
  customer_email: string
  client_reference_id: string
}

export function PayPage({ sessionId, onPaid }: { sessionId: string; onPaid: () => void }) {
  const [session, setSession] = useState<CheckoutSession | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let alive = true
    fetch(`/api/checkout/sessions/${encodeURIComponent(sessionId)}`)
      .then(async (res) => {
        const data = await res.json()
        if (!res.ok) throw new Error(data.error || 'Session expired')
        if (alive) setSession(data)
      })
      .catch((err: unknown) => {
        if (alive) setError(err instanceof Error ? err.message : String(err))
      })
    return () => {
      alive = false
    }
  }, [sessionId])

  async function finish(result: 'paid' | 'failed' | 'canceled') {
    setBusy(true)
    setError(null)
    try {
      const res = await fetch('/api/pay/complete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_id: sessionId, result }),
      })
      const data = (await res.json()) as { url?: string; error?: string }
      if (!res.ok) throw new Error(data.error || 'Payment failed')
      if (result === 'paid') onPaid()
      go(data.url || '/')
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="mx-auto flex min-h-svh max-w-lg flex-col justify-center px-4 py-16">
      <p className="text-xs uppercase tracking-[0.28em] text-accent">
        Hosted checkout · mock Stripe
      </p>
      <h1 className="mt-2 font-display text-6xl tracking-[0.06em]">Potato Pay</h1>
      <p className="mt-3 text-sm text-mute">
        Stands in for Stripe Checkout. Pay now completes the session, then a signed
        webhook marks the CRM order. This page does not write to CRM itself.
      </p>
      {session ? (
        <div className="mt-8 border border-line p-5">
          <p className="break-all text-[11px] uppercase tracking-[0.14em] text-mute">
            {session.id}
          </p>
          <p className="mt-2 text-2xl">{formatMoney(session.amount_total / 100, session.currency)}</p>
          <p className="mt-1 text-sm text-mute">{session.customer_email}</p>
          <p className="mt-1 text-xs text-mute">{session.client_reference_id}</p>
        </div>
      ) : (
        <p className="mt-8 text-sm text-mute">{error || 'Loading checkout session…'}</p>
      )}
      {error && session ? <p className="mt-4 text-xs text-sale">{error}</p> : null}
      <div className="mt-8 flex flex-col gap-3">
        <button
          type="button"
          disabled={!session || busy}
          onClick={() => void finish('paid')}
          className="bg-accent py-3 text-xs font-semibold uppercase tracking-[0.22em] text-ink disabled:opacity-40"
        >
          Pay now
        </button>
        <button
          type="button"
          disabled={!session || busy}
          onClick={() => void finish('failed')}
          className="border border-sale py-3 text-xs uppercase tracking-[0.18em] text-sale disabled:opacity-40"
        >
          Simulate decline
        </button>
        <button
          type="button"
          disabled={busy}
          onClick={() => void finish('canceled')}
          className="py-3 text-xs uppercase tracking-[0.18em] text-mute"
        >
          Cancel
        </button>
      </div>
    </main>
  )
}
