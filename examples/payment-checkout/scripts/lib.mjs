import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

function readEnvFile() {
  const candidates = [path.join(root, ".env.local"), path.join(root, ".env")];
  const result = {};
  for (const candidate of candidates) {
    if (!fs.existsSync(candidate)) continue;
    for (const line of fs.readFileSync(candidate, "utf8").split(/\r?\n/)) {
      const value = line.trim();
      if (!value || value.startsWith("#")) continue;
      const split = value.indexOf("=");
      if (split > 0) result[value.slice(0, split)] = value.slice(split + 1);
    }
    break;
  }
  return result;
}

const fileEnv = readEnvFile();
const env = (name, fallback = "") => process.env[name] || fileEnv[name] || fallback;

export const config = {
  baseUrl: env("DAPTIN_BASE_URL", "http://localhost:6336").replace(/\/+$/, ""),
  adminEmail: env("DAPTIN_ADMIN_EMAIL", "admin@example.com"),
  adminPassword: env("DAPTIN_ADMIN_PASSWORD"),
  buyerEmail: env("CHECKOUT_USER_EMAIL", "buyer@example.com"),
  buyerPassword: env("CHECKOUT_USER_PASSWORD"),
  serviceEmail: env("PAYMENTS_SERVICE_EMAIL", "payments-service@example.com"),
  servicePassword: env("PAYMENTS_SERVICE_PASSWORD"),
  stripeSecretKey: env("STRIPE_SECRET_KEY"),
  stripePriceId: env("STRIPE_PRICE_ID"),
  stripeApiBaseUrl: env("STRIPE_API_BASE_URL", "https://api.stripe.com").replace(/\/+$/, ""),
  successUrl: env("CHECKOUT_SUCCESS_URL", "http://localhost:3000/payment/success"),
  cancelUrl: env("CHECKOUT_CANCEL_URL", "http://localhost:3000/payment/cancel"),
};

export function requireConfig() {
  for (const [name, value] of Object.entries({
    DAPTIN_ADMIN_PASSWORD: config.adminPassword,
    CHECKOUT_USER_PASSWORD: config.buyerPassword,
    PAYMENTS_SERVICE_PASSWORD: config.servicePassword,
    STRIPE_SECRET_KEY: config.stripeSecretKey,
    STRIPE_PRICE_ID: config.stripePriceId,
  })) {
    if (!value || value.includes("replace")) throw new Error(`${name} must be set in .env.local`);
  }
}

export async function request(method, route, body, token, contentType = "application/vnd.api+json", tolerate = false) {
  const response = await fetch(config.baseUrl + route, {
    method,
    headers: {
      ...(body === undefined ? {} : { "Content-Type": contentType }),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const responseText = await response.text();
  let payload = responseText;
  try { payload = responseText ? JSON.parse(responseText) : null; } catch { /* retain text */ }
  if (!response.ok && !tolerate) {
    const error = new Error(`${method} ${route} returned HTTP ${response.status}`);
    error.payload = payload;
    throw error;
  }
  return { ok: response.ok, status: response.status, payload };
}

export function items(response) {
  const data = response?.payload?.data;
  if (!data) return [];
  return Array.isArray(data) ? data : [data];
}

export function attr(item, key) {
  return item?.attributes?.[key] ?? item?.[key];
}

export async function action(type, name, attributes, token) {
  const response = await request("POST", `/action/${type}/${name}`, { attributes: attributes || {} }, token, "application/json");
  return response.payload;
}

export async function signIn(email, password) {
  const payload = await action("user_account", "signin", { email, password });
  for (const entry of Array.isArray(payload) ? payload : []) {
    const attributes = entry.Attributes || entry.attributes || {};
    if (attributes.key === "token" && attributes.value) return attributes.value;
    if (attributes.token) return attributes.token;
  }
  throw new Error(`sign in did not return a token for ${email}`);
}

export async function signUp(email, password, name) {
  const existing = await request("POST", "/action/user_account/signin", { attributes: { email, password } }, "", "application/json", true);
  if (existing.ok) return;
  await request("POST", "/action/user_account/signup", {
    attributes: { email, name, password, passwordConfirm: password },
  }, "", "application/json");
}

export async function findBy(table, column, value, token) {
  const query = encodeURIComponent(JSON.stringify([{ column, operator: "is", value }]));
  const response = await request("GET", `/api/${table}?query=${query}&page%5Bsize%5D=100`, undefined, token);
  return items(response).find((row) => String(attr(row, column)) === String(value)) || null;
}

export async function create(table, attributes, token, relationships) {
  const data = { type: table, attributes, ...(relationships ? { relationships } : {}) };
  const response = await request("POST", `/api/${table}`, { data }, token);
  return items(response)[0];
}

export async function patch(table, id, attributes, token, relationships) {
  const data = { type: table, id, attributes, ...(relationships ? { relationships } : {}) };
  const response = await request("PATCH", `/api/${table}/${id}`, { data }, token);
  return items(response)[0];
}

export async function upsert(table, column, value, attributes, token, relationships) {
  const existing = await findBy(table, column, value, token);
  return existing
    ? patch(table, existing.id, attributes, token, relationships)
    : create(table, { ...attributes, [column]: value }, token, relationships);
}
