# Task: Implement Frontend Operator App

## Objective
Analyze the existing MES backend repository and create a Vite + React + TypeScript frontend to interact with it via the Backend-For-Frontend (BFF). The UI will utilize Tailwind CSS v4 and `shadcn/ui`. All work will be done on the current branch without a separate feature branch.

## Steps
- [ ] Create `frontend/operator` Vite React TS app.
- [ ] Install and configure Tailwind CSS v4.
- [ ] Initialize `shadcn/ui` with Vite/React 19 defaults.
- [ ] Configure `vite.config.ts` proxy for BFF (`/api` and `/ws`).
- [ ] Implement a basic UI dashboard showing the Dispatch List.
- [ ] Implement start operation action.

## Constraints
- Follow all `@CONTRIBUTING.md` and `@AGENT.md` guidelines.
- Use Tailwind CSS v4 (no `tailwind.config.js`, use `@import "tailwindcss"`).
- Work directly on the current branch as requested by the user.
- Commit atomically using Conventional Commits.

## Definition of Done
- Basic frontend is fully initialized and can proxy to the BFF.
- `TASK.md` is removed in the final commit.