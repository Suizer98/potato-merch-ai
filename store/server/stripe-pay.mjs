import Stripe from 'stripe'

export function stripeEnabled() {
  const key = process.env.STRIPE_SECRET_KEY || ''
  return key.startsWith('sk_test_') || key.startsWith('sk_live_')
}

function client() {
  return new Stripe(process.env.STRIPE_SECRET_KEY)
}

function publicUrl() {
  const url = (process.env.STORE_PUBLIC_URL || '').trim()
  if (!url) throw new Error('STORE_PUBLIC_URL is required')
  return url.replace(/\/$/, '')
}

export async function createStripeCheckout({ orderId, orderNumber, email, items, stock }) {
  const stripe = client()
  const session = await stripe.checkout.sessions.create({
    mode: 'payment',
    customer_email: email,
    client_reference_id: orderNumber,
    metadata: { orderId, orderNumber, stock: stock || '' },
    currency: 'usd',
    expires_at: Math.floor(Date.now() / 1000) + 30 * 60,
    success_url: `${publicUrl()}/thanks?session_id={CHECKOUT_SESSION_ID}`,
    cancel_url: `${publicUrl()}/`,
    line_items: items.map((item) => ({
      quantity: Number(item.quantity) || 1,
      price_data: {
        currency: 'usd',
        unit_amount: Math.round(Number(item.price) * 100),
        product_data: {
          name: `${item.name} (${item.size})`,
        },
      },
    })),
  })
  return session
}

export async function retrieveStripeSession(id) {
  const session = await client().checkout.sessions.retrieve(id)
  return {
    id: session.id,
    object: 'checkout.session',
    status: session.status,
    payment_status: session.payment_status,
    amount_total: session.amount_total,
    currency: session.currency,
    customer_email: session.customer_email,
    client_reference_id: session.client_reference_id,
    metadata: session.metadata || {},
  }
}

export function parseStripeWebhook(raw, signature) {
  const secret = process.env.STRIPE_WEBHOOK_SECRET || ''
  if (!secret) {
    throw new Error('STRIPE_WEBHOOK_SECRET is not set')
  }
  return client().webhooks.constructEvent(raw, signature, secret)
}
