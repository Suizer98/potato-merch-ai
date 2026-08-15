const PHRASE = 'FREE SHIPPING ON ORDERS $75 SGD & UP'

export function Marquee() {
  const items = Array.from({ length: 12 }, (_, i) => (
    <span key={i} className="mx-6 whitespace-nowrap tracking-[0.18em]">
      {PHRASE}
    </span>
  ))

  return (
    <div className="overflow-hidden border-b border-line bg-accent text-ink">
      <div className="animate-marquee flex w-max py-2 text-[11px] font-semibold uppercase">
        {items}
        {items}
      </div>
    </div>
  )
}
