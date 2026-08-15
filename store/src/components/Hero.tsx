export function Hero() {
  return (
    <section className="relative overflow-hidden border-b border-line">
      <div
        className="absolute inset-0 opacity-40"
        style={{
          backgroundImage:
            'radial-gradient(circle at 20% 20%, rgba(200,255,61,0.18), transparent 42%), radial-gradient(circle at 80% 0%, rgba(255,255,255,0.08), transparent 35%), linear-gradient(160deg, #111 0%, #050505 55%, #141414 100%)',
        }}
      />
      <div
        className="absolute inset-0 opacity-[0.12]"
        style={{
          backgroundImage:
            'url("data:image/svg+xml,%3Csvg viewBox=\'0 0 200 200\' xmlns=\'http://www.w3.org/2000/svg\'%3E%3Cfilter id=\'n\'%3E%3CfeTurbulence type=\'fractalNoise\' baseFrequency=\'0.85\' numOctaves=\'3\' stitchTiles=\'stitch\'/%3E%3C/filter%3E%3Crect width=\'100%25\' height=\'100%25\' filter=\'url(%23n)\'/%3E%3C/svg%3E")',
        }}
      />

      <div className="relative mx-auto flex min-h-[72svh] max-w-7xl flex-col justify-end gap-6 px-4 pb-16 pt-24 md:px-8 md:pb-24">
        <p className="animate-fade-up text-xs uppercase tracking-[0.35em] text-accent">
          Potato Merch · Season 3
        </p>
        <h1 className="animate-fade-up font-display text-[clamp(4.5rem,16vw,10rem)] leading-[0.85] tracking-[0.04em]">
          WEAR
          <br />
          THE SPUD
        </h1>
        <p className="animate-fade-up max-w-md text-sm leading-relaxed text-paper/70 md:text-base">
          Limited drops, heavyweight cotton, cartoon potato energy. New colorways
          and restocks ship free over $75.
        </p>
        <div className="animate-fade-up flex flex-wrap gap-3">
          <a
            href="#shop"
            className="bg-paper px-6 py-3 text-xs font-semibold uppercase tracking-[0.22em] text-ink transition hover:bg-accent"
          >
            Shop now
          </a>
          <a
            href="#newsletter"
            className="border border-paper/40 px-6 py-3 text-xs uppercase tracking-[0.22em] transition hover:border-accent hover:text-accent"
          >
            Get restock alerts
          </a>
        </div>
      </div>
    </section>
  )
}
