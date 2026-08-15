import { createHmac, randomUUID, timingSafeEqual } from 'node:crypto'

const sessions = new Map()

export function webhookSecret() {
  return process.env.PAYMENT_WEBHOOK_SECRET || 'whsec_mock_potato'
}

export function publicSession(session) {
  return {
    id: session.id,
    object: 'checkout.session',
    status: session.status,
    payment_status: session.paymentStatus,
    amount_total: Math.round(session.total * 100),
    currency: 'usd',
    customer_email: session.email,
    client_reference_id: session.orderNumber,
    metadata: { orderId: session.orderId, orderNumber: session.orderNumber },
    success_url: `/thanks?session_id=${session.id}`,
    cancel_url: '/',
  }
}

export function createCheckoutSession({ orderId, orderNumber, total, email }) {
  const id = `cs_mock_${randomUUID().replaceAll('-', '')}`
  const session = {
    id,
    status: 'open',
    paymentStatus: 'unpaid',
    orderId,
    orderNumber,
    total,
    email,
    createdAt: Date.now(),
  }
  sessions.set(id, session)
  return session
}

export function getCheckoutSession(id) {
  return sessions.get(id) || null
}

export function signWebhook(payload, secret = webhookSecret()) {
  const timestamp = Math.floor(Date.now() / 1000)
  const signature = createHmac('sha256', secret)
    .update(`${timestamp}.${payload}`)
    .digest('hex')
  return `t=${timestamp},v1=${signature}`
}

export function verifyWebhook(payload, header, secret = webhookSecret()) {
  const parts = Object.fromEntries(
    String(header || '')
      .split(',')
      .map((piece) => {
        const eq = piece.indexOf('=')
        return [piece.slice(0, eq), piece.slice(eq + 1)]
      }),
  )
  const timestamp = parts.t
  const expected = createHmac('sha256', secret)
    .update(`${timestamp}.${payload}`)
    .digest('hex')
  const given = parts.v1 || ''
  if (!timestamp || given.length !== expected.length) return false
  const age = Math.abs(Date.now() / 1000 - Number(timestamp))
  if (age > 300) return false
  return timingSafeEqual(Buffer.from(given), Buffer.from(expected))
}

export function buildWebhookEvent(session, type) {
  return {
    id: `evt_mock_${randomUUID().replaceAll('-', '')}`,
    object: 'event',
    type,
    created: Math.floor(Date.now() / 1000),
    data: { object: publicSession(session) },
  }
}

export function applyHostedResult(session, result) {
  if (result === 'paid') {
    session.status = 'complete'
    session.paymentStatus = 'paid'
    return 'checkout.session.completed'
  }
  if (result === 'failed') {
    session.status = 'complete'
    session.paymentStatus = 'unpaid'
    return 'payment_intent.payment_failed'
  }
  session.status = 'expired'
  session.paymentStatus = 'unpaid'
  return 'checkout.session.expired'
}
