# ADR-010: Frontend Operator Stack

## Status
Accepted

## Context
KORS requires a web-based operator interface to interact with the Backend-For-Frontend (BFF). The project already uses a Go-based backend and NATS for inter-service communication. We need a modern, performant, and maintainable frontend stack that aligns with the rest of the project's developer experience.

## Decision
We decided to use the following technologies for all KORS frontend applications (starting with `frontend/operator`):

1.  **Vite**: Next-generation frontend tooling for fast development and bundling.
2.  **React 19**: Standard UI library with stable concurrent features and optimized rendering.
3.  **TypeScript**: For type-safe development and better integration with backend Protobuf-generated types.
4.  **Tailwind CSS v4**: For high-performance utility-first styling. We specifically chose v4 for its "CSS-first" approach, removing the need for `tailwind.config.js` and leveraging the `@tailwindcss/vite` plugin.
5.  **shadcn/ui**: For a highly accessible and fully customizable component library. Components are copied directly into the project (`src/components/ui`), allowing full control over their implementation without external package rigidness.

## Consequences

### Positive
*   **Developer Experience**: Extremely fast HMR (Hot Module Replacement) via Vite.
*   **Consistency**: Utility-first CSS ensures consistent spacing and typography.
*   **Performance**: Tailwind v4 generates minimal CSS and uses modern browser features.
*   **Accessibility**: shadcn/ui components follow WAI-ARIA standards out of the box.
*   **Maintainability**: Direct ownership of component code (shadcn) prevents "dependency hell" for UI elements.

### Negative
*   **Build Complexity**: Adds a Node.js-based build step to the primarily Go-based repository.
*   **Initial Setup**: Requires configuring proxies and Path Aliases (`@/*`) to integrate with the existing infrastructure.

## Implementation Details

### Path Aliases
The `@/*` alias is used to refer to the `src/` directory. Configuration is located in `vite.config.ts` and `tsconfig.json`.

### BFF Integration
Frontend applications do not communicate with internal services directly. All calls go to the BFF (`/api/v1/...`). Local development uses the Vite proxy to route `/api` and `/ws` to Traefik (port 80) with the `Host: kors.local` header.

### Styling
All styling is done within the `.tsx` files using Tailwind classes or the main `index.css` using the new `@import "tailwindcss";` directive. No legacy CSS modules are permitted.
