export type Availability = 'IN_STOCK' | 'PREORDER' | 'SOLD_OUT' | string

export type Product = {
  id: string
  name: string
  sku: string
  description: string
  price: number
  compareAtPrice: number
  stock: number
  category: string
  season: string
  availability: Availability
  sizes: string[]
  imageUrl: string
  isOnSale: boolean
  soldOut: boolean
}

export type CartItem = {
  product: Product
  size: string
  quantity: number
}
