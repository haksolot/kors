# KORS — Operator Frontend

This is the main web interface for KORS operators. It provides real-time access to the dispatch list, manufacturing order execution, and non-conformity declarations.

## Tech Stack

- **Framework**: [React 19](https://react.dev/)
- **Build Tool**: [Vite](https://vitejs.dev/)
- **Language**: [TypeScript](https://www.typescriptlang.org/)
- **Styling**: [Tailwind CSS v4](https://tailwindcss.com/)
- **UI Components**: [shadcn/ui](https://ui.shadcn.com/)

## Getting Started

### Prerequisites

- [Node.js](https://nodejs.org/) (v20 or later)
- The KORS backend infrastructure must be running (see `infra/local/README.md`)

### Installation

```bash
cd frontend/operator
npm install
```

### Development

Start the development server:

```bash
npm run dev
```

The application will be available at `http://localhost:5173/`. 
API requests to `/api` and WebSocket connections to `/ws` are automatically proxied to the local Traefik gateway (`http://localhost:80`) with the required `Host: kors.local` header.

## Project Structure

- `src/components/ui/`: Base shadcn components. Do not modify unless strictly necessary.
- `src/lib/utils.ts`: Utility functions (e.g., class merger for Tailwind).
- `src/App.tsx`: Main application entry point and dashboard logic.
- `vite.config.ts`: Vite configuration, including the BFF proxy and Tailwind plugin.

## Authentication

The application requires a valid JWT for access. During local development, you must paste a token from Keycloak into the "JWT Token" field to authorize requests. The token is persisted in `localStorage`.

## Conventions

Refer to `docs/adr/ADR-010-frontend-tech-stack.md` for architectural decisions. Follow the project's global conventions in `AGENT.md`.
