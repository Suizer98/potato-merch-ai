import { go } from '../nav'

export function ThanksPage({ orderNumber }: { orderNumber: string }) {
  return (
    <main className="mx-auto flex min-h-svh max-w-lg flex-col justify-center px-4 py-16">
      <p className="text-xs uppercase tracking-[0.28em] text-accent">Order confirmed</p>
      <h1 className="mt-2 font-display text-6xl tracking-[0.06em]">Thanks, spud</h1>
      <p className="mt-4 text-sm text-mute">
        {orderNumber
          ? `Order ${orderNumber} is marked Paid in Twenty CRM.`
          : 'Payment finished.'}
      </p>
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
