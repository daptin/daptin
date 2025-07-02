# OpenAPI Documentation Improvements Summary

## Changes Made to `/server/apiblueprint/apiblueprint.go`

### 1. Main API Description Enhancement
Added a comprehensive "Getting Started" section that explains:
- The 3-step initial setup process (signup → signin → become_an_administrator)
- **Critical Admin Bootstrapping Model**: Explains that ALL users have admin privileges until someone invokes `become_an_administrator`
- Security implications of the bootstrapping model
- Common error messages and their solutions

### 2. Action-Specific Documentation Updates

#### `become_an_administrator` Action
- Added critical bootstrapping information with emoji alerts (🚨)
- Explained that this is a ONE-TIME action that permanently sets the admin
- Clarified that ALL users have admin rights before this action is invoked
- Added prerequisites, side effects (system restart), and security implications
- Updated authentication requirement to clarify it needs a bearer token but NO admin must exist yet

#### `signup` Action
- Added prominent note about admin privileges for new users when no admin exists
- Listed specific validation requirements (8+ character password)
- Explained common error messages with solutions
- Warned about mobile/OTP configuration issues
- Added clear next steps after signup

#### `signin` Action
- Added prerequisites and clear request format
- Explained JWT token usage with examples
- Detailed the response actions (store, cookie, notify, redirect)
- Added security features information

### 3. Field-Specific Documentation

#### Mobile Field (signup)
- Added prominent warning about OTP configuration requirement
- Explained the "Decoding of secret as base32 failed" error
- Recommended leaving it empty for initial setup

### 4. Authentication Requirements
- Updated `getAuthRequirement` function to properly explain `become_an_administrator` requirements
- Clarified that it requires a bearer token but can only be invoked when NO admin exists

## Key Insights Documented

1. **Admin Bootstrapping Model**: Daptin uses a unique approach where all users start with admin privileges until someone claims the sole admin role. This is clever for initial setup but was previously undocumented.

2. **Password Requirements**: The 8-character minimum for passwords was enforced but not documented in the OpenAPI spec.

3. **OTP Pitfall**: The mobile field triggers OTP generation which fails if not configured, causing cryptic errors.

4. **One-Time Admin Setting**: Once an admin is set via `become_an_administrator`, this cannot be undone or transferred.

## Impact

These documentation improvements transform Daptin from having hidden setup requirements to being truly self-discoverable. New users can now:
- Understand the unique admin bootstrapping model
- Avoid common validation errors
- Successfully complete initial setup without external documentation
- Understand security implications of the setup process

The OpenAPI spec now serves as complete documentation for self-onboarding and self-service.