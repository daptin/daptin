# Payments and Checkout

Daptin does not contain a second payment or subscription subsystem. A payment
flow composes the authorities already present:

```text
user_account + api_plan + actions + credential + integration
              -> checkout_attempt -> verified provider state -> api_member
```

The runnable Stripe test-mode example lives in
[`examples/payment-checkout`](https://github.com/daptin/daptin/tree/master/examples/payment-checkout).
Run it with the latest published Daptin image.

## Contract

A permitted customer selects a backend-defined plan. Daptin records an owned
checkout attempt before contacting the provider, uses a private service-account
credential to create the provider session, and activates plan membership only
after retrieving and verifying provider state.

Four decisions remain independent:

| Decision | Authority |
|---|---|
| Who is buying | authenticated `user_account` and checkout row owner |
| Who may start or refresh checkout | world, row, and action permissions |
| Which provider secret may be used | service-user-owned `credential` |
| What the customer may consume | `api_plan`, `api_member`, metering, and quotas |

Permission to execute checkout never grants direct credential access. A paid
provider session never bypasses Daptin plan membership or resource permission.

## Durable Workflow

Use three named actions rather than trusting a redirect or putting provider
logic in a frontend:

1. `prepare_checkout` runs as the authenticated buyer. It loads the selected
   `api_plan` row and creates a customer-owned `checkout_attempt` containing the
   backend price, currency, and provider Price ID.
2. `begin_checkout` runs on that attempt, switches to a fixed payments service
   user, resolves that user's credential by a backend-fixed name, and invokes
   the installed Stripe integration.
   The attempt reference becomes the provider idempotency and client reference.
3. `refresh_checkout` retrieves the stored provider session with the same
   service credential. It verifies the client reference, mode, amount,
   currency, and paid status before creating one `checkout_entitlement` and one
   `api_member` owned by the buyer.

The durable attempt exists before the provider call. A retry therefore uses the
same provider idempotency key rather than creating an unrelated checkout.

## Why the Browser Redirect Is Not Payment Proof

Success and cancellation URLs are navigation. They are not authorization and
must not activate a plan. Query parameters, posted status values, customer IDs,
and provider session IDs are caller input.

`refresh_checkout` takes only the Daptin checkout-attempt reference. It reads
the provider session ID from that owned row and retrieves the session through
the installed integration. A mismatch or unavailable provider response fails
closed before membership is created.

Webhooks and scheduled recovery use this same action. A webhook may locate the
attempt, but it must not implement a separate entitlement path or treat its
payload as sufficient proof.

## Run the Stripe Test-Mode Example

Create a recurring USD 12.00 Stripe test Price, obtain its Price ID and a test
secret key, then:

```bash
cd examples/payment-checkout
cp .env.example .env.local
# Fill the passwords, STRIPE_SECRET_KEY, and STRIPE_PRICE_ID.
npm install
docker compose --env-file .env.local up --wait
npm run setup
```

The first run uploads the schema. Restart Daptin and finish provisioning:

```bash
docker compose --env-file .env.local restart daptin
npm run setup
npm run verify
```

The scripts provision through Daptin APIs and actions only. They do not write
the SQL database directly.

## Frontend Calls

After sign-in, prepare an attempt using the plan reference:

```bash
curl -X POST http://localhost:6336/action/api_plan/prepare_checkout \
  -H "Authorization: Bearer $BUYER_TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary '{"attributes":{"api_plan_id":"PLAN_REFERENCE_ID"}}'
```

Use the returned attempt reference to create or recover the provider session:

```bash
curl -X POST http://localhost:6336/action/checkout_attempt/begin_checkout \
  -H "Authorization: Bearer $BUYER_TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary '{"attributes":{"checkout_attempt_id":"CHECKOUT_ATTEMPT_REFERENCE_ID"}}'
```

Redirect the browser to the returned `checkout_url`. After the provider returns,
refresh from provider truth:

```bash
curl -X POST http://localhost:6336/action/checkout_attempt/refresh_checkout \
  -H "Authorization: Bearer $BUYER_TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary '{"attributes":{"checkout_attempt_id":"CHECKOUT_ATTEMPT_REFERENCE_ID"}}'
```

The frontend may poll its owned `checkout_attempt` row or subscribe to resource
events. It must never receive or submit the service user, provider secret,
credential reference, price, currency, provider Price ID, or membership owner.

## Permissions

Grant customers only the gates required to:

- read and execute enabled `api_plan` rows;
- create/read/update/execute their own `checkout_attempt` rows;
- execute the three wrapper actions.

Set the generated Stripe integration actions to no broad execute permission.
The wrapper definition contains the fixed service user and credential name and
performs `SWITCH_USER` before resolving the credential and invoking the
integration. Neither value is caller input.

The service credential remains owner-readable by the service user and
administrators. Do not add it to an ordinary customer group merely to make
checkout work.

See [[Permissions]], [[Credentials]], and
[[Integrations#run-an-integration-as-a-service-account]] for the individual
gates.

## Idempotency and Terminalization

The example uses two durable uniqueness boundaries:

- provider session creation uses the checkout-attempt reference as Stripe's
  `Idempotency-Key`;
- `checkout_entitlement.checkout_attempt_reference` and provider session ID are
  unique before `api_member` creation.

Successful entitlement creation and membership creation occur in the owning
Daptin action transaction. Duplicate refreshes observe the entitlement and do
not create another membership. Provider timeouts leave the attempt pending so
the same action can retrieve or retry it.

Do not keep a SQL transaction open across an unbounded stream. Checkout create
and retrieve calls must use bounded provider timeouts. Long-running recovery
belongs in a persisted task that invokes the same refresh action.

## Statuses

Treat status as explicit data. At minimum model:

| Local state | Meaning |
|---|---|
| `prepared` | attempt is durable; provider session not yet recorded |
| `provider_created` | hosted checkout URL is available |
| `open` / `unpaid` | provider has not confirmed payment |
| `complete` / `paid` | verified and eligible for terminalization |
| `expired` | provider session can no longer complete |
| `cancelled` | application or provider cancelled the attempt |
| `failed` | bounded provider or validation failure requiring review/retry |

Only the verified `complete` and `paid` combination with matching backend data
may create entitlement.

## Renewals, Cancellation, and Refunds

Initial checkout is only the first subscription event. Before production,
define separate named actions for renewals, payment failures, subscription
cancellation, refunds, and disputes. Each handler must retrieve provider truth,
locate the existing Daptin membership through persisted references, and update
that membership through normal resource APIs.

Do not infer ongoing entitlement merely from the existence of a historical
successful payment. The current `api_member.status` and period remain the
durable Daptin authority used by metering.

## Operational Checklist

- Use PostgreSQL or MySQL/MariaDB in production.
- Pin Daptin and the provider API contract.
- Protect and rotate the Daptin encryption secret and Stripe key.
- Use provider-restricted credentials where available.
- Configure bounded timeouts and persisted reconciliation tasks.
- Monitor pending/failed attempts and held metering reservations.
- Test duplicated, reordered, forged, mismatched, expired, cancelled, refunded,
  and disputed provider events.
- Confirm denied calls never reach Stripe.
- Confirm checkout rows and caller-facing metering belong to the buyer.

Related guides: [[API-Metering]], [[Authorization-Scenarios]],
[[Custom-Actions]], [[Integrations]], and [[Task-Scheduling]].
