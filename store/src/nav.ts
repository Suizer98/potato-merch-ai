export function go(to: string) {
  const absolute = /^https?:\/\//i.test(to)
  if (absolute) {
    window.location.assign(to)
    return
  }
  window.history.pushState({}, '', to)
  window.dispatchEvent(new PopStateEvent('popstate'))
}
