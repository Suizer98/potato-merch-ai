import {
  createOrder,
  fetchProductsFromCrm,
  updateOrderStatus,
  upsertCustomer,
} from './crm-client.mjs'
import {
  applyHostedResult,
  buildWebhookEvent,
  createCheckoutSession,
  getCheckoutSession,
  publicSession,
  signWebhook,
  verifyWebhook,
} from './pay-sessions.mjs'
import {
  createStripeCheckout,
  parseStripeWebhook,
  retrieveStripeSession,
  stripeEnabled,
} from './stripe-pay.mjs'

function json(res, status, body) {
  const payload = JSON.stringify(body)
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Cache-Control': 'no-store',
  })
  res.end(payload)
}

function readRaw(req) {
  return new Promise((resolve, reject) => {
    const chunks = []
    req.on('data', (chunk) => chunks.push(chunk))
    req.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')))
    req.on('error', reject)
  })
}

async function readBody(req) {
  const raw = await readRaw(req)
  if (!raw) return { raw: '', body: {} }
  return { raw, body: JSON.parse(raw) }
}

function lineSummary(items) {
  return items
    .map((item) => `${item.name} (${item.size}) x${item.quantity}`)
    .join('; ')
}

function nextOrderNumber() {
  return `ORD-${Date.now().toString(36).toUpperCase()}`
}

async function applyWebhookEvent(event) {
  const object = event.data?.object || {}
  const orderId = object.metadata?.orderId
  if (!orderId) throw new Error('Webhook session missing orderId')

  if (event.type === 'checkout.session.completed' && object.payment_status === 'paid') {
    await updateOrderStatus(orderId, 'PAID')
    return { received: true, crmStatus: 'PAID' }
  }
  if (
    event.type === 'payment_intent.payment_failed' ||
    event.type === 'checkout.session.expired'
  ) {
    await updateOrderStatus(orderId, 'CANCELLED')
    return { received: true, crmStatus: 'CANCELLED' }
  }
  return { received: true, crmStatus: null }
}

async function dispatchWebhook(session, type) {
  const event = buildWebhookEvent(session, type)
  const payload = JSON.stringify(event)
  const signature = signWebhook(payload)
  if (!verifyWebhook(payload, signature)) {
    throw new Error('Mock webhook signature failed')
  }
  const result = await applyWebhookEvent(event)
  console.log(
    `[store] webhook ${event.type} ${session.id} → CRM ${result.crmStatus || 'ignored'}`,
  )
  return { event, result }
}

export async function handleApi(req, res) {
  const url = new URL(req.url || '/', 'http://store.local')
  const path = url.pathname

  if (req.method === 'GET' && path === '/api/health') {
    json(res, 200, { ok: true })
    return true
  }

  if (req.method === 'GET' && path === '/api/products') {
    try {
      const products = await fetchProductsFromCrm()
      json(res, 200, { products, source: 'crm' })
    } catch (err) {
      console.error('[store] /api/products', err)
      json(res, 502, {
        products: [],
        source: 'error',
        error: err instanceof Error ? err.message : String(err),
      })
    }
    return true
  }

  if (req.method === 'POST' && path === '/api/newsletter') {
    try {
      const { body } = await readBody(req)
      const email = String(body.email || '').trim().toLowerCase()
      if (!email || !email.includes('@')) {
        json(res, 400, { error: 'Valid email required' })
        return true
      }
      await upsertCustomer({
        email,
        firstName: 'Newsletter',
        lastName: 'Subscriber',
      })
      json(res, 200, { ok: true })
    } catch (err) {
      console.error('[store] /api/newsletter', err)
      json(res, 502, { error: err instanceof Error ? err.message : String(err) })
    }
    return true
  }

  if (req.method === 'POST' && path === '/api/checkout') {
    try {
      const { body } = await readBody(req)
      const email = String(body.email || '').trim().toLowerCase()
      const firstName = String(body.firstName || '').trim()
      const lastName = String(body.lastName || '').trim()
      const mailingAddress = String(body.mailingAddress || '').trim()
      const items = Array.isArray(body.items) ? body.items : []
      const total = Number(body.total)

      if (!email || !email.includes('@') || !items.length || !Number.isFinite(total)) {
        json(res, 400, { error: 'Email, items, and total are required' })
        return true
      }

      await upsertCustomer({ email, firstName, lastName, mailingAddress })
      const orderNumber = nextOrderNumber()
      const order = await createOrder({
        orderNumber,
        total,
        customerEmail: email,
        lineItems: lineSummary(items),
        status: 'PENDING',
      })

      if (stripeEnabled()) {
        const session = await createStripeCheckout({
          orderId: order.id,
          orderNumber,
          email,
          items,
        })
        json(res, 200, {
          id: session.id,
          url: session.url,
          payUrl: session.url,
          orderNumber,
        })
        return true
      }

      const session = createCheckoutSession({
        orderId: order.id,
        orderNumber,
        total,
        email,
      })

      json(res, 200, {
        id: session.id,
        url: `/pay?session_id=${session.id}`,
        payUrl: `/pay?session_id=${session.id}`,
        orderNumber,
      })
    } catch (err) {
      console.error('[store] /api/checkout', err)
      json(res, 502, { error: err instanceof Error ? err.message : String(err) })
    }
    return true
  }

  if (req.method === 'GET' && path.startsWith('/api/checkout/sessions/')) {
    const id = decodeURIComponent(path.slice('/api/checkout/sessions/'.length))
    try {
      if (stripeEnabled() && id.startsWith('cs_')) {
        json(res, 200, await retrieveStripeSession(id))
        return true
      }
      const session = getCheckoutSession(id)
      if (!session) {
        json(res, 404, { error: 'Checkout session not found' })
        return true
      }
      json(res, 200, publicSession(session))
    } catch (err) {
      json(res, 404, { error: err instanceof Error ? err.message : String(err) })
    }
    return true
  }

  if (req.method === 'POST' && path === '/api/pay/complete') {
    try {
      const { body } = await readBody(req)
      const sessionId = String(body.session_id || '')
      const result = String(body.result || '')
      const session = getCheckoutSession(sessionId)
      if (!session) {
        json(res, 404, { error: 'Checkout session not found' })
        return true
      }
      if (session.status !== 'open') {
        json(res, 200, {
          url: result === 'paid' ? publicSession(session).success_url : '/',
        })
        return true
      }

      const type = applyHostedResult(session, result)
      await dispatchWebhook(session, type)
      json(res, 200, {
        url: result === 'paid' ? publicSession(session).success_url : '/',
      })
    } catch (err) {
      console.error('[store] /api/pay/complete', err)
      json(res, 502, { error: err instanceof Error ? err.message : String(err) })
    }
    return true
  }

  if (req.method === 'POST' && path === '/api/webhooks/payment') {
    try {
      const raw = await readRaw(req)
      const header = req.headers['stripe-signature'] || req.headers['x-webhook-signature']
      let event
      if (stripeEnabled() && process.env.STRIPE_WEBHOOK_SECRET && req.headers['stripe-signature']) {
        event = parseStripeWebhook(raw, req.headers['stripe-signature'])
      } else if (!verifyWebhook(raw, header)) {
        json(res, 400, { error: 'Invalid webhook signature' })
        return true
      } else {
        event = JSON.parse(raw)
      }
      const result = await applyWebhookEvent(event)
      console.log(
        `[store] webhook ${event.type} ${event.data?.object?.id || ''} → CRM ${result.crmStatus || 'ignored'}`,
      )
      json(res, 200, result)
    } catch (err) {
      console.error('[store] /api/webhooks/payment', err)
      json(res, 400, { error: err instanceof Error ? err.message : String(err) })
    }
    return true
  }

  return false
}
