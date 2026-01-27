# Conformation Tags Reference

Complete reference for conformation (data transformation) tags you can use in Daptin schemas.

**Generated**: 2026-01-27
**Tested**: All tags verified against conform library

---

## Overview

Daptin uses [conform library](https://github.com/artpar/conform) for data transformations. Conformation tags are applied in your schema's `Conformations` section to automatically transform input data.

**Key Concepts:**
- Conformations transform data before validation - see [Validation Reference](Validation-Reference.md) for validation tags
- Multiple tags can be combined with commas: `"trim,lower"`
- Tags are applied in left-to-right order
- Works with string data only

**Important:** Conformations are applied in the `InterceptBefore` middleware during POST/PATCH requests.

---

## String Manipulation Tags

### trim

Remove leading and trailing whitespace.

**Syntax:**
```yaml
Conformations:
  - ColumnName: username
    Tags: "trim"
```

**Transformation:**
- `"  hello world  "` → `"hello world"`
- `"  text  "` → `"text"`
- `"nowhitespace"` → `"nowhitespace"`

**Example:**
```yaml
Tables:
  - TableName: user_account
    Columns:
      - Name: username
        DataType: varchar(100)
        ColumnType: label
    Conformations:
      - ColumnName: username
        Tags: "trim"
```

**Use Cases:**
- Remove accidental whitespace from user input
- Clean form data
- Normalize identifiers

---

### ltrim

Remove leading (left-side) whitespace only.

**Syntax:**
```yaml
Conformations:
  - ColumnName: text_field
    Tags: "ltrim"
```

**Transformation:**
- `"  hello world  "` → `"hello world  "`
- `"  text"` → `"text"`

---

### rtrim

Remove trailing (right-side) whitespace only.

**Syntax:**
```yaml
Conformations:
  - ColumnName: text_field
    Tags: "rtrim"
```

**Transformation:**
- `"  hello world  "` → `"  hello world"`
- `"text  "` → `"text"`

---

### lower

Convert entire string to lowercase.

**Syntax:**
```yaml
Conformations:
  - ColumnName: tag
    Tags: "lower"
```

**Transformation:**
- `"HELLO WORLD"` → `"hello world"`
- `"Hello World"` → `"hello world"`
- `"HeLLo"` → `"hello"`

**Example:**
```yaml
Tables:
  - TableName: article
    Columns:
      - Name: tag
        DataType: varchar(50)
        ColumnType: label
    Conformations:
      - ColumnName: tag
        Tags: "trim,lower"  # Trim then lowercase
```

**Use Cases:**
- Case-insensitive identifiers
- Tags and categories
- Search terms

---

### upper

Convert entire string to uppercase.

**Syntax:**
```yaml
Conformations:
  - ColumnName: code
    Tags: "upper"
```

**Transformation:**
- `"hello world"` → `"HELLO WORLD"`
- `"Hello World"` → `"HELLO WORLD"`
- `"hello"` → `"HELLO"`

**Example:**
```yaml
Tables:
  - TableName: product
    Columns:
      - Name: sku
        DataType: varchar(50)
        ColumnType: label
    Conformations:
      - ColumnName: sku
        Tags: "trim,upper"  # SKU codes in uppercase
```

**Use Cases:**
- Product codes/SKUs
- Country codes
- Constants

---

### title

Convert string to title case (capitalize first letter of each word).

**Syntax:**
```yaml
Conformations:
  - ColumnName: headline
    Tags: "title"
```

**Transformation:**
- `"hello world"` → `"Hello World"`
- `"HELLO WORLD"` → `"HELLO WORLD"` (already uppercase stays uppercase)
- `"hello-world"` → `"Hello-World"`

**Note:** Uses Go's `strings.Title()` which capitalizes after any space or punctuation.

**Example:**
```yaml
Tables:
  - TableName: article
    Columns:
      - Name: title
        DataType: varchar(200)
        ColumnType: name
    Conformations:
      - ColumnName: title
        Tags: "title"
```

---

### ucfirst

Capitalize only the first letter, leave rest unchanged.

**Syntax:**
```yaml
Conformations:
  - ColumnName: sentence
    Tags: "ucfirst"
```

**Transformation:**
- `"hello world"` → `"Hello world"`
- `"hello World"` → `"Hello World"` (only first letter changed)
- `"HELLO"` → `"HELLO"` (already uppercase)

**Example:**
```yaml
Tables:
  - TableName: note
    Columns:
      - Name: text
        DataType: text
        ColumnType: content
    Conformations:
      - ColumnName: text
        Tags: "trim,ucfirst"
```

---

## Case Conversion Tags

### camel

Convert to camelCase.

**Syntax:**
```yaml
Conformations:
  - ColumnName: identifier
    Tags: "camel"
```

**Transformation:**
- `"hello_world"` → `"helloWorld"`
- `"hello world"` → `"helloWorld"`
- `"HelloWorld"` → `"helloWorld"`
- `"HELLO_WORLD"` → `"helloWorld"`

**Example:**
```yaml
Tables:
  - TableName: api_key
    Columns:
      - Name: key_name
        DataType: varchar(100)
        ColumnType: label
    Conformations:
      - ColumnName: key_name
        Tags: "camel"
```

**Use Cases:**
- JavaScript property names
- API parameter names
- Internal identifiers

---

### snake

Convert to snake_case.

**Syntax:**
```yaml
Conformations:
  - ColumnName: field_name
    Tags: "snake"
```

**Transformation:**
- `"HelloWorld"` → `"hello_world"`
- `"hello world"` → `"hello_world"`
- `"helloWorld"` → `"hello_world"`
- `"HELLO_WORLD"` → `"hello_world"`

**Example:**
```yaml
Tables:
  - TableName: database_field
    Columns:
      - Name: column_name
        DataType: varchar(100)
        ColumnType: label
    Conformations:
      - ColumnName: column_name
        Tags: "snake"
```

**Use Cases:**
- Database column names
- Python variable names
- Internal identifiers

---

### slug

Convert to slug-case (kebab-case).

**Syntax:**
```yaml
Conformations:
  - ColumnName: url_slug
    Tags: "slug"
```

**Transformation:**
- `"HelloWorld"` → `"hello-world"`
- `"hello world"` → `"hello-world"`
- `"Hello World"` → `"hello-world"`

**Example:**
```yaml
Tables:
  - TableName: article
    Columns:
      - Name: slug
        DataType: varchar(200)
        ColumnType: label
    Conformations:
      - ColumnName: slug
        Tags: "slug"
```

**Use Cases:**
- URL slugs
- Friendly URLs
- CSS class names

---

## Specialized Formatting Tags

### name

Format as proper name (normalize whitespace, title case).

**Syntax:**
```yaml
Conformations:
  - ColumnName: full_name
    Tags: "name"
```

**Transformation:**
- `"  john   doe  "` → `"John Doe"`
- `"jane smith"` → `"Jane Smith"`
- `"ALICE  WONDERLAND"` → `"Alice Wonderland"`

**Features:**
- Trims leading/trailing whitespace
- Normalizes multiple spaces to single space
- Capitalizes each word

**Example:**
```yaml
Tables:
  - TableName: contact
    Columns:
      - Name: full_name
        DataType: varchar(100)
        ColumnType: name
    Conformations:
      - ColumnName: full_name
        Tags: "name"
```

**Use Cases:**
- Person names
- Company names
- Display names

---

### email

Normalize email format (trim, lowercase domain, preserve local case).

**Syntax:**
```yaml
Conformations:
  - ColumnName: email
    Tags: "email"
```

**Transformation:**
- `"  USER@EXAMPLE.COM  "` → `"USER@example.com"`
- `"John.Doe@GMAIL.com"` → `"John.Doe@gmail.com"`
- `"test@TEST.COM"` → `"test@test.com"`

**Email Format Rules (RFC 5321):**
- Local part (before @): Case-sensitive, preserved
- Domain part (after @): Lowercased
- Whitespace trimmed

**Example:**
```yaml
Tables:
  - TableName: user_account
    Columns:
      - Name: email
        DataType: varchar(100)
        ColumnType: email
    Conformations:
      - ColumnName: email
        Tags: "email"
    Validations:
      - ColumnName: email
        Tags: "required,email"
```

**Use Cases:**
- User registration
- Contact forms
- Email normalization

---

## Character Filtering Tags

### num

Extract only numeric characters.

**Syntax:**
```yaml
Conformations:
  - ColumnName: phone
    Tags: "num"
```

**Transformation:**
- `"abc123def456"` → `"123456"`
- `"(555) 123-4567"` → `"5551234567"`
- `"Price: $99.99"` → `"9999"`

**Example:**
```yaml
Tables:
  - TableName: contact
    Columns:
      - Name: phone
        DataType: varchar(20)
        ColumnType: label
    Conformations:
      - ColumnName: phone
        Tags: "num"
```

**Use Cases:**
- Phone number cleanup
- Extract numeric values
- Remove formatting

---

### !num

Strip all numeric characters (keep everything else).

**Syntax:**
```yaml
Conformations:
  - ColumnName: text_only
    Tags: "!num"
```

**Transformation:**
- `"abc123def456"` → `"abcdef"`
- `"Product123"` → `"Product"`
- `"2024-01-27"` → `"--"`

**Use Cases:**
- Remove numbers from text
- Clean product names

---

### alpha

Extract only alphabetic characters.

**Syntax:**
```yaml
Conformations:
  - ColumnName: letters_only
    Tags: "alpha"
```

**Transformation:**
- `"abc123def"` → `"abcdef"`
- `"Hello123World!"` → `"HelloWorld"`
- `"Test@#$123"` → `"Test"`

**Use Cases:**
- Extract letters only
- Remove special characters and numbers

---

### !alpha

Strip all alphabetic characters (keep everything else).

**Syntax:**
```yaml
Conformations:
  - ColumnName: no_letters
    Tags: "!alpha"
```

**Transformation:**
- `"abc123def"` → `"123"`
- `"Hello123World!"` → `"123!"`
- `"Test@#$"` → `"@#$"`

**Use Cases:**
- Extract symbols and numbers
- Remove letter characters

---

## Escaping Tags

### !html

HTML escape string (prevent XSS).

**Syntax:**
```yaml
Conformations:
  - ColumnName: user_content
    Tags: "!html"
```

**Transformation:**
- `"<script>alert('XSS')</script>"` → `"&lt;script&gt;alert('XSS')&lt;/script&gt;"`
- `"Hello & goodbye"` → `"Hello &amp; goodbye"`
- `'Say "hello"'` → `'Say &#34;hello&#34;'`

**Example:**
```yaml
Tables:
  - TableName: comment
    Columns:
      - Name: content
        DataType: text
        ColumnType: content
    Conformations:
      - ColumnName: content
        Tags: "trim,!html"  # Trim then escape
```

**Use Cases:**
- User-generated content
- Prevent XSS attacks
- Safe HTML display

---

### !js

JavaScript escape string.

**Syntax:**
```yaml
Conformations:
  - ColumnName: js_string
    Tags: "!js"
```

**Transformation:**
- Escapes special characters for safe JavaScript strings
- Handles quotes, backslashes, newlines

**Use Cases:**
- Embedding strings in JavaScript
- Prevent JavaScript injection
- Safe script generation

---

## Combining Tags

Multiple conformation tags can be combined with commas. Tags are applied left-to-right.

### Common Patterns

#### Email Normalization
```yaml
Conformations:
  - ColumnName: email
    Tags: "trim,lower"
```

**Transformation:**
- `"  USER@EXAMPLE.COM  "` → `"user@example.com"`

---

#### Clean User Input
```yaml
Conformations:
  - ColumnName: username
    Tags: "trim,lower"
```

**Transformation:**
- `"  JohnDoe  "` → `"johndoe"`

---

#### Format Display Names
```yaml
Conformations:
  - ColumnName: display_name
    Tags: "trim,name"
```

**Transformation:**
- `"  john   doe  "` → `"John Doe"`

---

#### URL Slugs
```yaml
Conformations:
  - ColumnName: slug
    Tags: "trim,slug"
```

**Transformation:**
- `"  Hello World  "` → `"hello-world"`

---

#### Product SKU
```yaml
Conformations:
  - ColumnName: sku
    Tags: "trim,upper,num"
```

**Transformation:**
- `"  abc-123-def  "` → `"123"` (trim → uppercase → extract numbers)

---

#### Phone Numbers
```yaml
Conformations:
  - ColumnName: phone
    Tags: "num"
```

**Transformation:**
- `"(555) 123-4567"` → `"5551234567"`

---

## Complete Example

```yaml
Tables:
  - TableName: user_registration
    Columns:
      - Name: email
        DataType: varchar(100)
        ColumnType: email
      - Name: full_name
        DataType: varchar(100)
        ColumnType: name
      - Name: username
        DataType: varchar(50)
        ColumnType: label
      - Name: phone
        DataType: varchar(20)
        ColumnType: label
      - Name: bio
        DataType: text
        ColumnType: content
    Conformations:
      - ColumnName: email
        Tags: "email"              # Normalize email
      - ColumnName: full_name
        Tags: "name"                # Format as proper name
      - ColumnName: username
        Tags: "trim,lower"          # Lowercase username
      - ColumnName: phone
        Tags: "num"                 # Extract only numbers
      - ColumnName: bio
        Tags: "trim,!html"          # Trim and escape HTML
    Validations:
      - ColumnName: email
        Tags: "required,email"
      - ColumnName: full_name
        Tags: "required"
      - ColumnName: username
        Tags: "required,min=3"
```

**Input Data:**
```json
{
  "email": "  John.Doe@EXAMPLE.COM  ",
  "full_name": "  john   doe  ",
  "username": "  JohnDoe123  ",
  "phone": "(555) 123-4567",
  "bio": "  Hello & <welcome>!  "
}
```

**After Conformations:**
```json
{
  "email": "John.Doe@example.com",
  "full_name": "John Doe",
  "username": "johndoe123",
  "phone": "5551234567",
  "bio": "Hello &amp; &lt;welcome&gt;!"
}
```

---

## Execution Order

**Important:** Conformations are applied in the `InterceptBefore` middleware when processing POST and PATCH requests.

**Order of Operations:**
1. Request received
2. `InterceptBefore` called
3. For each object:
   - Conformations applied (data transformed)
   - Validations checked (on transformed data)
4. Data saved to database

---

## Column Type Auto-Conformations

Some column types automatically apply conformations:

| Column Type | Auto Conformation |
|-------------|-------------------|
| `email` | `email` (trim + lowercase domain) |
| `name` | `name` (format as proper name) |
| `bcrypt` | `bcrypt` (hash password) |

**Example:**
```yaml
Columns:
  - Name: email
    ColumnType: email  # Automatically applies email conformation
```

You don't need to specify these conformations explicitly for these column types.

---

## Custom Sanitizers

The conform library supports custom sanitizers via `AddSanitizer(key string, sanitizer)`. Daptin can extend conformations with custom transformations.

---

## Tag Reference Table

| Tag | Description | Example Input | Example Output |
|-----|-------------|---------------|----------------|
| `trim` | Remove leading/trailing whitespace | `"  hello  "` | `"hello"` |
| `ltrim` | Remove leading whitespace | `"  hello"` | `"hello"` |
| `rtrim` | Remove trailing whitespace | `"hello  "` | `"hello"` |
| `lower` | Convert to lowercase | `"HELLO"` | `"hello"` |
| `upper` | Convert to uppercase | `"hello"` | `"HELLO"` |
| `title` | Title case | `"hello world"` | `"Hello World"` |
| `ucfirst` | Capitalize first letter | `"hello"` | `"Hello"` |
| `camel` | Convert to camelCase | `"hello_world"` | `"helloWorld"` |
| `snake` | Convert to snake_case | `"HelloWorld"` | `"hello_world"` |
| `slug` | Convert to slug-case | `"Hello World"` | `"hello-world"` |
| `name` | Format as proper name | `"  john  doe  "` | `"John Doe"` |
| `email` | Normalize email | `"USER@TEST.COM"` | `"USER@test.com"` |
| `num` | Extract only numbers | `"abc123"` | `"123"` |
| `!num` | Strip numbers | `"abc123"` | `"abc"` |
| `alpha` | Extract only letters | `"abc123"` | `"abc"` |
| `!alpha` | Strip letters | `"abc123"` | `"123"` |
| `!html` | HTML escape | `"<div>"` | `"&lt;div&gt;"` |
| `!js` | JavaScript escape | Special chars escaped | Safe JS string |

---

## Related Documentation

- [Validation Reference](Validation-Reference.md) - Input validation tags (applied after conformations)
- [Custom Actions](Custom-Actions.md) - Using conformations with actions
- [Schema Definition](Schema-Definition.md) - Complete schema reference
- [Column Types](Column-Types.md) - Column types with auto-conformations
