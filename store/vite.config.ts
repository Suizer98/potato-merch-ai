import { defineConfig, loadEnv, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import checker from 'vite-plugin-checker'

type CrmProduct = {
  id: string
  name: string
  sku: string
  description: string
  price: number
  compareAtPrice: number
  stock: number
  category: string
  season: string
  availability: string
  sizes: string[]
  imageUrl: string
  isOnSale: boolean
  soldOut: boolean
}

function crmApiPlugin(): Plugin {
  return {
    name: 'crm-api',
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        if (!req.url?.startsWith('/api/products')) {
          next()
          return
        }
        try {
          // Runtime ESM helper shared with server.mjs (no TS types shipped).
          // @ts-expect-error plain .mjs module
          const mod = (await import('./crm-client.mjs')) as {
            fetchProductsFromCrm: () => Promise<CrmProduct[]>
          }
          const products = await mod.fetchProductsFromCrm()
          const body = JSON.stringify({ products, source: 'crm' })
          res.setHeader('Content-Type', 'application/json; charset=utf-8')
          res.setHeader('Cache-Control', 'no-store')
          res.end(body)
        } catch (err) {
          const message = err instanceof Error ? err.message : String(err)
          console.error('[store-dev] /api/products', message)
          res.statusCode = 502
          res.setHeader('Content-Type', 'application/json; charset=utf-8')
          res.end(JSON.stringify({ products: [], source: 'error', error: message }))
        }
      })
    },
  }
}

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  for (const [key, value] of Object.entries(env)) {
    if (process.env[key] === undefined) process.env[key] = value
  }

  return {
    plugins: [
      react(),
      tailwindcss(),
      checker({ typescript: true, enableBuild: false }),
      crmApiPlugin(),
    ],
    server: {
      host: true,
      port: Number(process.env.PORT || 3001),
    },
    preview: {
      host: true,
      port: Number(process.env.PORT || 3001),
    },
  }
})
