import { useEffect, useMemo, useState } from 'react'
import { loadProducts } from './api/products'
import { CartDrawer } from './components/CartDrawer'
import { ChatWidget } from './components/ChatWidget'
import { Footer } from './components/Footer'
import { Header } from './components/Header'
import { Hero } from './components/Hero'
import { Marquee } from './components/Marquee'
import { Newsletter } from './components/Newsletter'
import { PayPage } from './components/PayPage'
import { ProductGrid } from './components/ProductGrid'
import { ThanksPage } from './components/ThanksPage'
import { useCart } from './hooks/useCart'
import type { Product } from './types'

function currentHref() {
  return window.location.pathname + window.location.search
}

export default function App() {
  const cart = useCart()
  const [href, setHref] = useState(currentHref)
  const [products, setProducts] = useState<Product[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [cartOpen, setCartOpen] = useState(false)
  const [season, setSeason] = useState('ALL')

  useEffect(() => {
    const onPop = () => setHref(currentHref())
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])

  useEffect(() => {
    let alive = true
    setLoading(true)
    loadProducts()
      .then((data) => {
        if (!alive) return
        setProducts(data.products || [])
        setError(data.source === 'error' ? data.error || 'CRM unavailable' : null)
      })
      .catch((err: unknown) => {
        if (!alive) return
        setError(err instanceof Error ? err.message : String(err))
        setProducts([])
      })
      .finally(() => {
        if (alive) setLoading(false)
      })
    return () => {
      alive = false
    }
  }, [])

  const filtered = useMemo(() => {
    if (season === 'ALL') return products
    return products.filter((product) => product.season === season)
  }, [products, season])

  const url = new URL(href, window.location.origin)
  if (url.pathname === '/pay') {
    return (
      <PayPage sessionId={url.searchParams.get('session_id') || ''} onPaid={cart.clear} />
    )
  }
  if (url.pathname === '/thanks') {
    return <ThanksPage sessionId={url.searchParams.get('session_id') || ''} onPaid={cart.clear} />
  }

  return (
    <div className="root min-h-svh bg-ink text-paper">
      <Marquee />
      <Header
        cartCount={cart.count}
        onOpenCart={() => setCartOpen(true)}
        season={season}
        onSeasonChange={setSeason}
      />
      <Hero />
      <ProductGrid
        products={filtered}
        loading={loading}
        error={error}
        onAdd={(product, size) => {
          cart.addItem(product, size)
          setCartOpen(true)
        }}
      />
      <Newsletter />
      <Footer />
      <CartDrawer
        open={cartOpen}
        onOpenChange={setCartOpen}
        items={cart.items}
        total={cart.total}
        onRemove={cart.removeItem}
        onQuantity={cart.setQuantity}
      />
      <ChatWidget />
    </div>
  )
}
