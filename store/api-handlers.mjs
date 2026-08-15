import { randomUUID } from 'node:crypto'
import {
  createOrder,
  fetchProductsFromCrm,
  updateOrderStatus,
  upsertCustomer,
} from './crm-client.mjs'

const pendingPays = new Map()

function json(res, status, body) {
  const payload = JSON.stringify(body)
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Cache-Control': 'no-store',
  })
  res.end(payload)
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = []
    req.on('data', (chunk) => chunks.push(chunk))
    req.on('end', () => {
      const raw = Buffer.concat(chunks).toString('utf8')
      if (!raw) {
        resolve({})
        return
      }
      try {
        resolve(JSON.parse(raw))
      } catch (err) {
        reject(err)
      }
    })
    req.on('error', reject)
  })
}

function lineSummary(items) {
  return items
    .map((item) => `${item.name} (${item.size}) x${item.quantity}`)
    .join('; ')
}

function nextOrderNumber() {
  return `ORD-${Date.now().toString(36).toUpperCase()}`
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
      const body = await readBody(req)
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
      const body = await readBody(req)
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

      const token = randomUUID()
      pendingPays.set(token, {
        orderId: order.id,
        orderNumber,
        total,
        email,
      })

      json(res, 200, {
        orderNumber,
        payUrl: `/pay?t=${encodeURIComponent(token)}`,
      })
    } catch (err) {
      console.error('[store] /api/checkout', err)
      json(res, 502, { error: err instanceof Error ? err.message : String(err) })
    }
    return true
  }

  if (req.method === 'GET' && path === '/api/pay') {
    const token = url.searchParams.get('t') || ''
    const pending = pendingPays.get(token)
    if (!pending) {
      json(res, 404, { error: 'Payment session expired' })
      return true
    }
    json(res, 200, pending)
    return true
  }

  if (req.method === 'POST' && path === '/api/pay') {
    try {
      const body = await readBody(req)
      const token = String(body.token || '')
      const action = String(body.action || '')
      const pending = pendingPays.get(token)
      if (!pending) {
        json(res, 404, { error: 'Payment session expired' })
        return true
      }

      if (action === 'success') {
        await updateOrderStatus(pending.orderId, 'PAID')
        pendingPays.delete(token)
        json(res, 200, {
          ok: true,
          thanksUrl: `/thanks?order=${encodeURIComponent(pending.orderNumber)}`,
        })
        return true
      }

      if (action === 'cancel' || action === 'fail') {
        await updateOrderStatus(pending.orderId, 'CANCELLED')
        pendingPays.delete(token)
        json(res, 200, { ok: true, thanksUrl: '/' })
        return true
      }

      json(res, 400, { error: 'Unknown pay action' })
    } catch (err) {
      console.error('[store] /api/pay', err)
      json(res, 502, { error: err instanceof Error ? err.message : String(err) })
    }
    return true
  }

  return false
}
