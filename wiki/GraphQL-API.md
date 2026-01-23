# GraphQL API

Auto-generated GraphQL schema from your data model.

## Enabling GraphQL

GraphQL is disabled by default. Enable it:

```bash
# Via config API
curl -X POST http://localhost:6336/_config/backend/graphql.enable \
  -H "Authorization: Bearer $TOKEN" \
  -d 'true'

# Restart required
curl -X POST http://localhost:6336/action/world/restart_daptin \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"attributes": {}}'
```

Or via action:

```bash
curl -X POST http://localhost:6336/action/world/__enable_graphql \
  -H "Authorization: Bearer $TOKEN"
```

## Endpoint

```
POST http://localhost:6336/graphql
```

## Authentication

```bash
curl -X POST http://localhost:6336/graphql \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query": "{ ... }"}'
```

## Schema Generation

Daptin automatically generates:
- Types for each table
- Query types (single, list)
- Mutation types (create, update, delete)
- Relationship fields
- Input types

## Queries

### Get Single Record

```graphql
query {
  todo(id: "abc123") {
    id
    title
    completed
    created_at
  }
}
```

### List Records

```graphql
query {
  todos(page: 1, size: 10) {
    id
    title
    completed
  }
}
```

### With Filtering

```graphql
query {
  todos(filter: { completed: true }) {
    id
    title
  }
}
```

### With Relationships

```graphql
query {
  order(id: "order-123") {
    id
    total
    customer {
      id
      name
      email
    }
    items {
      id
      product {
        name
        price
      }
      quantity
    }
  }
}
```

### Nested Queries

```graphql
query {
  customers {
    id
    name
    orders {
      id
      total
      items {
        product {
          name
        }
      }
    }
  }
}
```

## Mutations

### Create

```graphql
mutation {
  createTodo(input: {
    title: "New task"
    completed: false
  }) {
    id
    title
    created_at
  }
}
```

### Update

```graphql
mutation {
  updateTodo(id: "abc123", input: {
    completed: true
  }) {
    id
    title
    completed
    updated_at
  }
}
```

### Delete

```graphql
mutation {
  deleteTodo(id: "abc123") {
    success
  }
}
```

### With Relationships

```graphql
mutation {
  createOrder(input: {
    customer_id: "cust-123"
    total: 99.99
  }) {
    id
    customer {
      name
    }
  }
}
```

## Type Definitions

Generated schema example:

```graphql
type Todo {
  id: ID!
  reference_id: String!
  title: String!
  completed: Boolean
  created_at: DateTime!
  updated_at: DateTime!
  user_account: UserAccount
}

type Query {
  todo(id: ID!): Todo
  todos(page: Int, size: Int, filter: TodoFilter): [Todo!]!
}

type Mutation {
  createTodo(input: CreateTodoInput!): Todo!
  updateTodo(id: ID!, input: UpdateTodoInput!): Todo!
  deleteTodo(id: ID!): DeleteResult!
}

input CreateTodoInput {
  title: String!
  completed: Boolean
}

input UpdateTodoInput {
  title: String
  completed: Boolean
}

input TodoFilter {
  completed: Boolean
  title: String
}
```

## Variables

```bash
curl -X POST http://localhost:6336/graphql \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "query GetTodo($id: ID!) { todo(id: $id) { title completed } }",
    "variables": { "id": "abc123" }
  }'
```

## Introspection

Get schema info:

```graphql
query {
  __schema {
    types {
      name
      fields {
        name
        type {
          name
        }
      }
    }
  }
}
```

Get type info:

```graphql
query {
  __type(name: "Todo") {
    name
    fields {
      name
      type {
        name
        kind
      }
    }
  }
}
```

## Actions via GraphQL

Execute actions:

```graphql
mutation {
  executeAction(
    entityType: "user_account"
    actionName: "signin"
    input: {
      email: "user@example.com"
      password: "password123"
    }
  ) {
    responseType
    attributes
  }
}
```

## JavaScript Client

```javascript
async function graphqlQuery(query, variables = {}) {
  const response = await fetch('http://localhost:6336/graphql', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${TOKEN}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ query, variables })
  });
  return response.json();
}

// Usage
const result = await graphqlQuery(`
  query GetTodos($completed: Boolean) {
    todos(filter: { completed: $completed }) {
      id
      title
    }
  }
`, { completed: false });

console.log(result.data.todos);
```

## Apollo Client Setup

```javascript
import { ApolloClient, InMemoryCache, createHttpLink } from '@apollo/client';
import { setContext } from '@apollo/client/link/context';

const httpLink = createHttpLink({
  uri: 'http://localhost:6336/graphql',
});

const authLink = setContext((_, { headers }) => ({
  headers: {
    ...headers,
    authorization: `Bearer ${TOKEN}`,
  }
}));

const client = new ApolloClient({
  link: authLink.concat(httpLink),
  cache: new InMemoryCache()
});
```

## Security Considerations

- GraphQL disabled by default
- Admin-only enable
- Respects Daptin permission model
- Query complexity not limited (potential DoS)
