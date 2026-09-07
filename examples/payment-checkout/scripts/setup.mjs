import fs from "node:fs";
import path from "node:path";
import YAML from "yaml";
import {
  action, config, findBy, patch, request, requireConfig, root, signIn, signUp, upsert,
} from "./lib.mjs";

requireConfig();

await signUp(config.adminEmail, config.adminPassword, "Daptin Administrator");
await signUp(config.serviceEmail, config.servicePassword, "Payments Service");
await signUp(config.buyerEmail, config.buyerPassword, "Checkout Buyer");

let adminToken = await signIn(config.adminEmail, config.adminPassword);
await request("POST", "/action/world/become_an_administrator", { attributes: {} }, adminToken, "application/json", true);
await new Promise((resolve) => setTimeout(resolve, 2500));
adminToken = await signIn(config.adminEmail, config.adminPassword);
const serviceUser = await findBy("user_account", "email", config.serviceEmail, adminToken);
if (!serviceUser) throw new Error("payments service user was not found");

const specificationObject = JSON.parse(fs.readFileSync(path.join(root, "openapi", "stripe-checkout.json"), "utf8"));
specificationObject.servers[0].url = config.stripeApiBaseUrl;
const providerUrl = new URL(config.stripeApiBaseUrl);
const specification = JSON.stringify(specificationObject);
const integration = await upsert("integration", "name", "stripe_checkout", {
  title: "Stripe Checkout",
  specification,
  specification_language: "openapiv3",
  specification_format: "json",
  authentication_type: "custom_credentials",
  authentication_specification: JSON.stringify({ scheme: "bearer", token_field: "secret_key" }),
  allow_insecure: providerUrl.protocol !== "https:",
  allow_private_network: !["api.stripe.com"].includes(providerUrl.hostname),
  enable: true,
}, adminToken);
await action("integration", "install_integration", { integration_id: integration.id }, adminToken);

const checkoutWorld = await findBy("world", "table_name", "checkout_attempt", adminToken);
if (!checkoutWorld) {
  const source = fs.readFileSync(path.join(root, "schemas", "schema_payment_checkout.yaml"), "utf8")
    .replaceAll("__PAYMENTS_SERVICE_USER_REFERENCE_ID__", serviceUser.id)
    .replaceAll("__CHECKOUT_SUCCESS_URL__", config.successUrl)
    .replaceAll("__CHECKOUT_CANCEL_URL__", config.cancelUrl);
  const schema = YAML.parse(source);
  if (!schema.Tables || !schema.Actions) throw new Error("checkout schema is missing tables or actions");
  const upload = await request("POST", "/action/world/upload_system_schema", {
    attributes: {
      schema_file: [{
        name: "schema_payment_checkout.yaml",
        file: `data:text/yaml;base64,${Buffer.from(source).toString("base64")}`,
      }],
    },
  }, adminToken, "application/json", true);
  const notified = Array.isArray(upload.payload) && upload.payload.some((entry) =>
    entry.ResponseType === "client.notify" && entry.Attributes?.type === "success");
  // Older images can write the schema and then report 503 while rendering the
  // success notification. Current builds return the notification normally.
  const olderImageWroteFile = upload.status === 503 && JSON.stringify(upload.payload)
    .includes("action performer [client.notify] is not available");
  if (!notified && !olderImageWroteFile) {
    throw new Error(`schema upload was not accepted: ${JSON.stringify(upload.payload)}`);
  }
  console.log("[setup] schema uploaded; restart Daptin and run npm run setup again");
  process.exit(0);
}

let credential = await findBy("credential", "name", "stripe-checkout-service", adminToken);
if (!credential) {
  const provisioned = await action("credential", "provision_stripe_checkout_credential", {
    credential_content: JSON.stringify({ secret_key: config.stripeSecretKey }),
  }, adminToken);
  const record = provisioned.find((entry) => entry.ResponseType === "credential")?.Attributes;
  if (!record?.reference_id) throw new Error("credential provisioning action did not return a credential");
  credential = { id: record.reference_id };
} else {
  credential = await patch("credential", credential.id, {
    content: JSON.stringify({ secret_key: config.stripeSecretKey }),
  }, adminToken);
}

const plan = await upsert("api_plan", "name", "Checkout Pro", {
  price_monthly_cents: 1200,
  currency: "USD",
  stripe_price_id: config.stripePriceId,
  limits: JSON.stringify([{ metric: "requests", window: "month", maximum: 10000, mode: "hard" }]),
  metadata: JSON.stringify({ checkout_example: true }),
}, adminToken);

// Generated provider actions remain unavailable to customers. Product clients
// call only prepare_checkout, begin_checkout, and refresh_checkout.
for (const name of ["stripe_checkout/createCheckoutSession", "stripe_checkout/retrieveCheckoutSession"]) {
  const generated = await findBy("action", "action_name", name, adminToken);
  if (generated) await patch("action", generated.id, { permission: 0 }, adminToken);
}

console.log(JSON.stringify({
  plan_id: plan.id,
  integration_id: integration.id,
  service_user_id: serviceUser.id,
  credential_id: credential.id,
}, null, 2));
console.log("[setup] ready; sign in as the buyer and follow README.md");
