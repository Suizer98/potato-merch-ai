export function Footer() {
  return (
    <footer id="contact" className="mx-auto max-w-7xl px-4 py-14 md:px-8">
      <div className="grid gap-10 border-b border-line pb-10 md:grid-cols-3">
        <div>
          <p className="font-display text-4xl tracking-[0.08em]">POTATO</p>
          <p className="mt-3 max-w-xs text-sm text-mute">
            Official Potato Merch storefront. Inventory and customer records live
            in Twenty CRM.
          </p>
        </div>
        <div id="gift">
          <p className="text-xs uppercase tracking-[0.22em] text-paper">Support</p>
          <ul className="mt-3 space-y-2 text-sm text-mute">
            <li>Shipping Policy</li>
            <li>Refund Policy</li>
            <li>Privacy Policy</li>
            <li>Terms of Service</li>
          </ul>
        </div>
        <div>
          <p className="text-xs uppercase tracking-[0.22em] text-paper">Follow</p>
          <ul className="mt-3 space-y-2 text-sm text-mute">
            <li>Twitch</li>
            <li>Instagram</li>
            <li>YouTube</li>
          </ul>
        </div>
      </div>
      <p className="pt-6 text-xs uppercase tracking-[0.18em] text-mute">
        © {new Date().getFullYear()} Potato Merch. All rights reserved.
      </p>
    </footer>
  )
}
