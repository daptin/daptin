import { action, config, findBy, request, requireConfig, signIn } from "./lib.mjs";

requireConfig();
const adminToken = await signIn(config.adminEmail, config.adminPassword);
const buyerToken = await signIn(config.buyerEmail, config.buyerPassword);
const plan = await findBy("api_plan", "name", "Checkout Pro", adminToken);
if (!plan) throw new Error("Checkout Pro plan is missing; run npm run setup");

const prepared = await action("api_plan", "prepare_checkout", { api_plan_id: plan.id }, buyerToken);
const preparedResult = prepared.find((entry) => entry.ResponseType === "client.return")?.Attributes;
if (!preparedResult?.checkout_attempt_id) throw new Error("prepare_checkout did not return checkout_attempt_id");

const attemptId = preparedResult.checkout_attempt_id;
const begun = await action("checkout_attempt", "begin_checkout", { checkout_attempt_id: attemptId }, buyerToken);
const beginResult = begun.find((entry) => entry.ResponseType === "client.return")?.Attributes;
if (!beginResult?.provider_session_id) throw new Error("begin_checkout did not return a provider session ID");
if (config.stripeApiBaseUrl === "https://api.stripe.com" &&
    !String(beginResult.checkout_url).startsWith("https://checkout.stripe.com/")) {
  throw new Error("Stripe did not return a hosted checkout URL");
}

const credential = await findBy("credential", "name", "stripe-checkout-service", adminToken);
const direct = await request("POST", "/action/integration/stripe_checkout/createCheckoutSession", {
  attributes: { credential_id: credential.id },
}, buyerToken, "application/json", true);
if (direct.ok) throw new Error("buyer unexpectedly executed the generated provider action");

let mockReconciliation = null;
if (config.stripeApiBaseUrl !== "https://api.stripe.com") {
  await action("checkout_attempt", "refresh_checkout", { checkout_attempt_id: attemptId }, buyerToken);
  const entitlement = await findBy("checkout_entitlement", "checkout_attempt_reference", attemptId, adminToken);
  if (entitlement) throw new Error("unverified mock provider state unexpectedly created an entitlement");
  mockReconciliation = "unverified provider state denied";
}

console.log(JSON.stringify({
  checkout_attempt_id: attemptId,
  provider_session_id: beginResult.provider_session_id,
  checkout_url: beginResult.checkout_url === "<nil>" ? null : beginResult.checkout_url,
  direct_service_credential_status: direct.status,
  mock_reconciliation: mockReconciliation,
}, null, 2));
console.log("[verify] pay in Stripe test mode, then invoke refresh_checkout as described in README.md");
