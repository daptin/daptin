# Data conformations

Daptin uses the excellent [leebenson/conform](https://github.com/leebenson/conform) library to apply conformations on data before storing them in the database

- Conform: keep user input in check (go, golang)
- Trim, sanitize, and modify struct string fields in place, based on tags.

Use it for names, e-mail addresses, URL slugs, or any other form field where formatting matters.

Conform doesn't attempt any kind of validation on your fields.
