# Payment checkout example

This example composes Daptin resources, actions, permissions, integrations, and
metering into a Stripe-hosted subscription checkout. Daptin remains the only
application backend. Stripe is an attached payment provider; its secret never
enters the browser.

The workflow is intentionally split at durable boundaries:

```text
buyer -> prepare_checkout -> owned checkout_attempt
      -> begin_checkout -> SWITCH_USER -> Stripe Checkout Session
      -> refresh_checkout -> retrieve provider truth -> api_member
```

`prepare_checkout` commits the customer-owned attempt before provider work.
`begin_checkout` uses that attempt reference as Stripe's idempotency key and
client reference. `refresh_checkout` ignores redirect claims, retrieves the
stored provider session with the service credential, verifies its identity,
amount, currency, mode, and paid state, and creates the plan membership once.

## Prerequisites

- Docker with Compose
- Node.js 20 or newer
- a Stripe test-mode secret key
- a recurring Stripe Price ID whose amount is USD 12.00

Use the latest published Daptin image. The payment flow depends on the current
switched-user integration credential behavior.

To exercise the wiring without creating a real Checkout Session, start the
optional Stripe mock profile and set `STRIPE_API_BASE_URL=http://stripe-mock:12111`.
The mock proves routing and authorization only; it cannot prove a paid
subscription or entitlement reconciliation.

## Start and provision

```sh
cp .env.example .env.local
# Set every replace_me/replace-with value.
npm install
docker compose --env-file .env.local up --wait
npm run setup
```

The first setup uploads the schema. Restart once and finish provisioning:

```sh
docker compose --env-file .env.local restart daptin
npm run setup
```

The setup script uses only Daptin actions and JSON:API resources. It creates the
three users on a fresh instance, claims the administrator, provisions the Stripe
secret through an administrator-only action as a credential owned by the payments service user, installs the minimal
OpenAPI integration, uploads the checkout schema, creates the plan, and locks
the generated provider actions against direct customer execution.

## Start checkout

Run the smoke scenario:

```sh
npm run verify
```

It signs in as the buyer, invokes `prepare_checkout`, invokes `begin_checkout`
on the resulting attempt, prints the hosted Checkout URL, and proves that the
buyer cannot call the generated Stripe operation with the service credential.

Build a sales catalog with the ordinary resource query
`[{"column":"archived_at","operator":"is empty","value":null}]`. To retire a
plan, PATCH its `archived_at` field to a timestamp. The checkout actions reject
that plan before creating a new attempt or contacting Stripe. PATCH the field
to `null` to restore it. Already-created provider sessions remain eligible for
verified reconciliation so their membership and billing history stay intact.

The same calls from a frontend are:

```http
POST /action/api_plan/prepare_checkout
Authorization: Bearer BUYER_TOKEN
Content-Type: application/json

{"attributes":{"api_plan_id":"PLAN_REFERENCE_ID"}}
```

Then:

```http
POST /action/checkout_attempt/begin_checkout
Authorization: Bearer BUYER_TOKEN
Content-Type: application/json

{"attributes":{"checkout_attempt_id":"CHECKOUT_ATTEMPT_REFERENCE_ID"}}
```

Redirect the browser only to the `checkout_url` returned by the second action.
Do not put the Stripe secret, service user reference, credential reference,
amount, currency, or Stripe Price ID in frontend state.

## Reconcile provider state

After Stripe redirects the browser, call:

```http
POST /action/checkout_attempt/refresh_checkout
Authorization: Bearer BUYER_TOKEN
Content-Type: application/json

{"attributes":{"checkout_attempt_id":"CHECKOUT_ATTEMPT_REFERENCE_ID"}}
```

This call does not trust the redirect. It retrieves the session identified by
the customer-owned attempt and grants membership only when all backend-derived
values match. The unique `checkout_entitlement.checkout_attempt_reference`
prevents a repeated refresh from creating another membership.

A production webhook or scheduled recovery task should invoke this same
`refresh_checkout` action for the stored attempt. It must not implement another
entitlement path. If a public webhook accepts a Stripe event ID, retrieve the
event or Checkout Session using the service credential before invoking the
terminal action.

## Frontend responsibility

The frontend has four jobs:

1. invoke the two checkout actions;
2. redirect to the returned Stripe URL;
3. invoke or poll `refresh_checkout` after return;
4. render the owned `checkout_attempt` and resulting membership state.

It never decides whether payment succeeded and never creates `api_member`
directly.

## Production notes

- Pin the Daptin image by digest.
- Use a restricted Stripe key when the provider account supports it.
- Protect success and cancellation URLs with normal application session rules.
- Configure an authenticated provider callback or a bounded reconciliation
  task so payment completion does not depend on the customer returning.
- Model renewal, cancellation, refunds, disputes, tax, invoices, and proration
  as explicit provider events and Daptin actions before selling a production
  subscription.
- Keep payment authorization, Daptin permissions, plan membership, metering,
  and provider credentials as separate decisions.

For the full contract and security reasoning, see
[`wiki/Payments-and-Checkout.md`](../../wiki/Payments-and-Checkout.md).
