import { useEffect, useState } from 'react'
import { formatMoney } from '../api/products'
import { go } from '../nav'

type PayInfo = {
  orderNumber: string
  total: number
  email: string
}

export function PayPage({ token, onPaid }: { token: string; onPaid: () => void }) {
  const [info, setInfo] = useState<PayInfo | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let alive = true
    fetch(`/api/pay?t=${encodeURIComponent(token)}`)
      .then(async (res) => {
        const data = await res.json()
        if (!res.ok) throw new Error(data.error || 'Session expired')
        if (alive) setInfo(data)
      })
      .catch((err: unknown) => {
        if (alive) setError(err instanceof Error ? err.message : String(err))
      })
    return () => {
      alive = false
    }
  }, [token])

  async function finish(action: 'success' | 'cancel' | 'fail') {
    setBusy(true)
    setError(null)
    try {
      const res = await fetch('/api/pay', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token, action }),
      })
      const data = (await res.json()) as { thanksUrl?: string; error?: string }
      if (!res.ok) throw new Error(data.error || 'Payment failed')
      if (action === 'success') onPaid()
      go(data.thanksUrl || '/')
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="mx-auto flex min-h-svh max-w-lg flex-col justify-center px-4 py-16">
      <p className="text-xs uppercase tracking-[0.28em] text-accent">Potato Pay · demo</p>
      <h1 className="mt-2 font-display text-6xl tracking-[0.06em]">Mock checkout</h1>
      <p className="mt-3 text-sm text-mute">
        Stands in for Stripe/PayPal redirect. No card is charged. Pay marks the CRM
        order Paid; Cancel marks it Cancelled.
      </p>
      {info ? (
        <div className="mt-8 border border-line p-5">
          <p className="text-xs uppercase tracking-[0.18em] text-mute">{info.orderNumber}</p>
          <p className="mt-2 text-2xl">{formatMoney(info.total)}</p>
          <p className="mt-1 text-sm text-mute">{info.email}</p>
        </div>
      ) : (
        <p className="mt-8 text-sm text-mute">{error || 'Loading payment session…'}</p>
      )}
      {error && info ? <p className="mt-4 text-xs text-sale">{error}</p> : null}
      <div className="mt-8 flex flex-col gap-3">
        <button
          type="button"
          disabled={!info || busy}
          onClick={() => void finish('success')}
          className="bg-accent py-3 text-xs font-semibold uppercase tracking-[0.22em] text-ink disabled:opacity-40"
        >
          Pay now
        </button>
        <button
          type="button"
          disabled={!info || busy}
          onClick={() => void finish('fail')}
          className="border border-sale py-3 text-xs uppercase tracking-[0.18em] text-sale disabled:opacity-40"
        >
          Simulate decline
        </button>
        <button
          type="button"
          disabled={busy}
          onClick={() => void finish('cancel')}
          className="py-3 text-xs uppercase tracking-[0.18em] text-mute"
        >
          Cancel and return
        </button>
      </div>
    </main>
  )
}
