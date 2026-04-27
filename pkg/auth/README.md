# pkg/auth

Authentication and authorization interfaces for mediahub.

## Responsibilities

- Define the `Role` type and role constants (admin, standard, jellyfin)
- Define the `User` struct representing an authenticated user
- Define the `Service` interface for authentication operations (login, token validation, token refresh, user management)
- Define the `UserStore` interface for user persistence (CRUD + password updates)

## Design

This package contains only types and interfaces with zero external dependencies. Implementations live in separate packages that import these interfaces.
