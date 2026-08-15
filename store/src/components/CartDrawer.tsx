import { Dialog } from '@base-ui/react/dialog'
import { useState } from 'react'
import type { CartItem } from '../types'
import { formatMoney } from '../api/products'
import { go } from '../nav'

type CartDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  items: CartItem[]
  total: number
  onRemove: (productId: string, size: string) => void
  onQuantity: (productId: string, size: string, quantity: number) => void
}

export function CartDrawer({
  open,
  onOpenChange,
  items,
  total,
  onRemove,
  onQuantity,
}: CartDrawerProps) {
  const [step, setStep] = useState<'cart' | 'checkout'>('cart')
  const [email, setEmail] = useState('')
  const [firstName, setFirstName] = useState('')
  const [lastName, setLastName] = useState('')
  const [mailingAddress, setMailingAddress] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function checkout() {
    setBusy(true)
    setError(null)
    try {
      const res = await fetch('/api/checkout', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email,
          firstName,
          lastName,
          mailingAddress,
          total,
          items: items.map((item) => ({
            name: item.product.name,
            sku: item.product.sku,
            size: item.size,
            quantity: item.quantity,
            price: item.product.price,
          })),
        }),
      })
      const data = (await res.json()) as { payUrl?: string; error?: string }
      if (!res.ok || !data.payUrl) throw new Error(data.error || 'Checkout failed')
      onOpenChange(false)
      setStep('cart')
      go(data.payUrl)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next)
        if (!next) setStep('cart')
      }}
    >
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 z-50 bg-black/60 transition data-ending-style:opacity-0 data-starting-style:opacity-0" />
        <Dialog.Popup className="fixed inset-y-0 right-0 z-50 flex w-full max-w-md origin-right flex-col border-l border-line bg-ink text-paper outline-none transition data-ending-style:translate-x-8 data-ending-style:opacity-0 data-starting-style:translate-x-8 data-starting-style:opacity-0">
          <div className="flex items-center justify-between border-b border-line px-5 py-4">
            <Dialog.Title className="font-display text-3xl tracking-[0.08em]">
              {step === 'checkout' ? 'Checkout' : 'Cart'}
            </Dialog.Title>
            <Dialog.Close className="text-xs uppercase tracking-[0.2em] text-mute hover:text-paper">
              Close
            </Dialog.Close>
          </div>

          <div className="flex-1 overflow-y-auto px-5 py-4">
            {step === 'cart' ? (
              items.length === 0 ? (
                <p className="py-16 text-center text-sm text-mute">Your cart is empty</p>
              ) : (
                <ul className="space-y-5">
                  {items.map((item) => (
                    <li
                      key={`${item.product.id}-${item.size}`}
                      className="flex gap-4 border-b border-line/70 pb-5"
                    >
                      <div className="h-24 w-20 shrink-0 overflow-hidden bg-[#151515]">
                        {item.product.imageUrl ? (
                          <img
                            src={item.product.imageUrl}
                            alt=""
                            className="h-full w-full object-cover"
                          />
                        ) : null}
                      </div>
                      <div className="flex flex-1 flex-col gap-2">
                        <div className="flex items-start justify-between gap-3">
                          <div>
                            <p className="text-sm font-medium">{item.product.name}</p>
                            <p className="text-xs uppercase tracking-[0.16em] text-mute">
                              Size {item.size}
                            </p>
                          </div>
                          <p className="text-sm">
                            {formatMoney(item.product.price * item.quantity)}
                          </p>
                        </div>
                        <div className="mt-auto flex items-center justify-between">
                          <div className="flex items-center border border-paper/20">
                            <button
                              type="button"
                              className="px-3 py-1 text-sm"
                              onClick={() =>
                                onQuantity(item.product.id, item.size, item.quantity - 1)
                              }
                            >
                              −
                            </button>
                            <span className="min-w-8 text-center text-sm">{item.quantity}</span>
                            <button
                              type="button"
                              className="px-3 py-1 text-sm"
                              onClick={() =>
                                onQuantity(item.product.id, item.size, item.quantity + 1)
                              }
                            >
                              +
                            </button>
                          </div>
                          <button
                            type="button"
                            className="text-[11px] uppercase tracking-[0.16em] text-mute hover:text-sale"
                            onClick={() => onRemove(item.product.id, item.size)}
                          >
                            Remove
                          </button>
                        </div>
                      </div>
                    </li>
                  ))}
                </ul>
              )
            ) : (
              <form
                id="checkout-form"
                className="flex flex-col gap-3"
                onSubmit={(event) => {
                  event.preventDefault()
                  void checkout()
                }}
              >
                <p className="text-xs text-mute">
                  Creates a PENDING order in Twenty CRM, then redirects to a mock
                  Potato Pay page.
                </p>
                <input
                  required
                  type="email"
                  placeholder="Email"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  className="border border-paper/25 bg-transparent px-3 py-2 text-sm outline-none focus:border-accent"
                />
                <div className="grid grid-cols-2 gap-3">
                  <input
                    required
                    placeholder="First name"
                    value={firstName}
                    onChange={(event) => setFirstName(event.target.value)}
                    className="border border-paper/25 bg-transparent px-3 py-2 text-sm outline-none focus:border-accent"
                  />
                  <input
                    required
                    placeholder="Last name"
                    value={lastName}
                    onChange={(event) => setLastName(event.target.value)}
                    className="border border-paper/25 bg-transparent px-3 py-2 text-sm outline-none focus:border-accent"
                  />
                </div>
                <textarea
                  required
                  rows={3}
                  placeholder="Shipping address"
                  value={mailingAddress}
                  onChange={(event) => setMailingAddress(event.target.value)}
                  className="border border-paper/25 bg-transparent px-3 py-2 text-sm outline-none focus:border-accent"
                />
                {error ? <p className="text-xs text-sale">{error}</p> : null}
              </form>
            )}
          </div>

          <div className="border-t border-line px-5 py-5">
            <div className="mb-4 flex items-center justify-between text-sm">
              <span className="uppercase tracking-[0.18em] text-mute">Subtotal</span>
              <span className="font-medium">{formatMoney(total)}</span>
            </div>
            {step === 'cart' ? (
              <button
                type="button"
                disabled={items.length === 0}
                onClick={() => setStep('checkout')}
                className="w-full bg-accent py-3 text-xs font-semibold uppercase tracking-[0.22em] text-ink transition hover:brightness-110 disabled:cursor-not-allowed disabled:bg-paper/20 disabled:text-paper/40"
              >
                Checkout
              </button>
            ) : (
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => setStep('cart')}
                  className="flex-1 border border-paper/30 py-3 text-xs uppercase tracking-[0.18em]"
                >
                  Back
                </button>
                <button
                  type="submit"
                  form="checkout-form"
                  disabled={busy}
                  className="flex-[2] bg-accent py-3 text-xs font-semibold uppercase tracking-[0.22em] text-ink disabled:opacity-60"
                >
                  {busy ? 'Creating order…' : 'Pay with Potato Pay'}
                </button>
              </div>
            )}
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
