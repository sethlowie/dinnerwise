import { createConnectTransport } from "@connectrpc/connect-web";

// Absolute base URL to the Go Connect server. The server enables permissive
// CORS for local dev, so the browser can call it cross-origin from Vite.
const baseUrl = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

export const transport = createConnectTransport({
  baseUrl,
});
