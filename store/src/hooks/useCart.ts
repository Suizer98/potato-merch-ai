import { useCallback, useMemo, useState } from 'react'
import type { CartItem, Product } from '../types'

export function useCart() {
  const [items, setItems] = useState<CartItem[]>([])

  const addItem = useCallback((product: Product, size: string) => {
    setItems((prev) => {
      const idx = prev.findIndex(
        (item) => item.product.id === product.id && item.size === size,
      )
      if (idx === -1) return [...prev, { product, size, quantity: 1 }]
      return prev.map((item, i) =>
        i === idx ? { ...item, quantity: item.quantity + 1 } : item,
      )
    })
  }, [])

  const removeItem = useCallback((productId: string, size: string) => {
    setItems((prev) =>
      prev.filter((item) => !(item.product.id === productId && item.size === size)),
    )
  }, [])

  const setQuantity = useCallback((productId: string, size: string, quantity: number) => {
    setItems((prev) => {
      if (quantity <= 0) {
        return prev.filter(
          (item) => !(item.product.id === productId && item.size === size),
        )
      }
      return prev.map((item) =>
        item.product.id === productId && item.size === size
          ? { ...item, quantity }
          : item,
      )
    })
  }, [])

  const clear = useCallback(() => setItems([]), [])

  const count = useMemo(
    () => items.reduce((sum, item) => sum + item.quantity, 0),
    [items],
  )

  const total = useMemo(
    () => items.reduce((sum, item) => sum + item.product.price * item.quantity, 0),
    [items],
  )

  return { items, addItem, removeItem, setQuantity, clear, count, total }
}
