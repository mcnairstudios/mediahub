# pkg/auth

Authentication and authorization interfaces for mediahub.

## Responsibilities

- Define the `Role` type and role constants (admin, standard, jellyfin)
- Define the `User` struct representing an authenticated user (with optional email for OAuth)
- Define the `Service` interface for authentication operations (login, token validation, token refresh, user management)
- Define the `UserStore` interface for user persistence (CRUD + password updates + email lookup)
- Provide `JWTService` implementation with `GetUserByEmail` for Google OAuth email matching

## Design

This package contains types, interfaces, and the JWT-based implementation. Google OAuth is handled by `pkg/api/google_auth.go` which calls `JWTService.GetUserByEmail` to match Google accounts to mediahub users by email. No auto-registration -- admin must add an email to a user account before OAuth login works.
