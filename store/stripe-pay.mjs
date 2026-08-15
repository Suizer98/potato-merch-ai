import Stripe from 'stripe'

export function stripeEnabled() {
  const key = process.env.STRIPE_SECRET_KEY || ''
  return key.startsWith('sk_test_') || key.startsWith('sk_live_')
}

function client() {
  return new Stripe(process.env.STRIPE_SECRET_KEY)
}

function publicUrl() {
  return (process.env.STORE_PUBLIC_URL || 'http://localhost:3001').replace(/\/$/, '')
}

export async function createStripeCheckout({ orderId, orderNumber, email, items }) {
  const stripe = client()
  const session = await stripe.checkout.sessions.create({
    mode: 'payment',
    customer_email: email,
    client_reference_id: orderNumber,
    metadata: { orderId, orderNumber },
    currency: 'sgd',
    locale: 'en',
    adaptive_pricing: { enabled: false },
    success_url: `${publicUrl()}/thanks?session_id={CHECKOUT_SESSION_ID}`,
    cancel_url: `${publicUrl()}/`,
    line_items: items.map((item) => ({
      quantity: Number(item.quantity) || 1,
      price_data: {
        currency: 'sgd',
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
