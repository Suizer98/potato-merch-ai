import { useMemo, useState } from 'react'
import { Select } from '@base-ui/react/select'
import type { Product } from '../types'
import { discountPercent, formatMoney, seasonLabel } from '../api/products'

type ProductCardProps = {
  product: Product
  onAdd: (product: Product, size: string) => void
}

export function ProductCard({ product, onAdd }: ProductCardProps) {
  const sizes = product.sizes.length ? product.sizes : ['M']
  const [size, setSize] = useState(sizes[0] ?? 'M')
  const off = useMemo(
    () => discountPercent(product.price, product.compareAtPrice),
    [product.price, product.compareAtPrice],
  )
  const disabled = product.soldOut && product.availability !== 'PREORDER'

  return (
    <article className="group flex flex-col gap-4">
      <div className="relative aspect-[4/5] overflow-hidden bg-[#151515]">
        {product.imageUrl ? (
          <img
            src={product.imageUrl}
            alt={product.name}
            className="h-full w-full object-cover transition duration-700 group-hover:scale-[1.04]"
            loading="lazy"
          />
        ) : (
          <div className="flex h-full items-center justify-center font-display text-6xl text-paper/20">
            SPUD
          </div>
        )}

        {off != null && product.isOnSale ? (
          <span className="absolute left-3 top-3 bg-sale px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-white">
            {off}% off
          </span>
        ) : null}

        {product.availability === 'PREORDER' ? (
          <span className="absolute right-3 top-3 bg-paper px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-ink">
            Preorder
          </span>
        ) : null}

        {disabled ? (
          <div className="absolute inset-0 flex items-center justify-center bg-ink/55">
            <span className="border border-paper px-4 py-2 text-xs uppercase tracking-[0.25em]">
              Sold out
            </span>
          </div>
        ) : null}
      </div>

      <div className="flex flex-1 flex-col gap-3">
        <div className="space-y-1">
          <p className="text-[10px] uppercase tracking-[0.22em] text-mute">
            {seasonLabel(product.season)}
          </p>
          <h3 className="text-base font-medium tracking-wide">{product.name}</h3>
          <div className="flex items-baseline gap-2 text-sm">
            {product.compareAtPrice > product.price ? (
              <span className="text-mute line-through">
                {formatMoney(product.compareAtPrice)}
              </span>
            ) : null}
            <span className={product.isOnSale ? 'text-sale' : ''}>
              {formatMoney(product.price)}
            </span>
          </div>
        </div>

        <div className="mt-auto flex gap-2">
          <Select.Root
            value={size}
            onValueChange={(value) => {
              if (typeof value === 'string') setSize(value)
            }}
            disabled={disabled}
          >
            <Select.Trigger className="flex min-w-24 flex-1 items-center justify-between border border-paper/25 px-3 py-2 text-xs uppercase tracking-[0.14em] outline-none disabled:opacity-40 data-popup-open:border-accent">
              <Select.Value />
              <Select.Icon className="opacity-60">▾</Select.Icon>
            </Select.Trigger>
            <Select.Portal>
              <Select.Positioner className="z-50" sideOffset={4}>
                <Select.Popup className="max-h-60 origin-[var(--transform-origin)] overflow-auto border border-line bg-ink text-paper outline-none transition data-ending-style:scale-95 data-ending-style:opacity-0 data-starting-style:scale-95 data-starting-style:opacity-0">
                  {sizes.map((option) => (
                    <Select.Item
                      key={option}
                      value={option}
                      className="cursor-pointer px-3 py-2 text-xs uppercase tracking-[0.14em] outline-none data-highlighted:bg-paper data-highlighted:text-ink"
                    >
                      <Select.ItemText>{option}</Select.ItemText>
                    </Select.Item>
                  ))}
                </Select.Popup>
              </Select.Positioner>
            </Select.Portal>
          </Select.Root>

          <button
            type="button"
            disabled={disabled}
            onClick={() => onAdd(product, size)}
            className="flex-[1.4] bg-paper px-3 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-ink transition hover:bg-accent disabled:cursor-not-allowed disabled:bg-paper/30 disabled:text-paper/50"
          >
            {product.availability === 'PREORDER' ? 'Preorder' : 'Add to cart'}
          </button>
        </div>
      </div>
    </article>
  )
}
