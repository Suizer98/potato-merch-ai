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

function clientError(message) {
  const err = new Error(message)
  err.status = 400
  return err
}

function quantityFromCart(value) {
  const n = Number(value)
  if (!Number.isFinite(n)) return 0
  return Math.min(20, Math.max(1, Math.round(n)))
}

async function pricedItemsFromCrm(rawItems) {
  const catalog = await fetchProductsFromCrm()
  const bySku = new Map()
  for (const product of catalog) {
    if (product.sku) bySku.set(String(product.sku).toLowerCase(), product)
  }

  const items = []
  for (const item of rawItems) {
    const sku = String(item.sku || '').trim()
    const product = bySku.get(sku.toLowerCase())
    if (!product) throw clientError(sku ? `Unknown SKU: ${sku}` : 'Each item needs a SKU')
    if (product.soldOut) throw clientError(`${product.name} is sold out`)
    const quantity = quantityFromCart(item.quantity)
    items.push({
      name: product.name,
      sku: product.sku,
      size: String(item.size || '').trim() || 'M',
      quantity,
      price: Number(product.price),
    })
  }

  const total = items.reduce((sum, item) => sum + item.price * item.quantity, 0)
  if (!Number.isFinite(total) || total <= 0) throw clientError('Cart total is invalid')
  return { items, total }
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

function requiredEnv(name) {
  const value = (process.env[name] || '').trim()
  if (!value) throw new Error(name + ' is required')
  return value
}

function chatHttpUrl() {
  return requiredEnv('CHAT_HTTP_URL').replace(/\/$/, '')
}

export async function handleApi(req, res) {
  const url = new URL(req.url || '/', 'http://store.local')
  const path = url.pathname

  if (req.method === 'GET' && path === '/api/health') {
    json(res, 200, { ok: true })
    return true
  }

  if (req.method === 'POST' && path === '/api/chat') {
    try {
      const { body } = await readBody(req)
      const upstream = await fetch(`${chatHttpUrl()}/v1/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          session_id: body.session_id || '',
          message: body.message || '',
          model: body.model || '',
          agent_id: body.agent_id || '',
        }),
      })
      if (!upstream.ok || !upstream.body) {
        const text = await upstream.text()
        json(res, upstream.status || 502, { error: text || 'Chat unavailable' })
        return true
      }
      res.writeHead(200, {
        'Content-Type': 'text/event-stream; charset=utf-8',
        'Cache-Control': 'no-cache',
        Connection: 'keep-alive',
      })
      for await (const chunk of upstream.body) {
        res.write(chunk)
      }
      res.end()
    } catch (err) {
      console.error('[store] /api/chat', err)
      if (!res.headersSent) {
        json(res, 502, { error: err instanceof Error ? err.message : String(err) })
      } else {
        res.end()
      }
    }
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
      const rawItems = Array.isArray(body.items) ? body.items : []

      if (!email || !email.includes('@') || !rawItems.length) {
        json(res, 400, { error: 'Email and items are required' })
        return true
      }

      const { items, total } = await pricedItemsFromCrm(rawItems)

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
      const status = err && err.status === 400 ? 400 : 502
      json(res, status, { error: err instanceof Error ? err.message : String(err) })
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
