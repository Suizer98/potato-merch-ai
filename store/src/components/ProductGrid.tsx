import type { Product } from '../types'
import { ProductCard } from './ProductCard'

type ProductGridProps = {
  products: Product[]
  loading: boolean
  error: string | null
  onAdd: (product: Product, size: string) => void
}

export function ProductGrid({ products, loading, error, onAdd }: ProductGridProps) {
  return (
    <section id="shop" className="mx-auto max-w-7xl px-4 py-16 md:px-8 md:py-24">
      <div className="mb-10 flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <p className="text-xs uppercase tracking-[0.28em] text-accent">Shop</p>
          <h2 className="font-display text-5xl tracking-[0.06em] md:text-6xl">
            Latest drop
          </h2>
        </div>
        <p className="max-w-sm text-sm text-mute">
          Pulled live from Potato Merch CRM inventory. Sale pricing and stock
          update with every restock.
        </p>
      </div>

      {loading ? (
        <div className="grid gap-8 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="animate-soft-pulse space-y-4">
              <div className="aspect-[4/5] bg-paper/5" />
              <div className="h-4 w-2/3 bg-paper/10" />
              <div className="h-3 w-1/3 bg-paper/10" />
            </div>
          ))}
        </div>
      ) : null}

      {!loading && error ? (
        <div className="border border-sale/40 bg-sale/10 px-4 py-6 text-sm text-paper">
          Could not load products from CRM: {error}
        </div>
      ) : null}

      {!loading && !error && products.length === 0 ? (
        <div className="border border-line px-4 py-10 text-center text-sm text-mute">
          No products yet. Start the CRM seed service, then refresh.
        </div>
      ) : null}

      {!loading && products.length > 0 ? (
        <div className="grid gap-x-6 gap-y-12 sm:grid-cols-2 lg:grid-cols-3">
          {products.map((product) => (
            <ProductCard key={product.id} product={product} onAdd={onAdd} />
          ))}
        </div>
      ) : null}
    </section>
  )
}
