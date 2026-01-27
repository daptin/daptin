# Validation Tags Reference

Complete reference for validation tags you can use in Daptin schemas.

**Generated**: 2026-01-27
**Tested**: All tags verified against running API

---

## Overview

Daptin uses [validator.v9](https://github.com/go-playground/validator) for input validation. Validation tags are applied in your schema's `Validations` section to enforce data quality rules.

**Key Concepts:**
- Validation happens AFTER conformation (data transformation) - see [Conformation Reference](Conformation-Reference.md)
- Failed validation returns a 400 error with the specific tag that failed
- Use `omitempty` for optional fields that should skip validation when empty
- Combine multiple tags with commas: `"required,email"`

---

## Basic Validation Tags

### required

Field must not be empty.

**Syntax:**
```yaml
Validations:
  - ColumnName: username
    Tags: "required"
```

**Validation Rules:**
- Empty string (`""`) fails
- Null/missing field fails
- Any non-empty value passes

**Example:**
```yaml
Tables:
  - TableName: user_profile
    Columns:
      - Name: username
        DataType: varchar(100)
        ColumnType: label
    Validations:
      - ColumnName: username
        Tags: "required"
```

**Test Results:**
- ✅ `"username": "john"` → Success
- ❌ `"username": ""` → Field validation 'username' failed on the 'required' tag
- ❌ Missing field → NOT NULL constraint failed

---

### omitempty

Skip validation if field is empty/null. Typically combined with other tags for optional fields.

**Syntax:**
```yaml
Validations:
  - ColumnName: email
    Tags: "omitempty,email"
```

**Usage:**
- For optional fields that have validation rules
- Allows field to be omitted entirely
- If provided, other tags are checked

**Example:**
```yaml
Tables:
  - TableName: contact
    Columns:
      - Name: email
        DataType: varchar(100)
        ColumnType: email
        IsNullable: true
    Validations:
      - ColumnName: email
        Tags: "omitempty,email"  # Optional, but must be valid email if provided
```

---

## Format Validation Tags

### email

Validates email address format.

**Syntax:**
```yaml
Validations:
  - ColumnName: email_field
    Tags: "email"
```

**Validation Rules:**
- Must contain `@` symbol
- Must have domain part after `@`
- Follows RFC 5322 email format

**Test Results:**
- ✅ `"user@example.com"` → Success
- ❌ `"notanemail"` → Field validation 'email_field' failed on the 'email' tag
- ❌ `"user@"` → Field validation 'email_field' failed on the 'email' tag

**Example:**
```yaml
Tables:
  - TableName: newsletter
    Columns:
      - Name: subscriber_email
        DataType: varchar(100)
        ColumnType: email
    Validations:
      - ColumnName: subscriber_email
        Tags: "required,email"
```

---

### url

Validates URL format.

**Syntax:**
```yaml
Validations:
  - ColumnName: website
    Tags: "url"
```

**Validation Rules:**
- Must include protocol (`http://` or `https://`)
- Must follow valid URL structure

**Test Results:**
- ✅ `"http://example.com"` → Success
- ✅ `"https://example.com/path?query=value"` → Success
- ❌ `"example.com"` → Field validation 'website' failed on the 'url' tag (missing protocol)
- ❌ `"not a url"` → Field validation 'website' failed on the 'url' tag

**Example:**
```yaml
Tables:
  - TableName: link
    Columns:
      - Name: website
        DataType: varchar(200)
        ColumnType: url
    Validations:
      - ColumnName: website
        Tags: "omitempty,url"
```

---

### iscolor

Validates color format (hex or rgb).

**Syntax:**
```yaml
Validations:
  - ColumnName: theme_color
    Tags: "iscolor"
```

**Validation Rules:**
- Hex short form: `#RGB` (e.g., `#f00`)
- Hex long form: `#RRGGBB` (e.g., `#ff0000`)
- RGB function: `rgb(R,G,B)` (e.g., `rgb(255,0,0)`)

**Test Results:**
- ✅ `"#f00"` → Success
- ✅ `"#ff0000"` → Success
- ✅ `"rgb(255,0,0)"` → Success
- ❌ `"notacolor"` → Field validation 'theme_color' failed on the 'iscolor' tag

**Example:**
```yaml
Tables:
  - TableName: product
    Columns:
      - Name: color
        DataType: varchar(20)
        ColumnType: color
    Validations:
      - ColumnName: color
        Tags: "required,iscolor"
```

---

## Geographic Validation Tags

### latitude

Validates latitude coordinates.

**Syntax:**
```yaml
Validations:
  - ColumnName: lat
    Tags: "latitude"
```

**Validation Rules:**
- Range: -90 to 90 (inclusive)
- Decimal values accepted

**Test Results:**
- ✅ `0` → Success
- ✅ `45.5` → Success
- ✅ `-90` → Success (South Pole)
- ✅ `90` → Success (North Pole)
- ❌ `91` → Field validation 'lat' failed on the 'latitude' tag
- ❌ `-91` → Field validation 'lat' failed on the 'latitude' tag

**Example:**
```yaml
Tables:
  - TableName: location
    Columns:
      - Name: lat
        DataType: float(11,7)
        ColumnType: location.latitude
      - Name: lng
        DataType: float(11,7)
        ColumnType: location.longitude
    Validations:
      - ColumnName: lat
        Tags: "required,latitude"
      - ColumnName: lng
        Tags: "required,longitude"
```

---

### longitude

Validates longitude coordinates.

**Syntax:**
```yaml
Validations:
  - ColumnName: lng
    Tags: "longitude"
```

**Validation Rules:**
- Range: -180 to 180 (inclusive)
- Decimal values accepted

**Test Results:**
- ✅ `0` → Success (Prime Meridian)
- ✅ `120.5` → Success
- ✅ `-180` → Success (Antimeridian)
- ✅ `180` → Success (Antimeridian)
- ❌ `181` → Field validation 'lng' failed on the 'longitude' tag
- ❌ `-181` → Field validation 'lng' failed on the 'longitude' tag

---

## Numeric Comparison Tags

### min=X

Minimum value (inclusive).

**Syntax:**
```yaml
Validations:
  - ColumnName: quantity
    Tags: "min=1"
```

**Validation Rules:**
- Value must be >= X
- Works with integers and floats

**Test Results (min=10):**
- ❌ `5` → Field validation failed on the 'min' tag
- ✅ `10` → Success (boundary)
- ✅ `50` → Success
- ✅ `100` → Success

**Example:**
```yaml
Tables:
  - TableName: order
    Columns:
      - Name: quantity
        DataType: int(11)
        ColumnType: measurement
    Validations:
      - ColumnName: quantity
        Tags: "required,min=1"  # At least 1 item
```

---

### max=X

Maximum value (inclusive).

**Syntax:**
```yaml
Validations:
  - ColumnName: age
    Tags: "max=120"
```

**Validation Rules:**
- Value must be <= X
- Works with integers and floats

**Test Results (max=100):**
- ✅ `50` → Success
- ✅ `100` → Success (boundary)
- ❌ `150` → Field validation failed on the 'max' tag

**Example:**
```yaml
Tables:
  - TableName: user_profile
    Columns:
      - Name: age
        DataType: int(11)
        ColumnType: measurement
    Validations:
      - ColumnName: age
        Tags: "omitempty,min=0,max=150"
```

---

### gt=X

Greater than (exclusive).

**Syntax:**
```yaml
Validations:
  - ColumnName: score
    Tags: "gt=0"
```

**Validation Rules:**
- Value must be > X (NOT equal)
- Works with integers and floats

**Example:**
```yaml
Tables:
  - TableName: rating
    Columns:
      - Name: score
        DataType: float(5,2)
        ColumnType: measurement
    Validations:
      - ColumnName: score
        Tags: "required,gt=0,lte=5"  # Between 0 (exclusive) and 5 (inclusive)
```

---

### gte=X

Greater than or equal.

**Syntax:**
```yaml
Validations:
  - ColumnName: price
    Tags: "gte=0"
```

**Validation Rules:**
- Value must be >= X
- Works with integers and floats

**Test Results (gte=5):**
- ❌ `4` → Field validation failed on the 'gte' tag
- ✅ `5` → Success (boundary)
- ✅ `99` → Success

**Example:**
```yaml
Tables:
  - TableName: product
    Columns:
      - Name: price
        DataType: decimal(10,2)
        ColumnType: measurement
    Validations:
      - ColumnName: price
        Tags: "required,gte=0"  # Price cannot be negative
```

---

### lt=X

Less than (exclusive).

**Syntax:**
```yaml
Validations:
  - ColumnName: percentage
    Tags: "lt=100"
```

**Validation Rules:**
- Value must be < X (NOT equal)
- Works with integers and floats

**Test Results (lt=100):**
- ✅ `99` → Success
- ❌ `100` → Field validation failed on the 'lt' tag

**Example:**
```yaml
Tables:
  - TableName: discount
    Columns:
      - Name: percentage
        DataType: int(11)
        ColumnType: measurement
    Validations:
      - ColumnName: percentage
        Tags: "required,gte=0,lt=100"  # 0 to 99
```

---

### lte=X

Less than or equal.

**Syntax:**
```yaml
Validations:
  - ColumnName: rating
    Tags: "lte=5"
```

**Validation Rules:**
- Value must be <= X
- Works with integers and floats

**Test Results (lte=50):**
- ✅ `1` → Success
- ✅ `50` → Success (boundary)
- ❌ `51` → Field validation failed on the 'lte' tag

**Example:**
```yaml
Tables:
  - TableName: review
    Columns:
      - Name: stars
        DataType: int(11)
        ColumnType: measurement
    Validations:
      - ColumnName: stars
        Tags: "required,gte=1,lte=5"  # 1 to 5 stars
```

---

## String Validation Tags

### len=X

Exact length validation.

**Syntax:**
```yaml
Validations:
  - ColumnName: postal_code
    Tags: "len=5"
```

**Validation Rules:**
- String length must be exactly X characters
- Counts characters, not bytes

**Test Results (len=5):**
- ❌ `"abcd"` (4 chars) → Field validation failed on the 'len' tag
- ✅ `"abcde"` (5 chars) → Success
- ❌ `"abcdef"` (6 chars) → Field validation failed on the 'len' tag

**Example:**
```yaml
Tables:
  - TableName: address
    Columns:
      - Name: zip_code
        DataType: varchar(5)
        ColumnType: label
    Validations:
      - ColumnName: zip_code
        Tags: "required,len=5"  # US ZIP code
```

---

### oneof=A B C

Value must be one of the specified options (space-separated).

**Syntax:**
```yaml
Validations:
  - ColumnName: status
    Tags: "oneof=draft published archived"
```

**Validation Rules:**
- Value must exactly match one of the options
- Options are space-separated
- Case-sensitive

**Test Results (oneof=red green blue):**
- ✅ `"red"` → Success
- ✅ `"green"` → Success
- ✅ `"blue"` → Success
- ❌ `"yellow"` → Field validation failed on the 'oneof' tag

**Example:**
```yaml
Tables:
  - TableName: article
    Columns:
      - Name: status
        DataType: varchar(20)
        ColumnType: label
    Validations:
      - ColumnName: status
        Tags: "required,oneof=draft published archived"
```

---

## Combining Tags

Multiple validation tags can be combined with commas.

**Common Patterns:**

### Optional Email
```yaml
Validations:
  - ColumnName: email
    Tags: "omitempty,email"
```

### Bounded Number
```yaml
Validations:
  - ColumnName: percentage
    Tags: "required,gte=0,lte=100"
```

### Year Range
```yaml
Validations:
  - ColumnName: birth_year
    Tags: "required,min=1900,max=2100"
```

### Price Validation
```yaml
Validations:
  - ColumnName: price
    Tags: "required,gt=0"  # Must be positive
```

---

## Complete Example

```yaml
Tables:
  - TableName: product
    Columns:
      - Name: name
        DataType: varchar(200)
        ColumnType: name
      - Name: price
        DataType: decimal(10,2)
        ColumnType: measurement
      - Name: quantity
        DataType: int(11)
        ColumnType: measurement
      - Name: category
        DataType: varchar(50)
        ColumnType: label
      - Name: website
        DataType: varchar(200)
        ColumnType: url
        IsNullable: true
      - Name: color
        DataType: varchar(20)
        ColumnType: color
        IsNullable: true
    Validations:
      - ColumnName: name
        Tags: "required"
      - ColumnName: price
        Tags: "required,gt=0"
      - ColumnName: quantity
        Tags: "required,gte=0"
      - ColumnName: category
        Tags: "required,oneof=electronics clothing food"
      - ColumnName: website
        Tags: "omitempty,url"
      - ColumnName: color
        Tags: "omitempty,iscolor"
```

---

## Error Messages

When validation fails, Daptin returns a 400 error with this format:

```json
{
  "errors": [{
    "status": "400",
    "title": "Key: '' Error:Field validation 'field_name' failed on the 'tag_name' tag"
  }]
}
```

**Examples:**
- Email validation: `Field validation 'email' failed on the 'email' tag`
- Required field: `Field validation 'username' failed on the 'required' tag`
- Min value: `Field validation 'price' failed on the 'min' tag`

---

## Additional Validation Tags

The validator.v9 library supports many additional tags. Common ones include:

| Tag | Description | Example |
|-----|-------------|---------|
| `eq=X` | Equals value | `"eq=5"` |
| `ne=X` | Not equals value | `"ne=0"` |
| `alpha` | Alphabetic characters only | `"alpha"` |
| `alphanum` | Alphanumeric characters only | `"alphanum"` |
| `numeric` | Numeric characters only | `"numeric"` |
| `base64` | Valid base64 encoding | `"base64"` |
| `contains=X` | Contains substring | `"contains=test"` |
| `excludes=X` | Does not contain substring | `"excludes=admin"` |
| `startswith=X` | Starts with prefix | `"startswith=user_"` |
| `endswith=X` | Ends with suffix | `"endswith=.com"` |

See [validator.v9 documentation](https://github.com/go-playground/validator/tree/v9.31.0) for complete list.

---

## Testing Your Validations

**Always test validation tags before deploying:**

1. Create test table with validation rules
2. Try valid values (should succeed)
3. Try invalid values (should fail with specific error)
4. Check error messages match expected tag

**Example Test Script:**
```bash
# Valid data - should succeed
curl -X POST http://localhost:6336/api/product \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "product",
      "attributes": {
        "name": "Laptop",
        "price": 999.99,
        "quantity": 10,
        "category": "electronics"
      }
    }
  }'

# Invalid price - should fail
curl -X POST http://localhost:6336/api/product \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "product",
      "attributes": {
        "name": "Laptop",
        "price": 0,
        "quantity": 10,
        "category": "electronics"
      }
    }
  }'
# Expected error: Field validation 'price' failed on the 'gt' tag
```

---

## Related Documentation

- [Conformation Reference](Conformation-Reference.md) - Data transformation before validation
- [Custom Actions](Custom-Actions.md) - Using validations with actions
- [Schema Definition](Schema-Definition.md) - Complete schema reference
- [Column Types](Column-Types.md) - Available column types
