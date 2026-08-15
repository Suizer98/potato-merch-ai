import type { Product } from '../types'

type ProductsResponse = {
  products: Product[]
  source: string
  error?: string
}

export async function loadProducts(): Promise<ProductsResponse> {
  const res = await fetch('/api/products')
  const data = (await res.json()) as ProductsResponse
  return data
}

export function formatMoney(amount: number, currency = 'USD') {
  const code = (currency || 'USD').toUpperCase()
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: code,
  }).format(amount)
}

export function discountPercent(price: number, compareAt: number): number | null {
  if (!compareAt || compareAt <= price) return null
  return Math.round(((compareAt - price) / compareAt) * 100)
}

export function seasonLabel(season: string): string {
  switch (season) {
    case 'SEASON_1':
      return 'Season 1'
    case 'SEASON_2':
      return 'Season 2'
    case 'SEASON_3':
      return 'Season 3'
    default:
      return season
  }
}
