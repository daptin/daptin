import jsonapi_requests

api = jsonapi_requests.Api.config({
    'API_ROOT': 'http://localhost:6336/api',
    'AUTH': ('basic_auth_login', 'basic_auth_password'),
    'VALIDATE_SSL': False,
    'TIMEOUT': 1,
    "RETRIES": 0
})

endpoint = api.endpoint('user')
response = endpoint.get()

for profile in response.data:
    print(profile.attributes['name'])

endpoint = api.endpoint('post')

res = endpoint.post(
    object=jsonapi_requests.JsonApiObject(attributes={'title': 'post1', 'content': 'some.domain.pl'}, type='post'))
print(res.data)
