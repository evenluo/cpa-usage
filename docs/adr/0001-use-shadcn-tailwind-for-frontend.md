# Use shadcn/ui and Tailwind for the frontend

CPA Usage v1 will use React, TypeScript, Vite, Tailwind CSS, and shadcn/ui for the redesigned frontend. This replaces the old keeper-style SCSS-module component system as the primary UI path because the product needs a polished analytics workspace with consistent controls, source-owned components, and a visual system that can be customized toward the Magic-like dashboard direction without adopting a heavier component framework.

## Considered Options

- Continue the old keeper SCSS-module component system.
- Use shadcn/ui with Tailwind CSS.
- Use a heavier component framework such as Ant Design.

## Consequences

- Frontend UI structure does not need to preserve old component compatibility.
- Backend collection, SQLite storage, pricing, and CPA usage semantics remain compatible.
- shadcn/ui provides base controls, while charts use Recharts with custom or Tremor-style primitives for a more polished analytics visual language.
- ECharts is not the v1 primary chart system.
