# Graphql

By default the GraphQL endpoint is not enabled. If you want to use GraphQL endpoint, here is how you can enable it.

Set ```graphql.enable``` to ```true``` in config:

```bash
curl \
-H "Authorization: Bearer TOKEN" \
-X POST http://localhost:6336/_config/backend/graphql.enable --data true
```


You can try to GET it again to verify if it was set or not (in case token is invalid or not set)

```bash
curl \
-H "Authorization: Bearer TOKEN" \
http://localhost:6336/_config/backend/graphql.enable
```

You need to restart daptin for this setting to take effect. You can issue a restart by calling this:

```bash
curl 'http://localhost:6336/action/world/restart_daptin' \
-H 'Authorization: Bearer TOKEN' \
--data '{"attributes":{}}'
```

If everything goes well, the graphql endpoint should be enabled. You can test it

```bash
curl http://localhost:6336/graphql
```

Response

```json
{
	"data": null,
	"errors": [
		{
			"message": "Must provide an operation.",
			"locations": []
		}
	]
}
```

You can access the iGraphQL console at http://localhost:6336/graphql