import { useState } from 'react'

export function Newsletter() {
  const [email, setEmail] = useState('')
  const [done, setDone] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  return (
    <section
      id="newsletter"
      className="border-y border-line bg-[linear-gradient(180deg,#101010_0%,#070707_100%)]"
    >
      <div className="mx-auto flex max-w-7xl flex-col gap-6 px-4 py-16 md:flex-row md:items-end md:justify-between md:px-8 md:py-20">
        <div className="max-w-xl">
          <p className="text-xs uppercase tracking-[0.28em] text-accent">Newsletter</p>
          <h2 className="mt-2 font-display text-4xl tracking-[0.06em] md:text-5xl">
            Be first on drops
          </h2>
          <p className="mt-3 text-sm text-mute">
            New drops, restocked items, and special deals — straight to your inbox.
          </p>
        </div>

        <form
          className="flex w-full max-w-md flex-col gap-3 sm:flex-row"
          onSubmit={(event) => {
            event.preventDefault()
            if (!email.trim()) return
            setBusy(true)
            setError(null)
            void fetch('/api/newsletter', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ email }),
            })
              .then(async (res) => {
                const data = await res.json()
                if (!res.ok) throw new Error(data.error || 'Subscribe failed')
                setDone(true)
                setEmail('')
              })
              .catch((err: unknown) => {
                setError(err instanceof Error ? err.message : String(err))
              })
              .finally(() => setBusy(false))
          }}
        >
          <label className="sr-only" htmlFor="newsletter-email">
            Email
          </label>
          <input
            id="newsletter-email"
            type="email"
            required
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            placeholder="you@example.com"
            className="flex-1 border border-paper/25 bg-transparent px-4 py-3 text-sm outline-none placeholder:text-mute focus:border-accent"
          />
          <button
            type="submit"
            disabled={busy}
            className="bg-paper px-5 py-3 text-xs font-semibold uppercase tracking-[0.2em] text-ink hover:bg-accent disabled:opacity-60"
          >
            Subscribe
          </button>
        </form>
      </div>
      {error ? (
        <p className="pb-8 text-center text-xs text-sale">{error}</p>
      ) : done ? (
        <p className="pb-8 text-center text-xs uppercase tracking-[0.2em] text-accent">
          You’re on the list — saved in CRM.
        </p>
      ) : null}
    </section>
  )
}
