import path from 'node:path'
import { defineConfig, loadEnv, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import checker from 'vite-plugin-checker'

function crmApiPlugin(): Plugin {
  return {
    name: 'crm-api',
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        if (!req.url?.startsWith('/api/')) {
          next()
          return
        }
        try {
          // @ts-expect-error plain .mjs module
          const mod = (await import('./server/api-handlers.mjs')) as {
            handleApi: (
              incoming: typeof req,
              outgoing: typeof res,
            ) => Promise<boolean>
          }
          const handled = await mod.handleApi(req, res)
          if (!handled) next()
        } catch (err) {
          const message = err instanceof Error ? err.message : String(err)
          console.error('[store-dev] api', message)
          if (!res.headersSent) {
            res.statusCode = 502
            res.setHeader('Content-Type', 'application/json; charset=utf-8')
            res.end(JSON.stringify({ error: message }))
          }
        }
      })
    },
  }
}

export default defineConfig(({ mode }) => {
  const env = {
    ...loadEnv(mode, path.resolve(process.cwd(), '..'), ''),
    ...loadEnv(mode, process.cwd(), ''),
  }
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
