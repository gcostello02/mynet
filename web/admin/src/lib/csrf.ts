export function readCSRFCookie(): string {
  const parts = document.cookie.split(";").map((c) => c.trim());
  for (const p of parts) {
    if (p.startsWith("mynet_csrf=")) {
      return decodeURIComponent(p.slice("mynet_csrf=".length));
    }
  }
  return "";
}
