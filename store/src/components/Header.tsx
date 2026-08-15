import { Menu } from '@base-ui/react/menu'
import { go } from '../nav'

type HeaderProps = {
  cartCount: number
  onOpenCart: () => void
  season: string
  onSeasonChange: (season: string) => void
}

const SEASONS = [
  { value: 'ALL', label: 'Shop' },
  { value: 'SEASON_3', label: 'Season 3' },
  { value: 'SEASON_2', label: 'Season 2' },
  { value: 'SEASON_1', label: 'Season 1' },
]

export function Header({ cartCount, onOpenCart, season, onSeasonChange }: HeaderProps) {
  return (
    <header className="sticky top-0 z-40 border-b border-line/80 bg-ink/90 backdrop-blur-md">
      <div className="mx-auto flex max-w-7xl items-center justify-between gap-4 px-4 py-4 md:px-8">
        <nav className="hidden items-center gap-6 text-xs uppercase tracking-[0.2em] text-paper/80 md:flex">
          <a href="#shop" className="transition hover:text-accent">
            Home
          </a>
          <Menu.Root>
            <Menu.Trigger className="inline-flex items-center gap-1 uppercase tracking-[0.2em] outline-none hover:text-accent data-popup-open:text-accent">
              Shop
              <span aria-hidden>▾</span>
            </Menu.Trigger>
            <Menu.Portal>
              <Menu.Positioner sideOffset={8} className="z-50">
                <Menu.Popup className="min-w-40 origin-[var(--transform-origin)] border border-line bg-ink p-1 text-paper shadow-xl outline-none transition data-ending-style:scale-95 data-ending-style:opacity-0 data-starting-style:scale-95 data-starting-style:opacity-0">
                  {SEASONS.map((item) => (
                    <Menu.Item
                      key={item.value}
                      className="cursor-pointer px-3 py-2 text-xs uppercase tracking-[0.16em] outline-none data-highlighted:bg-paper data-highlighted:text-ink"
                      onClick={() => onSeasonChange(item.value)}
                    >
                      {item.label}
                    </Menu.Item>
                  ))}
                </Menu.Popup>
              </Menu.Positioner>
            </Menu.Portal>
          </Menu.Root>
          <a href="#gift" className="transition hover:text-accent">
            Gift
          </a>
          <a href="#contact" className="transition hover:text-accent">
            Contact
          </a>
        </nav>

        <button
          type="button"
          onClick={() => go('/')}
          className="font-display text-4xl leading-none tracking-[0.08em] text-paper md:absolute md:left-1/2 md:-translate-x-1/2"
        >
          POTATO
        </button>

        <div className="flex items-center gap-4 text-xs uppercase tracking-[0.18em]">
          <span className="hidden text-mute sm:inline">
            {season === 'ALL' ? 'All drops' : season.replace('_', ' ')}
          </span>
          <button
            type="button"
            onClick={onOpenCart}
            className="relative border border-paper/30 px-3 py-2 transition hover:border-accent hover:text-accent"
          >
            Cart
            <span className="ml-2 inline-flex min-w-5 items-center justify-center bg-paper px-1 text-[10px] font-bold text-ink">
              {cartCount}
            </span>
          </button>
        </div>
      </div>
    </header>
  )
}
