import http from 'node:http'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { loadEnvFile } from './load-env.mjs'
import { fetchProductsFromCrm } from './crm-client.mjs'

loadEnvFile()

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const DIST = path.join(__dirname, 'dist')
const PORT = Number(process.env.PORT || 3001)

const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.webp': 'image/webp',
  '.ico': 'image/x-icon',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
}

function sendJson(res, status, body) {
  const payload = JSON.stringify(body)
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Cache-Control': 'no-store',
  })
  res.end(payload)
}

function serveStatic(req, res) {
  const urlPath = decodeURIComponent((req.url || '/').split('?')[0])
  const safe = path.normalize(urlPath).replace(/^(\.\.[/\\])+/, '')
  let filePath = path.join(DIST, safe === path.sep ? 'index.html' : safe)

  if (!filePath.startsWith(DIST)) {
    res.writeHead(403)
    res.end('Forbidden')
    return
  }

  if (!fs.existsSync(filePath) || fs.statSync(filePath).isDirectory()) {
    filePath = path.join(DIST, 'index.html')
  }

  const ext = path.extname(filePath)
  const type = MIME[ext] || 'application/octet-stream'
  res.writeHead(200, { 'Content-Type': type })
  fs.createReadStream(filePath).pipe(res)
}

const server = http.createServer(async (req, res) => {
  if (req.method === 'GET' && (req.url || '').startsWith('/api/products')) {
    try {
      const products = await fetchProductsFromCrm()
      sendJson(res, 200, { products, source: 'crm' })
    } catch (err) {
      console.error('[store] /api/products', err)
      sendJson(res, 502, {
        products: [],
        source: 'error',
        error: err instanceof Error ? err.message : String(err),
      })
    }
    return
  }

  if (req.method === 'GET' && (req.url || '').startsWith('/api/health')) {
    sendJson(res, 200, { ok: true })
    return
  }

  serveStatic(req, res)
})

server.listen(PORT, '0.0.0.0', () => {
  console.log(`[store] listening on :${PORT} (CRM=${process.env.CRM_URL || 'http://localhost:3000'})`)
})
